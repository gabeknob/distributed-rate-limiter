# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Monorepo Structure

```
/
├── rate-limiter/      Spring Boot rate limiter service (Java 21, Maven)
├── mock-api/          Go HTTP server — fake tax receipt API (EC2 target)
├── lambda-authorizer/ Go Lambda — bridges API Gateway to the rate limiter
├── benchmark/         Go CLI load testing tool (runs locally)
└── infra/             Terraform (not yet implemented)
```

Each Go component is an independent module. A `go.work` file at the root links them for IDE support.

## Component Commands

### rate-limiter (Java/Maven)
See `rate-limiter/CLAUDE.md` for the full command reference. Quick reference:
```bash
cd rate-limiter
./mvnw test                          # run all tests (requires Docker)
./mvnw test -Dtest=TokenBucketTest   # run single test class
./mvnw spring-boot:run               # run locally (port 8080)
```

### mock-api (Go)
```bash
cd mock-api
go run .                   # run locally (default port 8080, override with PORT=9090)
make build                 # cross-compile for Linux arm64 (EC2 t4g)
```

### lambda-authorizer (Go)
```bash
cd lambda-authorizer
make build                 # compile Linux arm64 binary named 'bootstrap'
make zip                   # build + zip into function.zip for Lambda upload
# Required env var: RATE_LIMITER_URL (e.g. http://internal-alb-dns)
```

### benchmark (Go)
```bash
cd benchmark
go run . -url https://<api-gw-url> -clients 20 -rps 100 -duration 30 -client-id test-1
make build                 # compile native binary
```

## Architecture

```
Internet → API Gateway (HTTP API)
               │
               ▼ (Lambda Authorizer on every request, TTL=0)
         Lambda Authorizer (Go, VPC-attached)
               │  POST /api/ratelimit/check  {"key": X-Client-Id, "tokens": 1}
               ▼
         Internal ALB → ECS Fargate (rate-limiter) → ElastiCache Redis
               │
               │ isAuthorized: true/false
               ▼ (if authorized)
         HTTP Integration → Mock API (EC2 t4g.small, Go)
```

**Key design decisions:**
- Lambda Authorizer TTL must be 0 — any cache TTL bypasses rate limiting entirely
- Rate limit key is the `X-Client-Id` header value — each client gets its own token bucket
- Fail-open on rate limiter errors (configurable in lambda-authorizer/main.go)
- ECS tasks sit in public subnets for now; move to private + VPC endpoints in Terraform step

## Rate Limiter Configuration (ECS env vars)

| Variable | Value |
|---|---|
| `RATELIMITER_SECURITY_IP_WHITELIST` | *(empty)* |
| `RATELIMITER_SECURITY_API_KEYS_ENABLED` | `false` |
| `RATELIMITER_CAPACITY` | `100` |
| `RATELIMITER_REFILLRATE` | `10` |
| `MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE` | `health,metrics` |

## AWS Infrastructure

- **Region**: us-east-1
- **VPC**: 1 private subnet, 2 public subnets
- **ECS**: Fargate tasks in public subnets (SG restricts inbound to ALB only)
- **Redis**: ElastiCache OSS in private subnet
- **ALB**: Currently external; will be replaced with internal ALB in Step 1
- **Lambda**: Will be VPC-attached to reach the internal ALB
