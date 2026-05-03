package stack

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type NetworkStack struct {
	Stack awscdk.Stack
	Vpc   awsec2.IVpc
}

func NewNetworkStack(scope constructs.Construct, id string, props *awscdk.StackProps) *NetworkStack {
	stack := awscdk.NewStack(scope, &id, props)

	vpc := awsec2.NewVpc(stack, jsii.String("Rate-Limiter-VPC"), &awsec2.VpcProps{
		IpAddresses:           awsec2.IpAddresses_Cidr(jsii.String("10.0.0.0/16")),
		CreateInternetGateway: jsii.Bool(true),
		EnableDnsHostnames:    jsii.Bool(true),
		EnableDnsSupport:      jsii.Bool(true),
		MaxAzs:                jsii.Number(2),
		NatGateways:           jsii.Number(1),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("Rate-Limiter-Public"),
				CidrMask:   jsii.Number(24),
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
			{
				Name:       jsii.String("Rate-Limiter-Private"),
				CidrMask:   jsii.Number(24),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},
		},
	})

	return &NetworkStack{
		Stack: stack,
		Vpc:   vpc,
	}
}
