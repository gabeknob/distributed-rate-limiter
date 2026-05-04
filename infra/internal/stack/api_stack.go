package stack

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2authorizers"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2targets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
	"github.com/aws/aws-cdk-go/awscdk/v2/interfaces/interfacesawsec2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type ApiStackProps struct {
	awscdk.StackProps
	Vpc              awsec2.IVpc
	LambdaAuthorizer awslambda.IFunction
}

func NewApiStack(scope constructs.Construct, id string, props *ApiStackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, &props.StackProps)

	vpcLinkSG := awsec2.NewSecurityGroup(
		stack,
		jsii.String("VPC-Link-SG"),
		&awsec2.SecurityGroupProps{
			Vpc: props.Vpc,
		},
	)

	nlbSG := awsec2.NewSecurityGroup(
		stack,
		jsii.String("Api-NLB-SG"),
		&awsec2.SecurityGroupProps{
			Vpc: props.Vpc,
		},
	)

	ec2SG := awsec2.NewSecurityGroup(
		stack,
		jsii.String("Api-EC2-SG"),
		&awsec2.SecurityGroupProps{
			Vpc: props.Vpc,
		},
	)

	nlbSG.Connections().AllowFrom(
		vpcLinkSG,
		awsec2.Port_Tcp(jsii.Number(80)),
		jsii.String("Allow for traffic incoming from vpc links"),
	)

	ec2SG.Connections().AllowFrom(
		nlbSG,
		awsec2.Port_Tcp(jsii.Number(8080)),
		jsii.String("Allow for traffic incoming from the NLB"),
	)

	ec2SG.AddEgressRule(
		awsec2.Peer_AnyIpv4(),
		awsec2.Port_Tcp(jsii.Number(443)),
		jsii.String("Allow HTTPS for SSM and S3 (Binary downloads)"),
		nil,
	)

	apiBinAsset := awss3assets.NewAsset(
		stack,
		jsii.String("Mock-Api-Asset"),
		&awss3assets.AssetProps{
			Path: jsii.String("../mock-api"),
			Bundling: &awscdk.BundlingOptions{
				Image: awscdk.DockerImage_FromRegistry(jsii.String("golang:1.26")),
				Command: &[]*string{
					jsii.String("bash"),
					jsii.String("-c"),
					jsii.String("make build && cp mock-api /asset-output/"),
				},
				User: jsii.String("root"),
			},
		},
	)

	apiEc2Role := awsiam.NewRole(
		stack,
		jsii.String("Api-EC2-Role"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(jsii.String("ec2.amazonaws.com"), nil),
			ManagedPolicies: &[]awsiam.IManagedPolicy{
				awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("AmazonSSMManagedInstanceCore")),
			},
		},
	)

	apiBinAsset.GrantRead(apiEc2Role)

	ec2 := awsec2.NewInstance(
		stack,
		jsii.String("Api-EC2"),
		&awsec2.InstanceProps{
			InstanceType: awsec2.InstanceType_Of(awsec2.InstanceClass_T4G, awsec2.InstanceSize_SMALL),
			MachineImage: awsec2.MachineImage_LatestAmazonLinux2023(&awsec2.AmazonLinux2023ImageSsmParameterProps{
				CpuType: awsec2.AmazonLinuxCpuType_ARM_64,
			}),

			Vpc:           props.Vpc,
			SecurityGroup: ec2SG,
			Role:          apiEc2Role,
			VpcSubnets: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},

			AssociatePublicIpAddress: jsii.Bool(false),
			AllowAllOutbound:         jsii.Bool(false),
		},
	)

	tempZip := "/tmp/mock-api.zip"
	finalBin := "/usr/local/bin/mock-api"

	ec2.UserData().AddS3DownloadCommand(&awsec2.S3DownloadOptions{
		Bucket:    apiBinAsset.Bucket(),
		BucketKey: apiBinAsset.S3ObjectKey(),
		LocalFile: &tempZip,
	})

	ec2.AddUserData(
		jsii.String(fmt.Sprintf("unzip %s -d /tmp/extracted", tempZip)),
		jsii.String(fmt.Sprintf("mv /tmp/extracted/mock-api %s", finalBin)),
		jsii.String("chmod +x "+finalBin),

		jsii.String("printf '[Unit]\nDescription=Mock API\nAfter=network.target\n\n[Service]\nType=simple\nExecStart="+finalBin+"\nRestart=always\n\n[Install]\nWantedBy=multi-user.target' > /etc/systemd/system/mock-api.service"),
		jsii.String("systemctl daemon-reload"),
		jsii.String("systemctl enable mock-api"),
		jsii.String("systemctl start mock-api"),
	)

	ec2Target := awselasticloadbalancingv2targets.NewInstanceTarget(ec2, jsii.Number(8080))

	nlb := awselasticloadbalancingv2.NewNetworkLoadBalancer(
		stack,
		jsii.String("Api-Network-LB"),
		&awselasticloadbalancingv2.NetworkLoadBalancerProps{
			Vpc:            props.Vpc,
			InternetFacing: jsii.Bool(false),
			SecurityGroups: &[]awsec2.ISecurityGroup{nlbSG},
			VpcSubnets: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},
		},
	)

	listener := nlb.AddListener(
		jsii.String("Api-NLB-Listener"),
		&awselasticloadbalancingv2.BaseNetworkListenerProps{
			Port: jsii.Number(80),
		},
	)

	targetGroup := awselasticloadbalancingv2.NewNetworkTargetGroup(
		stack,
		jsii.String("Api-NLB-TargetGroup"),
		&awselasticloadbalancingv2.NetworkTargetGroupProps{
			Vpc:      props.Vpc,
			Port:     jsii.Number(8080),
			Protocol: awselasticloadbalancingv2.Protocol_TCP,
			Targets:  &[]awselasticloadbalancingv2.INetworkLoadBalancerTarget{ec2Target},
			HealthCheck: &awselasticloadbalancingv2.HealthCheck{
				Port:     jsii.String("8080"),
				Protocol: awselasticloadbalancingv2.Protocol_HTTP,
				Path:     jsii.String("/health"),
			},
		},
	)

	listener.AddTargetGroups(jsii.String("Attach-NLB"), targetGroup)

	vpcLink := awsapigatewayv2.NewVpcLink(
		stack,
		jsii.String("Api-VPC-Link"),
		&awsapigatewayv2.VpcLinkProps{
			Vpc:            props.Vpc,
			SecurityGroups: &[]interfacesawsec2.ISecurityGroupRef{vpcLinkSG},
			Subnets: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},
		},
	)

	nlbIntegration := awsapigatewayv2integrations.NewHttpNlbIntegration(
		jsii.String("Api-NLB-Integration"),
		listener,
		&awsapigatewayv2integrations.HttpNlbIntegrationProps{
			VpcLink: vpcLink,
		},
	)

	authorizer := awsapigatewayv2authorizers.NewHttpLambdaAuthorizer(
		jsii.String("Rate-Limiter-Authorizer"),
		props.LambdaAuthorizer,
		&awsapigatewayv2authorizers.HttpLambdaAuthorizerProps{
			ResponseTypes: &[]awsapigatewayv2authorizers.HttpLambdaResponseType{
				awsapigatewayv2authorizers.HttpLambdaResponseType_SIMPLE,
			},
			IdentitySource: &[]*string{
				jsii.String("$request.header.X-Client-Id"),
			},
			ResultsCacheTtl: awscdk.Duration_Millis(jsii.Number(0)),
		},
	)

	httpApi := awsapigatewayv2.NewHttpApi(
		stack,
		jsii.String("Api-ApiGw"),
		&awsapigatewayv2.HttpApiProps{DefaultAuthorizer: authorizer},
	)

	httpApi.AddRoutes(&awsapigatewayv2.AddRoutesOptions{
		Path: jsii.String("/{proxy+}"),
		Methods: &[]awsapigatewayv2.HttpMethod{
			awsapigatewayv2.HttpMethod_ANY,
		},
		Integration: nlbIntegration,
	})

	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
		Value: httpApi.Url(),
	})

	return stack
}
