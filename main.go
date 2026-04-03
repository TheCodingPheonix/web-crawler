// TODO: frontier queue, seed urls, actual scraping, telling program to look for links, prioritisation of links?, go to next links, do same
// TODO: politness
// TODO: later, maybe try using goquery instead and check how that effects performance, just using html standard for now
package main

import (
	"fmt"
	"log"
	"net/http"

	"golang.org/x/net/html"
)

func main() {
	url := "https://books.toscrape.com/"

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

	// 3. LOOP: Stream through the HTML tokens
	for {
		// Advance to the next token
		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			// End of the document (or an actual error)
			fmt.Printf("\nFinished parsing. Found %d links.\n", linkCount)
			return

		case html.StartTagToken, html.SelfClosingTagToken:
			// We found an opening tag (like <a>, <img>, <div>)
			t := z.Token()

			// Check if the tag is an anchor tag '<a>'
			if t.Data == "a" {
				// Loop through the attributes of the <a> tag looking for 'href'
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						linkCount++
						if linkCount <= 10 { // Just printing the first 10
							fmt.Printf("[%d] %s\n", linkCount, attr.Val)
						}
						break
					}
				}
			}
		}
	}
}
