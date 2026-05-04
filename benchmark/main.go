package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "", "API Gateway invoke URL (required)")
	clients := flag.Int("clients", 10, "number of independent clients, each with its own rate-limit bucket")
	rps := flag.Int("rps", 50, "total target requests per second (divided evenly across clients)")
	duration := flag.Int("duration", 30, "test duration in seconds")
	clientIDPrefix := flag.String("client-id", "", "prefix for client IDs (e.g. 'load-test' → 'load-test-1', 'load-test-2'); default: random UUIDs")
	flag.Parse()

	if *url == "" {
		log.Fatal("-url is required")
	}

	clientIDs := make([]string, *clients)
	for i := range clientIDs {
		if *clientIDPrefix != "" {
			clientIDs[i] = fmt.Sprintf("%s-%d", *clientIDPrefix, i+1)
		} else {
			clientIDs[i] = newUUID()
		}
	}

	if err := os.MkdirAll("output", 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	timestamp := time.Now().Format("2006-01-02_150405")
	label := *clientIDPrefix
	if label == "" {
		label = clientIDs[0][:8] // first 8 chars of first UUID
	}
	filename := fmt.Sprintf("output/%s_%s_%dc_%drps.txt", timestamp, label, *clients, *rps)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer file.Close()

	out := io.MultiWriter(os.Stdout, file)

	fmt.Fprintf(out, "=== Benchmark Run ===\n")
	fmt.Fprintf(out, "Time:       %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(out, "URL:        %s\n", *url)
	fmt.Fprintf(out, "Clients:    %d (each with own bucket)\n", *clients)
	fmt.Fprintf(out, "Total RPS:  %d (~%d RPS per client)\n", *rps, *rps / *clients)
	fmt.Fprintf(out, "Duration:   %ds\n", *duration)
	if *clientIDPrefix != "" {
		fmt.Fprintf(out, "Client-Ids: %s-1 ... %s-%d\n", *clientIDPrefix, *clientIDPrefix, *clients)
	} else {
		fmt.Fprintf(out, "Client-Ids: %s ... (+%d more UUIDs)\n", clientIDs[0], *clients-1)
	}
	fmt.Fprintf(out, "=====================\n\n")

	endpoint := *url + "/receipts"
	deadline := time.Now().Add(time.Duration(*duration) * time.Second)
	// Each client fires every (clients/rps) seconds to achieve rps/clients per client
	perClientInterval := time.Duration(float64(time.Second) * float64(*clients) / float64(*rps))

	var (
		total       atomic.Int64
		allowed     atomic.Int64
		rateLimited atomic.Int64
		errors      atomic.Int64
		latencySum  atomic.Int64 // microseconds
	)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	var wg sync.WaitGroup

	for _, id := range clientIDs {
		wg.Add(1)
		go func(clientID string) {
			defer wg.Done()
			ticker := time.NewTicker(perClientInterval)
			defer ticker.Stop()
			for time.Now().Before(deadline) {
				<-ticker.C
				go func() {
					req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
					req.Header.Set("X-Client-Id", clientID)

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
					case http.StatusForbidden: // 403 = rate limited
						rateLimited.Add(1)
					default:
						errors.Add(1)
					}
				}()
			}
		}(id)
	}

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		elapsed := 0
		for {
			select {
			case <-t.C:
				elapsed += 5
				fmt.Fprintf(out, "[%ds] Total: %d | Allowed: %d | Rate Limited (403): %d | Errors: %d | Avg: %.1fms\n",
					elapsed,
					total.Load(),
					allowed.Load(),
					rateLimited.Load(),
					errors.Load(),
					float64(latencySum.Load())/float64(max(total.Load(), 1))/1000.0,
				)
			case <-done:
				return
			}
		}
	}()

	wg.Wait()
	close(done)

	fmt.Fprintf(out, "\n--- Final Results ---\n")
	fmt.Fprintf(out, "Total:        %d\n", total.Load())
	fmt.Fprintf(out, "Allowed:      %d (%.1f%%)\n", allowed.Load(), pct(allowed.Load(), total.Load()))
	fmt.Fprintf(out, "Rate Limited: %d (%.1f%%)\n", rateLimited.Load(), pct(rateLimited.Load(), total.Load()))
	fmt.Fprintf(out, "Errors:       %d (%.1f%%)\n", errors.Load(), pct(errors.Load(), total.Load()))
	fmt.Fprintf(out, "Avg Latency:  %.1fms\n", float64(latencySum.Load())/float64(max(total.Load(), 1))/1000.0)

	fmt.Printf("\nOutput saved to %s\n", filename)
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

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
