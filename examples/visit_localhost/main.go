package main

import (
	"fmt"
	"log"

	omoikane "github.com/ieee0824/omoikane-go"
)

func main() {
	browser, err := omoikane.NewBrowser(omoikane.Options{
		UserAgent: "omoikane-go-example/0.2.2",
	})
	if err != nil {
		log.Fatalf("create browser: %v", err)
	}
	defer browser.Close()

	url := "http://localhost:8080"
	if err := browser.Navigate(url); err != nil {
		log.Fatalf("navigate to %s: %v", url, err)
	}

	title, err := browser.Evaluate("document.title")
	if err != nil {
		log.Fatalf("read document.title: %v", err)
	}

	bodyText, err := browser.Evaluate("document.body ? document.body.innerText : null")
	if err != nil {
		log.Fatalf("read document.body.innerText: %v", err)
	}

	content, err := browser.Content()
	if err != nil {
		log.Fatalf("read page content: %v", err)
	}

	fmt.Printf("URL: %s\n", url)
	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Body: %s\n", bodyText)
	fmt.Printf("HTML:\n%s\n", content)
}
