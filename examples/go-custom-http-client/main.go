package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type demoRequest struct {
	UserID int64             `json:"user_id"`
	Scene  string            `json:"scene"`
	Items  []string          `json:"items"`
	Meta   map[string]string `json:"meta"`
}

func main() {
	defaultURL := os.Getenv("AUTOCURL_DEMO_URL")
	if defaultURL == "" {
		defaultURL = "https://example.com/"
	}

	target := flag.String("url", defaultURL, "HTTP endpoint used by the demo")
	proxyMode := flag.String(
		"proxy-mode",
		"ignore-env",
		"ignore-env or explicit-env-proxy",
	)
	flag.Parse()

	transport := &http.Transport{
		// Unlike http.DefaultTransport, this custom transport deliberately
		// ignores HTTP_PROXY and HTTPS_PROXY.
		Proxy:               nil,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}

	switch *proxyMode {
	case "ignore-env":
		log.Print("proxy mode: ignore-env (the request stays direct and is not captured)")
	case "explicit-env-proxy":
		proxyURL, err := captureProxyFor(*target)
		if err != nil {
			log.Fatal(err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		log.Printf("proxy mode: explicit-env-proxy (%s)", proxyURL)
	default:
		log.Fatalf("unknown -proxy-mode %q", *proxyMode)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
	}
	if err := sendPOST(context.Background(), client, *target); err != nil {
		log.Fatal(err)
	}
}

func captureProxyFor(target string) (*url.URL, error) {
	endpoint, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse target: %w", err)
	}
	var raw string
	if endpoint.Scheme == "https" {
		raw = firstNonEmpty(os.Getenv("HTTPS_PROXY"), os.Getenv("https_proxy"))
	} else {
		raw = firstNonEmpty(os.Getenv("HTTP_PROXY"), os.Getenv("http_proxy"))
	}
	if raw == "" {
		return nil, fmt.Errorf(
			"no process-scoped proxy found; start this configuration through Autocurl",
		)
	}
	proxyURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse capture proxy: %w", err)
	}
	return proxyURL, nil
}

func sendPOST(ctx context.Context, client *http.Client, target string) error {
	payload := demoRequest{
		UserID: 10086,
		Scene:  "custom-go-http-client",
		Items:  []string{"header", "query", "nested-body"},
		Meta: map[string]string{
			"expected_when_ignore_env":         "request succeeds but Autocurl shows no row",
			"expected_when_explicit_env_proxy": "Autocurl captures this complete JSON body",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		target,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("get-info", "true")
	request.Header.Set("Authorization", "Bearer demo-token-not-a-real-secret")

	log.Printf("--> %s %s", request.Method, request.URL)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	preview, err := io.ReadAll(io.LimitReader(response.Body, 256))
	if err != nil {
		return err
	}
	log.Printf(
		"<-- %s, response preview: %s",
		response.Status,
		strings.TrimSpace(string(preview)),
	)
	log.Print("Done.")
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
