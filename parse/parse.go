package parse

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

var (
	siteFolder = "season-archive"
	csvFolder  = "parsed-csv"
)

func Run() {
	// Create CSV folder if it doesn't exist
	if err := os.MkdirAll(csvFolder, os.ModePerm); err != nil {
		log.Fatalf("Error creating CSV folder: %v", err)
	}

	// Get list of season numbers
	seasons, err := getAllSeasons()
	if err != nil {
		log.Fatalf("Error getting seasons: %v", err)
	}

	// Use goroutines to parse seasons concurrently
	numThreads := runtime.NumCPU() * 2
	fmt.Printf("Using %d threads for parsing seasons\n", numThreads)
	var wg sync.WaitGroup
	sem := make(chan struct{}, numThreads)
	for _, season := range seasons {
		wg.Add(1)
		sem <- struct{}{}
		go func(season int) {
			defer wg.Done()
			parseSeason(season)
			<-sem
		}(season)
	}
	wg.Wait()
	fmt.Println("Parsing complete.")
}

// returns slice of season numbers found in the siteFolder
func getAllSeasons() ([]int, error) {
	var seasons []int
	entries, err := os.ReadDir(siteFolder)
	if err != nil {
		return nil, err
	}
	// extract season number from directory names
	re := regexp.MustCompile(`\d+`)
	for _, entry := range entries {
		if entry.IsDir() {
			match := re.FindString(entry.Name())
			if match != "" {
				if num, err := strconv.Atoi(match); err == nil {
					seasons = append(seasons, num)
				}
			}
		}
	}
	return seasons, nil
}

// processes all HTML files and writes to a CSV
func parseSeason(season int) {
	fmt.Printf("Starting season %d\n", season)
	seasonDir := filepath.Join(siteFolder, fmt.Sprintf("season %d", season))
	entries, err := os.ReadDir(seasonDir)
	if err != nil {
		log.Printf("Error reading season directory %s: %v", seasonDir, err)
		return
	}

	// Create CSV file for this season
	csvPath := filepath.Join(csvFolder, fmt.Sprintf("j-archive-season-%d.csv", season))
	csvFile, err := os.Create(csvPath)
	if err != nil {
		log.Printf("Error creating CSV file %s: %v", csvPath, err)
		return
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Write CSV header
	header := []string{"epNum", "airDate", "round_name", "category", "value", "daily_double", "question", "answer"}
	writer.Write(header)

	for i, entry := range entries {
		if entry.IsDir() {
			continue
		}
		episodePath := filepath.Join(seasonDir, entry.Name())
		fmt.Printf("Season %d: Parsing episode %d/%d\n", season, i+1, len(entries))
		rounds, err := parseEpisode(episodePath)
		if err != nil {
			log.Printf("Error parsing episode %s: %v", episodePath, err)
			continue
		}
		// Write the row to the CSV
		for _, round := range rounds {
			for _, row := range round {
				writer.Write(row)
			}
		}
	}
	fmt.Printf("Season %d complete\n", season)
}

// parses an episode HTML file and returns data organized by Jeopardy round (Jeopardy, Double Jeopardy, Final Jeopardy)
// returns slice where each element is a round (a slice of rows, and each row is a []string)
func parseEpisode(filePath string) ([][][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return nil, err
	}

	// Extract episode number from the <title>.
	titleText := doc.Find("title").Text()
	reEpNum := regexp.MustCompile(`#(\d+)`)
	epNum := ""
	if m := reEpNum.FindStringSubmatch(titleText); len(m) >= 2 {
		epNum = m[1]
	}

	// Extract air date (YYYY-MM-DD) from the title
	reAirDate := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	airDate := reAirDate.FindString(titleText)

	hasRoundJ := doc.Find("#jeopardy_round").Length() > 0
	hasRoundDJ := doc.Find("#double_jeopardy_round").Length() > 0
	hasRoundFJ := doc.Find("#final_jeopardy_round").Length() > 0
	hasRoundTB := doc.Find("#final_jeopardy_round .final_round").Length() > 1

	var rounds [][][]string

	if hasRoundJ {
		jTable := doc.Find("#jeopardy_round")
		rows := parseRound(0, jTable, epNum, airDate)
		rounds = append(rounds, rows)
	}
	if hasRoundDJ {
		djTable := doc.Find("#double_jeopardy_round")
		rows := parseRound(1, djTable, epNum, airDate)
		rounds = append(rounds, rows)
	}
	if hasRoundFJ {
		// For Final Jeopardy, use the first .final_round element.
		fjTable := doc.Find("#final_jeopardy_round .final_round").First()
		rows := parseRound(2, fjTable, epNum, airDate)
		rounds = append(rounds, rows)
	}
	if hasRoundTB {
		// For Tiebreaker, use the second .final_round element.
		tbTable := doc.Find("#final_jeopardy_round .final_round").Eq(1)
		rows := parseRound(3, tbTable, epNum, airDate)
		rounds = append(rounds, rows)
	}

	if len(rounds) == 0 {
		return nil, fmt.Errorf("no rounds found in episode %s", filePath)
	}
	return rounds, nil
}

// parses a game round from the provided table selection and returns rows of the CSV
func parseRound(round int, table *goquery.Selection, epNum, airDate string) [][]string {
	var rows [][]string

	if round < 2 {
		// Get category names for Jeopardy (round==0) or Double Jeopardy (round==1).
		var categories []string
		table.Find("td.category_name").Each(func(i int, s *goquery.Selection) {
			categories = append(categories, strings.TrimSpace(s.Text()))
		})
		x := 0
		// Iterate over each clue
		table.Find("td.clue").Each(func(i int, s *goquery.Selection) {
			clueText := strings.TrimSpace(s.Text())
			if clueText == "" {
				// Skip empty clues, assuming 6 categories per round
				x = (x + 1) % 6
				return
			}

			// Get the raw value (monetary value) from a td whose class contains "clue_value".
			valueRaw := strings.TrimSpace(s.Find("td[class*='clue_value']").Text())
			value := ""
			if valueRaw != "" {
				v := strings.ReplaceAll(strings.TrimPrefix(valueRaw, "D: $"), ",", "")
				v = strings.TrimPrefix(v, "$")
				value = v
			} else {
				value = "-100"
			}
			// Determine if clue is a Daily Double
			dailyDouble := "false"
			if strings.HasPrefix(valueRaw, "DD:") {
				dailyDouble = "true"
			}
			// Get the question text
			question := ""
			s.Find("td.clue_text").EachWithBreak(func(i int, sel *goquery.Selection) bool {
				if style, exists := sel.Attr("style"); !exists || !strings.Contains(style, "display:none") {
					question = strings.TrimSpace(sel.Text())
					return false
				}
				return true
			})

			// Extract answer from onmouseover attribute
			answer := ""
			// Find the visible clue text from the container <td class="clue">
			visibleClueTd := s.Find("td.clue_text").First()

			if visibleClueTd.Length() > 0 {
				// Get clue ID
				clueID, exists := visibleClueTd.Attr("id")
				if exists {
					// Move up to the parent <tr> of the clue
					tr := visibleClueTd.ParentsFiltered("tr")
					if tr.Length() > 0 {
						// Find the sibling hidden <td>
						responseSel := tr.Find("td#" + clueID + "_r")
						if responseSel.Length() > 0 {
							answer = strings.TrimSpace(responseSel.Find("em.correct_response").Text())
						}
					}
				}
			}

			category := ""
			if x < len(categories) {
				category = categories[x]
			}
			roundName := "Jeopardy"
			if round == 1 {
				roundName = "Double Jeopardy"
			}

			// Append row to CSV
			row := []string{epNum, airDate, roundName, category, value, dailyDouble, question, answer}
			rows = append(rows, row)

			// Update column tracker (assuming 6 columns per round)
			if x == 5 {
				x = 0
			} else {
				x++
			}
		})
	} else if round == 2 {
		// Final Jeopardy
		onmouseover, exists := table.Find("div[onmouseover]").Attr("onmouseover")
		value := ""
		if exists {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(onmouseover))
			if err == nil {
				// Collect text from <td> elements that have no attributes
				var vals []string
				doc.Find("td").Each(func(i int, s *goquery.Selection) {
					vals = append(vals, strings.TrimSpace(s.Text()))
				})
				value = strings.Join(vals, ",")
			}
		}
		question := strings.TrimSpace(table.Find("td#clue_FJ").Text())
		answer := ""
		responseSel := table.Find("td#clue_FJ_r")
		if responseSel.Length() > 0 {
			answer = strings.TrimSpace(responseSel.Find("em.correct_response").Text())
		}

		dailyDouble := "false"
		category := strings.TrimSpace(table.Find("td.category_name").Text())
		roundName := "Final Jeopardy"
		row := []string{epNum, airDate, roundName, category, value, dailyDouble, question, answer}
		rows = append(rows, row)
	} else if round == 3 {
		// Tiebreaker round
		value := ""
		question := strings.TrimSpace(table.Find("td#clue_TB").Text())
		answer := ""
		onmouseover, exists := table.Find("div[onmouseover]").Attr("onmouseover")
		if exists {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(onmouseover))
			if err == nil {
				answer = strings.TrimSpace(doc.Find("em").Text())
			}
		}
		dailyDouble := "false"
		category := strings.TrimSpace(table.Find("td.category_name").Text())
		roundName := "Tiebreaker"
		row := []string{epNum, airDate, roundName, category, value, dailyDouble, question, answer}
		rows = append(rows, row)
	}

	return rows
}
