package main

import (
	"bufio"
	"fmt"
	"io"
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

	crawl(seeds, logname)
}

/*
Essentially do a BFS on url tree
*/
func crawl(seeds []string, logname string) {
	visited := make(map[string]struct{})                 // visited urls
	var frontier []string = seeds                        // to visit, the crawl frontier
	var iidx map[string]string = make(map[string]string) // inverted index
	log, err := os.Create(logname)
	if err != nil {
		fmt.Printf("Could not create log file: %s\n", logname)
		return
	}
	defer log.Close()

	// make a file named logname
	// time in increments

	exec_start := time.Now()
	for len(frontier) > 0 && len(visited) < 100 {
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
		frontier = frontier[1:]
	}
	logiidx(log, iidx, time.Since(exec_start))
}

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
	plain := get_readable(seed)
	punct := "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	urlRegex := `https?://[^\s/$.?#].[^\s]*|www\.[^\s/$.?#].[^\s]*`
	re := regexp.MustCompile(urlRegex)
	var discovered []string

	scanner := bufio.NewScanner(strings.NewReader(plain))
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		t := scanner.Text()
		matches := re.FindAllString(t, -1)
		for _, url := range matches {
			//fmt.Printf("%s\n", url)
			discovered = append(discovered, url)
			continue
		}

		word := strings.Trim(t, punct)
		iidx[word] = seed
	}

	return discovered
}

func get_readable(seed string) string {
	resp, err := http.Get(string(seed)) // annoying the typedef doesn't work
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
