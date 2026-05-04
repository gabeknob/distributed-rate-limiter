# Distributed Rate Limiter on AWS

This project provides a high performance infrastructure for distributed rate limiting on AWS, utilizing the core logic from the `uppnrise/distributed-rate-limiter` repository as a foundation. It is designed to manage request quotas across multiple clients in a distributed environment, ensuring system stability and resource protection.

## Architecture

The system utilizes a multi-layer approach to ensure low latency and high availability. It integrates AWS Lambda for authorization, ECS Fargate for the core rate limiting service, and ElastiCache (Valkey) for centralized state management.

![Diagram](https://github.com/user-attachments/assets/e49b5c39-d1a8-4b03-be77-1727526971e6)

Requests are intercepted at the API Gateway level by a Lambda Authorizer, which communicates with the Rate Limiter service via a Private VPC Link. This ensures that all traffic remains within the internal AWS network. The architecture follows a fail-open strategy: if the rate limiter service is unreachable, the authorizer defaults to allowing the request to prioritize availability.

The underlying service provides several algorithms to control traffic flow, including Fixed Window and Leaky Bucket implementations. For this specific deployment, the **Token Bucket** algorithm was selected to allow for controlled bursts of traffic while maintaining a strict long-term average rate.

The Rate Limiter service is built with Spring Boot and integrates with a Valkey based ElastiCache cluster to store token states. This allows multiple ECS tasks to share the same quota information, providing a consistent rate limit across the entire cluster.

## Implementation

This repository is structured as a monorepo, encapsulating the infrastructure definition, authorization logic, mock services, and performance testing tools.

### Lambda Authorizer

The `lambda-authorizer` directory contains a custom AWS Lambda function written in Go that acts as the gatekeeper at the API Gateway level. It intercepts incoming traffic, extracts the `X-Client-Id` header, and queries the internal Rate Limiter service to verify quotas. A critical feature of this function is its fail open mechanism. If the internal rate limiter service times out or returns an error, the Lambda catches the exception, logs it, and explicitly allows the request to proceed. This design guarantees that temporary internal connectivity issues do not block legitimate client traffic.

### CDK

The CDK implementation located in `/infra` is organized to separate infrastructure definition from execution logic:

- **cmd/infra/**: Contains the `main.go` entry point for the CDK application. It handles environment variable ingestion and stack instantiation.
- **internal/stack/**: Contains the individual CDK stack definitions (Network, Data, RateLimiter, and Api).

#### Requirements

The following tools and configurations are required to build and deploy the infrastructure:

- **AWS CLI**: Configured with an identity possessing permissions for IAM, VPC, ECS, Lambda, and ElastiCache management.
- **AWS CDK**: Installed globally to handle the Infrastructure as Code (IaC) deployment.
- **Golang**: Version 1.26 or higher is required to build the Lambda Authorizer and the CDK infrastructure binary.
- **Docker**: Necessary for CDK bundling processes and for building the Rate Limiter container image.
- **Make**: Used for automating the build and cleanup processes for the binaries.

#### Environment Variables

The infrastructure supports dynamic configuration via environment variables. To streamline the process, the `cdk.json` is configured to automatically compile the Go source into a standalone binary before synthesis.

##### Default Values

If a variable is not provided in the terminal, the following default values are applied:

| Variable               | Description                           | Default Value  |
| :--------------------- | :------------------------------------ | :------------- |
| `RL_CAPACITY`          | Maximum tokens in the bucket          | 10             |
| `RL_REFILL_RATE`       | Tokens added to the bucket per second | 1              |
| `RL_ALGORITHM`         | Selected rate limiting algorithm      | TOKEN_BUCKET   |
| `RL_EXPOSED_ENDPOINTS` | Management endpoints to expose        | health,metrics |
| `RL_API_KEYS_ENABLED`  | Toggle for API key security           | false          |
| `RL_IP_WHITELIST`      | List of allowed IP addresses          | (empty)        |

##### Deployment

The project uses a self-compiling deployment strategy. When you run a CDK command, it triggers the build of the `cmd/infra/main.go` binary.

**Standard Deployment:**

```bash
cdk deploy --all
```

**Deployment with Overrides:**

```bash
RL_CAPACITY=50 RL_REFILL_RATE=5 cdk deploy RateLimiterStack
```

#### Usage

##### API Access

Once the `ApiStack` is deployed and the target group is healthy, endpoints can be accessed using `curl`. The `X-Client-Id` header is required:

```bash
curl -i -H "X-Client-Id: dev-user-01" {Your API URL}/receipts
```

### Benchmark

To perform a stress test and verify the rate limiting logic, use the provided Go benchmark tool located in the `/benchmark` directory. This tool simulates a realistic environment by utilizing multiple independent clients with distinct user IDs to ensure that the distributed state is correctly managed. You can easily configure the load by adjusting parameters such as the Target Requests Per Second (RPS), the number of concurrent clients, and the duration of the test:

```bash
go run main.go \
  -url {Your API URL} \
  -clients 5 \
  -rps 50 \
  -duration 60 \
  -client-id load-test
```
