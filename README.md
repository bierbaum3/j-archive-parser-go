# Jeopardy Archive Parser

Heavily inspired by jbovee's [j-archive-parser](https://github.com/jbovee/j-archive-parser)

Jeopardy Archive Parser is a tool designed to download and parse episodes from [J! Archive](http://j-archive.com). My primary motivation in making this is to be able to more easily build Anki decks from clues that have appeared on the show, as well as getting more experience with Go.

It has two modes:

- **Download Mode:** Downloads HTML pages for specific seasons and saves each episode's page locally.
- **Parse Mode:** Processes the downloaded HTML files to extract relevant game details (see the [parse](parse) package for more details).

## Requirements

- [Go](https://golang.org/) 1.24 or later
- An active Internet connection to access [Jeopardy! Archive](http://j-archive.com)

## Usage

There are two modes: download and parse. Specify the mode with the `-mode` flag and provide additional options as needed.

### Download Mode

Downloads HTML files for the specified seasons to the **season-archive** directory.

`-mode=download`: Runs the program in download mode.

`-seasons`: A comma-separated list of season numbers to download. If omitted, the program defaults to downloading season 41 (the most recent season as of this writing).

```bash
go run main.go -mode=download -seasons=1,2,3
```

### Parse Mode

Processes the previously downloaded HTML files and writes the results to CSVs in the **parsed-csv** directory.

`-mode=parse`: Runs the program in parse mode.

```bash
go run main.go -mode=parse
```
