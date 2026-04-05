package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/k3a/html2text"
)

const MAX_FRONTIER = 100

func main() {
	seeds := []string{"https://doc.rust-lang.org/book/",
		"urlhttps://tokio.rs/#tk-lib-tonic2",
		"urhttps://embassy.dev/l3",
		"https://en.wikipedia.org/wiki/Functor",
		"https://en.wikipedia.org/wiki/Map",
		"https://go.dev/tour/welcome/1",
	}

	logname := "visited.txt"
	index_logname := "iidx_log.jsonl"

	crawl(seeds, logname, index_logname)
}

/*
Essentially do a BFS on url tree
*/
func crawl(seeds []string, logname string, iidx_logname string) {
	visited := make(map[string]struct{})                 // visited urls
	var frontier []string = seeds                        // to visit, the crawl frontier
	var iidx map[string]string = make(map[string]string) // inverted index
	power := 1

	log, err := os.Create(logname)
	if err != nil {
		fmt.Printf("Could not create log file: %s\n", logname)
		return
	}
	defer log.Close()

	iidx_log, err := os.Create(iidx_logname)
	if err != nil {
		fmt.Printf("Could not create log file for inverted index %s\n", iidx_logname)
		return
	}
	defer iidx_log.Close()

	exec_start := time.Now()
	for len(frontier) > 0 && len(visited) < 110 {
		time_taken := time.Since(exec_start)
		logprog(log, frontier[0], len(frontier), len(visited), len(iidx), time_taken)

		visited[frontier[0]] = struct{}{}        // mark the next url as visited
		discovered := process(frontier[0], iidx) // process the text at the current url

		for _, d := range discovered {
			if _, exists := visited[d]; !exists {
				frontier = append(frontier, d) // a discovered url has not been seen before, so add it to the frontier
				if len(frontier) > MAX_FRONTIER {
					break
				}
			}
		}
		frontier = frontier[1:] // actually pop the first item

		if len(visited) > int(math.Pow10(power)) {
			// this will checkpoint the idx when visited passes:
			// 		10, 100, 1000, ... 10^power
			checkpoint_iidx(iidx_log, iidx)
			power++
		}

	}
	logiidx(log, iidx, time.Since(exec_start)) // now that everything is done log the inverted index
}

func checkpoint_iidx(log *os.File, iidx map[string]string) {
	b, err := json.MarshalIndent(iidx, "", "  ")
	if err != nil {
		fmt.Printf("Couldn't log inverted index: %s\n", err)
	}

	log.WriteString(string(b))
}

/*
Function that logs the first 10 entries of a map to the file pointed to by log.
*/
func logiidx(log *os.File, iidx map[string]string, time_taken time.Duration) {
	log.WriteString("\n\n--- The top 10 entries in the inverted index ---\n\n")
	entries_written := 0
	for k, v := range iidx {
		if entries_written > 10 {
			break
		}

		log.WriteString(fmt.Sprintf("%s ↦ %s\n\n", k, v))
		entries_written++
	}

	log.WriteString(fmt.Sprintf("took %s\n", time_taken))
}

/*
Function that logs the current progress to stdout + log file. The weird control characters in the printf
statements make it appear like the time + lengths are updating in place.
*/
func logprog(log *os.File, name string, flen int, vlen int, ilen int, time_taken time.Duration) {
	// file loggin
	log.WriteString(fmt.Sprintf("%s %d %s\n", name, vlen, time_taken))

	// stdout printing
	fmt.Printf("\033[5A")
	fmt.Printf("\r\033[Kcurrent:       %s\n", name)
	fmt.Printf("\r\033[Klen(frontier): %d\n", flen)
	fmt.Printf("\r\033[Klen(visited) : %d\n", vlen)
	fmt.Printf("\r\033[Klen(iidx)    : %d\n", ilen)
	fmt.Printf("\r\033[Ktime taken   : %s\n", time_taken)
}

/*
process a seed url, being sure to update values in the inverted index (iidx)

seed : url to process
iidx : the inverted index mapping words to urls

returns: the list of urls found when processing seed
*/
func process(seed string, iidx map[string]string) []string {
	punct := "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	url_regex := `https?://[^\s/$.?#].[^\s]*|www\.[^\s/$.?#].[^\s]*`
	re := regexp.MustCompile(url_regex)
	plain := get_readable(seed)
	var discovered []string

	scanner := bufio.NewScanner(strings.NewReader(plain))
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		t := scanner.Text()
		matches := re.FindAllString(t, -1)
		for _, url := range matches {
			discovered = append(discovered, url) // found a url
			continue
		}

		word := strings.Trim(t, punct) // get rid of end punctuation
		iidx[word] = seed              // update inverted index
	}

	return discovered
}

/*
Gets the human readable text from the url given by seed.
*/
func get_readable(seed string) string {
	var client = &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Get(string(seed)) // annoying the typedef doesn't work
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	plain := html2text.HTML2Text(string(body))
	return plain
}
