# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

**Requirements**: Java 21+ and Docker (for Testcontainers)

```bash
# Build & verify (runs all tests, ~120s)
./mvnw clean install

# Run all tests (~60s, includes Testcontainers Redis startup)
./mvnw test

# Run a single test class
./mvnw test -Dtest=TokenBucketTest

# Run tests matching a pattern
./mvnw test -Dtest=Redis*

# Skip tests (fast packaging)
./mvnw clean package -DskipTests

# Code coverage report → target/site/jacoco/index.html
./mvnw jacoco:report
./mvnw jacoco:check   # enforce 50% min coverage threshold

# Static analysis
./mvnw clean compile spotbugs:check
./mvnw pmd:check
./mvnw checkstyle:check   # Google style guide

# Security scan (OWASP, CVSS ≥ 7 fails build)
./mvnw dependency-check:check

# Run the application (port 8080)
./mvnw spring-boot:run

# With Redis via Docker Compose
docker-compose up -d redis && ./mvnw spring-boot:run

# Gatling load tests (disabled by default, requires Docker, 16GB+ RAM)
./mvnw gatling:test -Dgatling.simulationClass=dev.bnacar.distributedratelimiter.loadtest.BasicLoadTest

# React dashboard
cd examples/web-dashboard && npm install && npm run dev   # port 5173
```

## Architecture

### Request Flow

```
POST /api/ratelimit/check
  → RateLimitController
  → RateLimiterService (resolves config via ConfigurationResolver)
  → RedisRateLimiterBackend (or InMemoryRateLimiterBackend as fallback)
  → Response: {allowed, tokensRequested, key}
```

**Fail-open**: When Redis is unavailable, the service automatically falls back to `InMemoryRateLimiterBackend` and continues serving requests (state lost on restart).

### Configuration Hierarchy (highest → lowest priority)

1. **Per-key exact match** — `ratelimiter.keys.{key}.capacity`
2. **Pattern-based wildcard** — `ratelimiter.patterns.user:*.capacity`
3. **Global defaults** — `ratelimiter.capacity`, `ratelimiter.refillRate`

Resolved at runtime by `ConfigurationResolver.java`.

### Rate Limiting Algorithms

| Algorithm | Default? | Best For |
|-----------|----------|----------|
| Token Bucket | Yes | General APIs with burst tolerance |
| Sliding Window | — | Strict rate enforcement |
| Fixed Window | — | Memory efficiency, high scale |
| Leaky Bucket | — | Traffic shaping, constant output |
| Composite | — | Enterprise multi-constraint scenarios |

**Composite** combines multiple algorithms via `CompositeRateLimiter.java` with 5 combination logics: `ALL_MUST_PASS`, `ANY_CAN_PASS`, `WEIGHTED_AVERAGE`, `HIERARCHICAL_AND`, `PRIORITY_BASED`.

### Redis Implementation

- Atomic operations implemented via **Lua scripts** in `RedisRateLimiterBackend.java` — prevents race conditions during refill-and-consume
- Redis data structure: Hash fields (`tokens`, `lastRefillTime`, `capacity`, `refillRate`) with TTL
- Connection pooling configured via `spring.data.redis.lettuce.pool.*`

### Key Source Packages

All under `src/main/java/dev/bnacar/distributedratelimiter/`:

- `ratelimit/` — algorithms, services, backends, `ConfigurationResolver`
- `controller/` — 8 REST controllers (RateLimit, Config, Admin, Benchmark, Performance, Geographic, Adaptive, Schedule)
- `geo/` — geographic rate limiting, CDN header parsing (CloudFlare/CloudFront/Azure)
- `adaptive/` — ML-driven adaptive limiting (`AdaptiveRateLimitEngine`, `AdaptiveMLModel`, `AnomalyDetector`)
- `monitoring/` — `MetricsService` (Prometheus), `PerformanceRegressionService`
- `security/` — `SecurityFilter`, `IpSecurityService`, `ApiKeyService`
- `observability/` — correlation ID filter
- `config/` — Spring configuration beans

### Testing

- **67 test classes**, organized to mirror main source packages
- Integration tests use **Testcontainers** (`@Testcontainers`) — Docker required
- Time-based assertions use **Awaitility** (not `Thread.sleep`)
- Test profile: `application-test.properties` (disables geographic limiting)
- Performance tests (`*LoadTest*`, `MemoryUsageTest`) require `MAVEN_OPTS=-Xmx4g`

### REST API Surface

- `/api/ratelimit/check` — core rate limit check
- `/api/ratelimit/config/*` — runtime config CRUD
- `/api/admin/*` — key resets, shutdown
- `/api/performance/*` — real-time metrics and regression detection
- `/api/benchmark/*` — integrated load testing
- `/api/geographic/*` — geographic rule management
- `/api/adaptive/*` — adaptive limit adjustments
- `/actuator/*` — Spring Boot health, Prometheus metrics

Swagger UI: http://localhost:8080/swagger-ui/index.html

## Troubleshooting

- **Build fails**: Verify `java -version` shows 21.x — Java 17 will not compile this project
- **Tests fail**: Ensure Docker daemon is running (Testcontainers requires it)
- **OOM in tests**: Set `MAVEN_OPTS=-Xmx4g` before running the full test suite
- **Redis DOWN in health**: Check `spring.data.redis.*` properties; service will still function via in-memory fallback
