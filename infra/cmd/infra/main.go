package main

import (
	"os"

	stack "github.com/gabeknob/distributed-rate-limiter/infra/internal/stack"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

func getEnv(key, fallback string) *string {
	value := os.Getenv(key)
	if value == "" {
		return jsii.String(fallback)
	}
	return jsii.String(value)
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	env := &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}

	defaultProps := &awscdk.StackProps{Env: env}

	network := stack.NewNetworkStack(app, "NetworkStack", defaultProps)
	data := stack.NewDataStack(app, "DataStack", &stack.DataStackProps{
		StackProps: *defaultProps,
		Vpc:        network.Vpc,
	})
	rateLimiter := stack.NewRateLimiterStack(app, "RateLimiterStack", &stack.RateLimiterStackProps{
		StackProps:         *defaultProps,
		Vpc:                network.Vpc,
		RedisSecurityGroup: *data.RedisSecurityGroup,

		RateLimiterContainerEnv: &stack.RateLimiterContainerEnv{
			RedisHost:              data.RedisEndpointAddress,
			RedisPort:              data.RedisEndpointPort,
			LimiterCapacity:        getEnv("RL_CAPACITY", "10"),
			LimiterRefillRate:      getEnv("RL_REFILL_RATE", "1"),
			IpWhitelist:            getEnv("RL_IP_WHITELIST", ""),
			ExposedEndpoints:       getEnv("RL_EXPOSED_ENDPOINTS", "health,metrics"),
			RateLimiterAlgorithm:   getEnv("RL_ALGORITHM", "TOKEN_BUCKET"),
			SecurityApiKeysEnabled: getEnv("RL_API_KEYS_ENABLED", "false"),
		},
	})
	stack.NewApiStack(app, "ApiStack", &stack.ApiStackProps{
		StackProps:       *defaultProps,
		Vpc:              network.Vpc,
		LambdaAuthorizer: rateLimiter.AuthorizerLambda,
	})

	app.Synth(nil)
}
