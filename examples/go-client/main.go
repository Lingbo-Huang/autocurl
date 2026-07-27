package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	target := os.Getenv("AUTOCURL_DEMO_URL")
	if target == "" {
		target = "https://example.com"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer response.Body.Close()
	fmt.Println(response.StatusCode)
}
