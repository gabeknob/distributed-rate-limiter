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

	stack.NewNetworkStack(app, "NetworkStack", defaultProps)

	app.Synth(nil)
}
