# Distributed Rate Limiter on AWS

This project provides a high performance infrastructure for distributed rate limiting on AWS, utilizing the core logic from the `uppnrise/distributed-rate-limiter` repository. It is designed to manage request quotas across multiple clients in a distributed environment, ensuring system stability and resource protection.

## Architecture

The system utilizes a multi-layer approach to ensure low latency and high availability. It integrates AWS Lambda for authorization, ECS Fargate for the core rate limiting service, and ElastiCache (Valkey) for centralized state management.

![Diagram](https://github.com/user-attachments/assets/e49b5c39-d1a8-4b03-be77-1727526971e6)

Requests are intercepted at the API Gateway level by a Lambda Authorizer, which communicates with the Rate Limiter service via a Private VPC Link. This ensures that all traffic remains within the internal AWS network.

## Implementation

The underlying service provides several algorithms to control traffic flow, including Fixed Window and Leaky Bucket implementations. For this specific deployment, the **Token Bucket** algorithm was selected to allow for controlled bursts of traffic while maintaining a strict long term average rate.

The Rate Limiter service is built with Spring Boot and integrates with a Valkey based ElastiCache cluster to store token states. This allows multiple ECS tasks to share the same quota information, providing a consistent rate limit across the entire cluster.

## Requirements

The following tools and configurations are required to build and deploy the infrastructure:

- **AWS CLI**: Configured with an identity that possesses permissions for IAM, VPC, ECS, Lambda, and ElastiCache management.
- **AWS CDK**: Installed globally to handle the Infrastructure as Code (IaC) deployment.
- **Golang**: Version 1.26 or higher is required to build the Lambda Authorizer and the CDK infrastructure binary.
- **Docker**: Necessary for CDK bundling processes and for building the Rate Limiter container image.
- **Make**: Used for automating the build and cleanup processes for the binaries.

## Environment Variables

The infrastructure supports dynamic configuration via environment variables during the deployment phase. If a variable is not provided in the terminal, the following default values are applied:

| Variable               | Description                           | Default Value  |
| :--------------------- | :------------------------------------ | :------------- |
| `RL_CAPACITY`          | Maximum tokens in the bucket          | 10             |
| `RL_REFILL_RATE`       | Tokens added to the bucket per second | 1              |
| `RL_ALGORITHM`         | Selected rate limiting algorithm      | TOKEN_BUCKET   |
| `RL_EXPOSED_ENDPOINTS` | Management endpoints to expose        | health,metrics |
| `RL_API_KEYS_ENABLED`  | Toggle for API key security           | false          |
| `RL_IP_WHITELIST`      | List of allowed IP addresses          | (empty)        |

## Usage

### API Access

Once the `ApiStack` is deployed and the api target group is healthy, you can access the protected endpoints using a standard `curl` command. Ensure you include the `X-Client-Id` header to identify the request source:

```bash
curl -i -H "X-Client-Id: your-client-id" https://your-api-id.execute-api.us-east-1.amazonaws.com/receipts
```

### Benchmarking

To perform a stress test and verify the rate limiting logic, use the provided Go benchmark tool. This tool simulates multiple independent clients to verify that the distributed state is correctly managed:

```bash
go run main.go \
  -url https://ffzrxupwal.execute-api.us-east-1.amazonaws.com \
  -clients 5 \
  -rps 50 \
  -duration 60 \
  -client-id load-test
```
