// TODO: frontier queue, seed urls, actual scraping, telling program to look for links, prioritisation of links?, go to next links, do same
// TODO: politness
// TODO: later, maybe try using goquery instead and check how that effects performance, just using html standard for now
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/net/html"
)

type WebpageForMongo struct {
	ID      bson.ObjectID `bson:"_id,omitempty"`
	URL     string        `bson:"url"`
	Title   string        `bson:"title"`
	Content string        `bson:"content"`
}

type Webpage struct {
	title     string
	words     []string
	links     []string
	linkCount int
}

// using regex and a different function to get title
var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func extractTitle(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	matches := titleRe.FindSubmatch(data)
	if matches == nil {
		return "", nil
	}

	return strings.TrimSpace(string(matches[1])), nil
}

// for extractor
func hiddenByattributes(tok html.Token) bool {
	for _, attr := range tok.Attr {
		switch attr.Key {
		case "hidden":
			// do this
			return true

		case "aria-hidden":
			// do this
			if attr.Val == "true" {
				return true
			}

		case "style":
			// do this
			s := strings.ReplaceAll(attr.Val, " ", "")
			if strings.Contains(s, "display:none") || strings.Contains(s, "visibility:hidden") {
				return true
			}
		}
	}
	return false
}

var neverVisible = map[string]bool{
	"script": true, "style": true, "noscript": true,
	"head": true, "title": true,
	"meta": true, "link": true,
	"template": true,
}

// returns words, links, linkCount
func extractor(url string) (webpage Webpage) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("failed to fetch: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("Status code error: %d", res.StatusCode)
	}

	// extraction
	webpage.title, err = extractTitle(res.Body)

	tokenizer := html.NewTokenizer(res.Body)

	fmt.Println("Extracting links...")
	webpage.linkCount = 0

	var skipStack []string

	skipping := func() bool { return len(skipStack) > 0 }

	// 3. LOOP: Stream through the HTML tokens
	for {
		// Advance to the next token
		tt := tokenizer.Next()
		t := tokenizer.Token()

		switch tt {
		case html.ErrorToken:
			// End of the document (or an actual error)
			fmt.Printf("\nFinished parsing. Found %d links.\n", webpage.linkCount)
			return webpage

		case html.StartTagToken, html.SelfClosingTagToken:
			// We found an opening tag (like <a>, <img>, <div>)

			// Check if the tag is an anchor tag '<a>'
			if t.Data == "a" {
				// Loop through the attributes of the <a> tag looking for 'href'
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						webpage.linkCount++
						webpage.links = append(webpage.links, attr.Val)
						break
					}
				}
			}

			hide := neverVisible[t.Data] || hiddenByattributes(t)
			if hide && tt == html.StartTagToken {
				skipStack = append(skipStack, t.Data)
			}

		case html.EndTagToken:
			if len(skipStack) > 0 && skipStack[len(skipStack)-1] == t.Data {
				skipStack = skipStack[:len(skipStack)-1]
			}

		case html.TextToken:
			if !skipping() {
				text := strings.TrimSpace()
				if text != "" {
					for _, w := range strings.Fields(text) {
						webpage.words = append(webpage.words, w)
						if len(webpage.words) >= 500 {
							return webpage
						}
					}

				}
			}

		}

	}
}

func linkExtractor(url string) ([]string, int) {
	// 1. FETCH: Make the request
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Fatalf("Status code error: %d", res.StatusCode)
	}

	// 2. TOKENIZE: Create a new tokenizer over the response body
	// This does NOT load the whole document into memory!
	z := html.NewTokenizer(res.Body)

	fmt.Println("Extracting links...")
	linkCount := 0
	var links []string

	// 3. LOOP: Stream through the HTML tokens
	for {
		// Advance to the next token
		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			// End of the document (or an actual error)
			fmt.Printf("\nFinished parsing. Found %d links.\n", linkCount)
			return links, linkCount

		case html.StartTagToken, html.SelfClosingTagToken:
			// We found an opening tag (like <a>, <img>, <div>)
			t := z.Token()

			// Check if the tag is an anchor tag '<a>'
			if t.Data == "a" {
				// Loop through the attributes of the <a> tag looking for 'href'
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						linkCount++
						links = append(links, attr.Val)
						break
					}
				}
			}
		}
	}
}

// for now, writing in files to just inspect the data, later on we integrate with mongoDB
func writeInFile(links []string) {
	file, err := os.OpenFile("links.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// 3. Iterate over the slice and write line-by-line
	for _, line := range links {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			log.Fatalf("Failed writing line: %s", err)
		}
	}

	// 4. IMPORTANT: Flush the buffer to ensure all data is written to disk
	err = writer.Flush()
	if err != nil {
		log.Fatalf("Failed flushing buffer: %s", err)
	}

	log.Println("Buffered file written successfully!")
}

func main() {
	url := "https://books.toscrape.com/"
	links, _ := linkExtractor(url)
	for i := range 5 {
		// Just printing the first 10
		fmt.Printf("[%d] %s\n", i+1, links[i])
	}
	writeInFile(links)

	// this is the mongoDB stuff
	// Use the SetServerAPIOptions() method to set the version of the Stable API on the client
	// serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	// if err := godotenv.Load(); err != nil {
	// 	log.Println("No .env file found")
	// }
	// var uri string
	// if uri = os.Getenv("MONGODB_URI"); uri == "" {
	// 	log.Fatal("You must set your 'MONGODB_URI' environment variable. See\n\t https://www.mongodb.com/docs/drivers/go/current/connect/mongoclient/#environment-variable")
	// }
	// opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

	// // Create a new client and connect to the server
	// client, err := mongo.Connect(opts)
	// if err != nil {
	// 	panic(err)
	// }

	// defer func() {
	// 	if err = client.Disconnect(context.TODO()); err != nil {
	// 		panic(err)
	// 	}
	// }()

	// // Send a ping to confirm a successful connection
	// if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
	// 	panic(err)
	// }
	// fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
}
