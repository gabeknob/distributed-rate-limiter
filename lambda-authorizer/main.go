package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// rateLimitRequest matches the Spring Boot RateLimitRequest model.
type rateLimitRequest struct {
	Key    string `json:"key"`
	Tokens int    `json:"tokens"`
}

// rateLimitResponse matches the Spring Boot RateLimitResponse model.
type rateLimitResponse struct {
	Key             string `json:"key"`
	TokensRequested int    `json:"tokensRequested"`
	Allowed         bool   `json:"allowed"`
}

var (
	rateLimiterURL = mustEnv("RATE_LIMITER_URL") // e.g. http://internal-alb-dns
	httpClient     = &http.Client{Timeout: 5 * time.Second}
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func checkRateLimit(clientID string) (bool, error) {
	body, err := json.Marshal(rateLimitRequest{Key: clientID, Tokens: 1})
	if err != nil {
		return false, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := httpClient.Post(
		rateLimiterURL+"/api/ratelimit/check",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return false, fmt.Errorf("rate limiter call failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read response: %w", err)
	}

	var result rateLimitResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("unmarshal response (status %d, body: %s): %w", resp.StatusCode, string(data), err)
	}

	return result.Allowed, nil
}

func handler(_ context.Context, event events.APIGatewayV2CustomAuthorizerV2Request) (events.APIGatewayV2CustomAuthorizerSimpleResponse, error) {
	clientID := event.Headers["x-client-id"]
	if clientID == "" {
		log.Println("missing X-Client-Id header — denying request")
		return events.APIGatewayV2CustomAuthorizerSimpleResponse{IsAuthorized: false}, nil
	}

	allowed, err := checkRateLimit(clientID)
	if err != nil {
		// Fail open: log the error but allow the request through.
		// Change to IsAuthorized: false to fail closed if preferred.
		log.Printf("rate limiter error for client %s: %v — failing open", clientID, err)
		return events.APIGatewayV2CustomAuthorizerSimpleResponse{IsAuthorized: true}, nil
	}

	log.Printf("client=%s allowed=%v", clientID, allowed)
	return events.APIGatewayV2CustomAuthorizerSimpleResponse{IsAuthorized: allowed}, nil
}

func main() {
	lambda.Start(handler)
}
