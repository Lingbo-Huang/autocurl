package main

import (
	"bytes"
	"context"
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

type createOrderRequest struct {
	UserID   int64             `json:"user_id"`
	ShopID   string            `json:"shop_id"`
	Items    []orderItem       `json:"items"`
	Address  shippingAddress   `json:"address"`
	Metadata map[string]string `json:"metadata"`
}

type orderItem struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price_cent"`
}

type shippingAddress struct {
	Province string `json:"province"`
	City     string `json:"city"`
	Detail   string `json:"detail"`
}

func main() {
	defaultURL := os.Getenv("AUTOCURL_DEMO_URL")
	if defaultURL == "" {
		defaultURL = "https://example.com/"
	}
	target := flag.String("url", defaultURL, "HTTP endpoint used by the demo")
	repeat := flag.Int("repeat", 1, "number of GET/POST request pairs to send")
	delay := flag.Duration("delay", 2*time.Second, "delay between repeated request pairs")
	flag.Parse()
	if *repeat < 1 {
		log.Fatal("-repeat must be at least 1")
	}

	client := &http.Client{Timeout: 20 * time.Second}
	ctx := context.Background()

	log.Printf("Autocurl Go demo target: %s", *target)
	log.Print("Start this configuration with Run -> Run Selected with Autocurl")

	for iteration := 1; iteration <= *repeat; iteration++ {
		if *repeat > 1 {
			log.Printf("Request pair %d/%d", iteration, *repeat)
		}
		if err := sendGET(ctx, client, *target); err != nil {
			log.Fatalf("GET failed: %v", err)
		}
		if err := sendPOST(ctx, client, *target); err != nil {
			log.Fatalf("POST failed: %v", err)
		}
		if iteration < *repeat {
			time.Sleep(*delay)
		}
	}

	log.Print("Done. Open the Autocurl tool window and copy either captured cURL.")
	log.Print("The documentation target may return 405 for POST; that is expected and still verifies the complete captured body.")
}

func sendGET(ctx context.Context, client *http.Client, target string) error {
	endpoint, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("parse target: %w", err)
	}
	query := endpoint.Query()
	query.Set("user_id", "10086")
	query.Set("scene", "jetbrains-plugin-demo")
	query.Set("keyword", "large body curl capture")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	addDemoHeaders(request)
	return send(client, request)
}

func sendPOST(ctx context.Context, client *http.Client, target string) error {
	payload := createOrderRequest{
		UserID: 10086,
		ShopID: "shop-autocurl-demo",
		Items: []orderItem{
			{SKU: "sku-red-001", Name: "Mechanical keyboard", Quantity: 1, Price: 69900},
			{SKU: "sku-blue-002", Name: "USB-C cable", Quantity: 3, Price: 3900},
		},
		Address: shippingAddress{
			Province: "Shanghai",
			City:     "Shanghai",
			Detail:   "Autocurl demo address",
		},
		Metadata: map[string]string{
			"source":     "goland",
			"debug_mode": "true",
			"note":       "This nested body should appear in the generated cURL",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	addDemoHeaders(request)
	request.Header.Set("Content-Type", "application/json")

	// Set a breakpoint on the next line to inspect the request before it is sent.
	return send(client, request)
}

func addDemoHeaders(request *http.Request) {
	request.Header.Set("get-info", "true")
	request.Header.Set("X-Demo-Trace-ID", fmt.Sprintf("autocurl-%d", time.Now().UnixMilli()))
	request.Header.Set("Authorization", "Bearer demo-token-not-a-real-secret")
	request.Header.Set("X-API-Key", "demo-api-key")
}

func send(client *http.Client, request *http.Request) error {
	log.Printf("--> %s %s", request.Method, request.URL)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	preview, err := io.ReadAll(io.LimitReader(response.Body, 512))
	if err != nil {
		return err
	}
	log.Printf("<-- %s, response preview: %s", response.Status, strings.TrimSpace(string(preview)))
	return nil
}
