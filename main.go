// Distributed Systems Asgn1
// used global variables for data structures instead of passing
package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
)

const MAX_URLS = 500

// map of stop words (from https://gist.github.com/sebleier/554280)
var stopwords = map[string]struct{}{
	"i": {}, "me": {}, "my": {}, "myself": {},
	"we": {}, "our": {}, "ours": {}, "ourselves": {},
	"you": {}, "your": {}, "yours": {}, "yourself": {}, "yourselves": {},
	"he": {}, "him": {}, "his": {}, "himself": {},
	"she": {}, "her": {}, "hers": {}, "herself": {},
	"it": {}, "its": {}, "itself": {},
	"they": {}, "them": {}, "their": {}, "theirs": {}, "themselves": {},
	"what": {}, "which": {}, "who": {}, "whom": {},
	"this": {}, "that": {}, "these": {}, "those": {},
	"am": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {},
	"have": {}, "has": {}, "had": {}, "having": {},
	"do": {}, "does": {}, "did": {}, "doing": {},
	"a": {}, "an": {}, "the": {},
	"and": {}, "but": {}, "if": {}, "or": {}, "because": {},
	"as": {}, "until": {}, "while": {},
	"of": {}, "at": {}, "by": {}, "for": {}, "with": {},
	"about": {}, "against": {}, "between": {}, "into": {},
	"through": {}, "during": {}, "before": {}, "after": {},
	"above": {}, "below": {},
	"to": {}, "from": {}, "up": {}, "down": {},
	"in": {}, "out": {}, "on": {}, "off": {},
	"over": {}, "under": {},
	"again": {}, "further": {}, "then": {}, "once": {},
	"here": {}, "there": {}, "when": {}, "where": {},
	"why": {}, "how": {},
	"all": {}, "any": {}, "both": {}, "each": {},
	"few": {}, "more": {}, "most": {}, "other": {},
	"some": {}, "such": {},
	"no": {}, "nor": {}, "not": {}, "only": {}, "own": {},
	"same": {}, "so": {}, "than": {}, "too": {}, "very": {},
	"s": {}, "t": {}, "can": {}, "will": {}, "just": {},
	"don": {}, "should": {}, "now": {},
}

// global variables
var inverted_index map[string][]string // inverted index of key words to urls
var crawl_frontier []string            // dynamic list of urls to crawl
var total_urls_crawled int
var seen_urls map[string]bool // map for keeping track of seen urls
var client = &http.Client{}   // used for creating custom http request

// helper for checking error status
func check(e error) {
	if e != nil {
		panic(e)
	}
}

// this function is used to log the word url pairs to a json file
func log_inverted_index(file *os.File) {
	jsonString, err := json.MarshalIndent(inverted_index, "", "  ")
	check(err)
	file.Write(jsonString)
}

// used for stripping punctuation from word for inverted index
func strip_punctuation(w string) string {
	rs := []rune(w)
	var result strings.Builder

	//loop through each char in the string to check if
	//its punctuation
	for i, r := range rs {
		if unicode.IsPunct(r) {
			//check if the chunk is a float
			if r == '.' && i > 0 && i < len(rs)-1 &&
				unicode.IsDigit(rune(rs[i-1])) &&
				unicode.IsDigit(rune(rs[i+1])) {
				result.WriteRune(r)
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// this function handles lowercasing all characters, removing punctuation, and stop words
// returns empty string if resulting word is nil
func filter_words(words string) []string {
	filtered := make([]string, 0, 8)
	for w := range strings.FieldsSeq(words) {
		w = strings.ToLower(w)
		w = strip_punctuation(w)
		if _, found := stopwords[w]; !found && len(w) > 0 {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

// this function will be used to send get requests on url passed
// and extract the urls of the page updating the global crawl_frontier as well
// as the inverted index
func process(url_to_process string) error {
	//we need to convert url to its correct URL structure for resolving relative paths
	base, err := url.Parse(url_to_process)
	if err != nil {
		return err
	}
	//need to set up get request with user agent header to prevent being blocked
	req, err := http.NewRequest("GET", url_to_process, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Golang_Crawler/1.0")
	//send the get request and save response
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d for %s", resp.StatusCode, url_to_process)
	}

	defer resp.Body.Close()

	//filter only html text
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return nil
	}

	//parse the response body with html parsing module
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}
	//to ensure that urls arent added multiple times when multiple occurences of same word
	seenWords := make(map[string]bool)
	//descend html tree and filter href attributes (from: https://pkg.go.dev/golang.org/x/net/html#Parse)
	for n := range doc.Descendants() {
		// scrape urls
		if n.Type == html.ElementNode && n.DataAtom == atom.A {
			for _, a := range n.Attr {
				if a.Key == "href" {
					//we need to resolve href into absolute url structure
					u, err := url.Parse(a.Val)
					if err != nil {
						continue
					}
					//skip non http references
					if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != ""{
						continue
					}
					u = base.ResolveReference(u)
					//avoid adding already seen urls
					if _, seen := seen_urls[u.String()]; seen {
						continue
					}
					seen_urls[u.String()] = true
					crawl_frontier = append(crawl_frontier, u.String())
					break
				}
			}
		}
		// populate the inverted index
		if n.Type == html.TextNode {
			//ignore scripts or style sheets
			if n.Parent != nil && n.Parent.Type == html.ElementNode {
				if n.Parent.Data == "script" || n.Parent.Data == "style" {
					continue
				}
			}
			valid_words := filter_words(n.Data)
			for _, word := range valid_words {
				if seenWords[word] {
					continue
				}
				seenWords[word] = true
				inverted_index[word] = append(inverted_index[word], url_to_process)
			}
		}
	}
	return nil
}

// given a seed url this function descends into links,
// processing each page
func crawl(url string, url_log_file *os.File) {
	crawl_frontier = append(crawl_frontier, url)
	//crawl urls while descent is possible
	for len(crawl_frontier) > 0 {
		if total_urls_crawled >= MAX_URLS {
			break
		}
		fmt.Println(total_urls_crawled, url)
		total_urls_crawled += 1
		//pop url from list and process it
		url = crawl_frontier[0]
		crawl_frontier = crawl_frontier[1:]
		fmt.Fprint(url_log_file, url, "\n") //writing url each time could slow down maybe buffer?
		err := process(url)
		if err != nil{
			fmt.Println(err)
		}
	}
}

func main() {
	crawl_frontier = make([]string, 0, 8)
	inverted_index = make(map[string][]string)
	seen_urls = make(map[string]bool)
	seeded_urls := []string{"https://en.wikipedia.org/wiki/Go_(programming_language)"}

	//log files
	url_log_file, err := os.Create("/tmp/crawl_log.txt")
	check(err)
	defer url_log_file.Close()
	inverted_index_json_file, err := os.Create("/tmp/inverted_index_log.json")
	check(err)
	defer inverted_index_json_file.Close()

	t1 := time.Now()
	//crawl the urls
	for _, url := range seeded_urls {
		crawl(url, url_log_file)
	}
	t2 := time.Now()
	log_inverted_index(inverted_index_json_file)
	fmt.Printf("Total Time Crawling: %v\n", t2.Sub(t1))
}
