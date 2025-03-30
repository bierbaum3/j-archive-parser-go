package download

import (
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL           = "http://j-archive.com"
	seasonURLTemplate = "http://j-archive.com/showseason.php?season=%d"
	gameURLTemplate   = "http://j-archive.com/showgame.php?game_id=%s"
	siteFolder        = "season-archive"
	latestSeason      = 41
)

var (
	episodeRe = regexp.MustCompile(`^(https?://(www\.)?j-archive\.com/)?showgame\.php\?game_id=\d+$`)
	epIdRe    = regexp.MustCompile(`game_id=(\d+)`)
	epNumRe   = regexp.MustCompile(`#(\d{1,4})`)
)

func Run(seasons []int) {
	// Default to downloading season 41 if none provided
	if len(seasons) == 0 {
		seasons = []int{latestSeason}
	}

	err := os.MkdirAll(siteFolder, os.ModePerm)
	if err != nil {
		log.Fatalf("Error creating directory %s: %v", siteFolder, err)
	}

	numThreads := runtime.NumCPU() * 2
	fmt.Printf("Using %d threads\n", numThreads)

	var wg sync.WaitGroup
	seasonChan := make(chan int, numThreads)

	for _, season := range seasons {
		wg.Add(1)
		seasonChan <- season
		go func(season int) {
			defer wg.Done()
			downloadSeason(season)
			<-seasonChan
		}(season)
	}

	wg.Wait()
}

// downloads a season page, parses it for episode links, and downloads each episode's HTML
func downloadSeason(season int) {
	fmt.Printf("Downloading Season %d\n", season)
	seasonFolder := filepath.Join(siteFolder, fmt.Sprintf("season %d", season))
	// Create season folder if needed
	if err := os.MkdirAll(seasonFolder, os.ModePerm); err != nil {
		log.Printf("Error creating season folder %s: %v", seasonFolder, err)
		return
	}

	// Download the season page
	seasonURL := fmt.Sprintf(seasonURLTemplate, season)
	resp, err := http.Get(seasonURL)
	if err != nil {
		log.Printf("Error downloading season page %s: %v", seasonURL, err)
		return
	}
	defer resp.Body.Close()

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("Error parsing season page %s: %v", seasonURL, err)
		return
	}

	// Collect episode links and their text
	var episodeLinks []string
	var linkTexts []string
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && episodeRe.MatchString(href) {
			episodeLinks = append(episodeLinks, href)
			linkTexts = append(linkTexts, s.Text())
		}
	})

	fmt.Printf("Found %d episode links in Season %d\n", len(episodeLinks), season)

	// Reverse slices to process links in correct order
	reverseStrings(episodeLinks)
	reverseStrings(linkTexts)

	// Loop through each episode link and extract episode numbers and IDs
	for i, link := range episodeLinks {
		match := epNumRe.FindStringSubmatch(linkTexts[i])
		if len(match) < 2 {
			log.Printf("Episode number not found in text: %s", linkTexts[i])
			continue
		}
		episodeNumber := match[1]
		gameFile := filepath.Join(seasonFolder, fmt.Sprintf("%s.html", episodeNumber))

		if _, err := os.Stat(gameFile); err == nil {
			continue
		}

		matchID := epIdRe.FindStringSubmatch(link)
		if len(matchID) < 2 {
			log.Printf("Game id not found in link: %s", link)
			continue
		}
		episodeID := matchID[1]
		gameURL := fmt.Sprintf(gameURLTemplate, episodeID)
		fmt.Printf("Downloading Episode %s from Season %d\n", episodeNumber, season)

		err = downloadFile(gameURL, gameFile)
		if err != nil {
			log.Printf("Error downloading episode %s: %v", episodeNumber, err)
		}
		// Wait 2-6 seconds between downloads to not overload the server
		sleepTime := rand.IntN(6) + 2
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}

	fmt.Printf("Season %d finished\n", season)
}

// downloads HTML content from each URL and saves it to a file
func downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET error: %v", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("file creation error: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}
	return nil
}

// helper to reverse a slice of strings in place
func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
