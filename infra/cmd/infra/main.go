package main

import (
	"os"

	stack "github.com/gabeknob/distributed-rate-limiter/infra/internal/stack"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

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
		StackProps: *defaultProps,
		Vpc:        network.Vpc,

		RedisSecurityGroup: data.RedisSecurityGroup,

		RateLimiterContainerEnv: &stack.RateLimiterContainerEnv{
			RedisHost:              data.RedisEndpointAddress,
			RedisPort:              data.RedisEndpointPort,
			LimiterCapacity:        jsii.String("10"),
			LimiterRefillRate:      jsii.String("1"),
			IpWhitelist:            jsii.String(""),
			ExposedEndpoints:       jsii.String("health,metrics"),
			RateLimiterAlgorithm:   jsii.String("TOKEN_BUCKET"),
			SecurityApiKeysEnabled: jsii.String("false"),
		},
	})
	stack.NewApiStack(app, "ApiStack", &stack.ApiStackProps{
		StackProps:       *defaultProps,
		Vpc:              network.Vpc,
		LambdaAuthorizer: rateLimiter.AuthorizerLambda,
	})

	app.Synth(nil)
}
