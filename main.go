package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"j-parser-go/download"
	"j-parser-go/parse"
	"strconv"
)

func main() {
	mode := flag.String("mode", "", "Mode: download or parse")
	seasonsFlag := flag.String("seasons", "", "Comma-separated list of seasons to download (e.g., 1,2,3)")
	flag.Parse()

	switch *mode {
	case "download":
		seasons := []int{}
		if *seasonsFlag != "" {
			seasonStrings := strings.Split(*seasonsFlag, ",")
			for _, s := range seasonStrings {
				num, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil {
					fmt.Printf("Invalid season number: %s\n", s)
					os.Exit(1)
				}
				seasons = append(seasons, num)
			}
		}
		download.Run(seasons)
	case "parse":
		parse.Run()
	default:
		fmt.Println("Please specify a valid mode: -mode=download or -mode=parse")
		os.Exit(1)
	}
}
