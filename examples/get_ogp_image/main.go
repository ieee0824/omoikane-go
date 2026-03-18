package main

import (
	"fmt"
	"html"
	"log"
	"os"
	"regexp"
	"strings"

	omoikane "github.com/ieee0824/omoikane-go"
)

var metaTagPattern = regexp.MustCompile(`(?is)<meta\b[^>]*>`)

func main() {
	url := "https://example.com"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	browser, err := omoikane.NewBrowser()
	if err != nil {
		log.Fatalf("create browser: %v", err)
	}
	defer browser.Close()

	if err := browser.Navigate(url); err != nil {
		log.Fatalf("navigate to %s: %v", url, err)
	}

	content, err := browser.Content()
	if err != nil {
		log.Fatalf("read page content: %v", err)
	}

	fmt.Printf("URL: %s\n", url)
	imageURL := findMetaContent(content,
		"property", "og:image:secure_url",
		"property", "og:image",
		"name", "twitter:image",
		"name", "twitter:image:src",
	)
	if imageURL == "" {
		fmt.Println("OGP Image: not found")
		return
	}

	fmt.Printf("OGP Image: %s\n", imageURL)
}

func findMetaContent(document string, pairs ...string) string {
	for _, tag := range metaTagPattern.FindAllString(document, -1) {
		for i := 0; i+1 < len(pairs); i += 2 {
			if metaValue(tag, pairs[i]) == pairs[i+1] {
				content := metaValue(tag, "content")
				if content != "" {
					return content
				}
			}
		}
	}
	return ""
}

func metaValue(tag string, attr string) string {
	pattern := regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(attr) + `\s*=\s*("([^"]*)"|'([^']*)')`)
	match := pattern.FindStringSubmatch(tag)
	if len(match) < 4 {
		return ""
	}

	value := match[2]
	if value == "" {
		value = match[3]
	}

	return html.UnescapeString(strings.TrimSpace(value))
}
