package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "", "API Gateway invoke URL (required)")
	clients := flag.Int("clients", 10, "number of concurrent goroutines")
	rps := flag.Int("rps", 50, "target requests per second")
	duration := flag.Int("duration", 30, "test duration in seconds")
	clientID := flag.String("client-id", "benchmark-client", "value for X-Client-Id header")
	flag.Parse()

	if *url == "" {
		log.Fatal("-url is required")
	}

	endpoint := *url + "/receipts"
	interval := time.Duration(float64(time.Second) / float64(*rps) * float64(*clients))
	deadline := time.Now().Add(time.Duration(*duration) * time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var (
		total       atomic.Int64
		allowed     atomic.Int64
		rateLimited atomic.Int64
		errors      atomic.Int64
		latencySum  atomic.Int64 // microseconds
	)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	sem := make(chan struct{}, *clients)
	var wg sync.WaitGroup

	// Print stats every 5 seconds
	go func() {
		t := time.NewTicker(5 * time.Second)
		elapsed := 0
		for range t.C {
			elapsed += 5
			fmt.Printf("[%ds] Total: %d | Allowed: %d | Rate Limited (403): %d | Errors: %d | Avg: %.1fms\n",
				elapsed,
				total.Load(),
				allowed.Load(),
				rateLimited.Load(),
				errors.Load(),
				float64(latencySum.Load())/float64(max(total.Load(), 1))/1000.0,
			)
		}
	}()

	for time.Now().Before(deadline) {
		<-ticker.C
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
			req.Header.Set("X-Client-Id", *clientID)

			start := time.Now()
			resp, err := httpClient.Do(req)
			elapsed := time.Since(start)
			latencySum.Add(elapsed.Microseconds())
			total.Add(1)

			if err != nil {
				errors.Add(1)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				allowed.Add(1)
			case http.StatusForbidden: // 403 = rate limited (Lambda authorizer denied)
				rateLimited.Add(1)
			default:
				errors.Add(1)
			}
		}()
	}

	wg.Wait()

	fmt.Println("\n--- Final Results ---")
	fmt.Printf("Total:        %d\n", total.Load())
	fmt.Printf("Allowed:      %d (%.1f%%)\n", allowed.Load(), pct(allowed.Load(), total.Load()))
	fmt.Printf("Rate Limited: %d (%.1f%%)\n", rateLimited.Load(), pct(rateLimited.Load(), total.Load()))
	fmt.Printf("Errors:       %d (%.1f%%)\n", errors.Load(), pct(errors.Load(), total.Load()))
	fmt.Printf("Avg Latency:  %.1fms\n", float64(latencySum.Load())/float64(max(total.Load(), 1))/1000.0)
}

func pct(n, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
