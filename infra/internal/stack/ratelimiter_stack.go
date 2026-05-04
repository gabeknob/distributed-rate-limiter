package stack

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type RateLimiterContainerEnv struct {
	RedisHost              *string
	RedisPort              *string
	LimiterCapacity        *string
	LimiterRefillRate      *string
	IpWhitelist            *string
	ExposedEndpoints       *string
	RateLimiterAlgorithm   *string
	SecurityApiKeysEnabled *string
}

type RateLimiterStackProps struct {
	awscdk.StackProps
	*RateLimiterContainerEnv

	Vpc                awsec2.IVpc
	RedisSecurityGroup awsec2.SecurityGroup
}

type RateLimiterStack struct {
	awscdk.Stack
	AuthorizerLambda awslambda.IFunction
}

func NewRateLimiterStack(scope constructs.Construct, id string, props *RateLimiterStackProps) *RateLimiterStack {
	stack := awscdk.NewStack(scope, &id, &props.StackProps)

	lambdaSG := awsec2.NewSecurityGroup(
		stack,
		jsii.String("Rate-Limiter-Lambda-Authorizer-SG"),
		&awsec2.SecurityGroupProps{
			Vpc:               props.Vpc,
			AllowAllOutbound:  jsii.Bool(false),
			SecurityGroupName: jsii.String("drl-lambda-authorizer-sg"),
		},
	)

	ecsSG := awsec2.NewSecurityGroup(
		stack,
		jsii.String("Rate-Limiter-ECS-SG"),
		&awsec2.SecurityGroupProps{
			Vpc:               props.Vpc,
			AllowAllOutbound:  jsii.Bool(false),
			SecurityGroupName: jsii.String("drl-ecs-authorizer-sg"),
		},
	)

	ecsSG.Connections().AllowFrom(
		lambdaSG,
		awsec2.Port_Tcp(jsii.Number(8080)),
		jsii.String("Allow traffic incoming from the Lambda Authorizer"),
	)

	ecsSG.AddEgressRule(
		awsec2.Peer_AnyIpv4(),
		awsec2.Port_Tcp(jsii.Number(443)),
		jsii.String("ECR/CloudWatch Access"),
		nil,
	)

	ecsSG.AddEgressRule(
		props.RedisSecurityGroup,
		awsec2.Port_Tcp(jsii.Number(6379)),
		jsii.String("Allow Outbound to Redis"),
		nil,
	)

	awsec2.NewCfnSecurityGroupIngress(
		stack,
		jsii.String("IngressFromEcsToRedis"),
		&awsec2.CfnSecurityGroupIngressProps{
			IpProtocol:            jsii.String("tcp"),
			FromPort:              jsii.Number(6379),
			ToPort:                jsii.Number(6379),
			GroupId:               props.RedisSecurityGroup.SecurityGroupId(),
			SourceSecurityGroupId: ecsSG.SecurityGroupId(),
		},
	)

	rateLimiterImage := awsecs.ContainerImage_FromAsset(
		jsii.String("../rate-limiter"),
		&awsecs.AssetImageProps{},
	)

	rateLimiterCluster := awsecs.NewCluster(
		stack,
		jsii.String("Rate-Limiter-Cluster"),
		&awsecs.ClusterProps{
			ClusterName: jsii.String("Rate-Limiter-Cluster"),
			Vpc:         props.Vpc,
			DefaultCloudMapNamespace: &awsecs.CloudMapNamespaceOptions{
				Name: jsii.String("local"),
			},
		},
	)

	rateLimitTask := awsecs.NewFargateTaskDefinition(
		stack,
		jsii.String("Rate-Limiter-Task"),
		&awsecs.FargateTaskDefinitionProps{
			Cpu:            jsii.Number(1024),
			MemoryLimitMiB: jsii.Number(2048),
			RuntimePlatform: &awsecs.RuntimePlatform{
				CpuArchitecture:       awsecs.CpuArchitecture_ARM64(),
				OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			},
		},
	)

	rateLimiterContainerEnv := map[string]*string{
		"SPRING_DATA_REDIS_HOST":                    props.RedisHost,
		"SPRING_DATA_REDIS_PORT":                    props.RedisPort,
		"MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE": props.ExposedEndpoints,
		"RATELIMITER_ALGORITHM":                     props.RateLimiterAlgorithm,
		"RATELIMITER_CAPACITY":                      props.LimiterCapacity,
		"RATELIMITER_REFILLRATE":                    props.LimiterRefillRate,
		"RATELIMITER_SECURITY_API_KEYS_ENABLED":     props.SecurityApiKeysEnabled,
		"RATELIMITER_SECURITY_IP_WHITELIST":         props.IpWhitelist,
	}

	rateLimiterContainer := rateLimitTask.AddContainer(
		jsii.String("Rate-Limiter-Container"),
		&awsecs.ContainerDefinitionOptions{
			Image: rateLimiterImage,
			Logging: awsecs.LogDrivers_AwsLogs(&awsecs.AwsLogDriverProps{
				StreamPrefix: jsii.String("rate-limiter"),
			}),
			Environment: &rateLimiterContainerEnv,
		},
	)

	awsecs.NewFargateService(
		stack,
		jsii.String("Rate-Limiter-Service"),
		&awsecs.FargateServiceProps{
			Cluster:        rateLimiterCluster,
			TaskDefinition: rateLimitTask,
			ServiceName:    jsii.String("Rate-Limiter-Service"),
			DesiredCount:   jsii.Number(1),
			SecurityGroups: &[]awsec2.ISecurityGroup{ecsSG},
			CloudMapOptions: &awsecs.CloudMapOptions{
				Name: jsii.String("rate-limiter"),
			},
			VpcSubnets: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},
		},
	)

	rateLimiterContainer.AddPortMappings(&awsecs.PortMapping{
		ContainerPort: jsii.Number(8080),
	})

	lambdaCode := awslambda.AssetCode_FromAsset(
		jsii.String("../lambda-authorizer"),
		&awss3assets.AssetOptions{
			Bundling: &awscdk.BundlingOptions{
				Image: awscdk.DockerImage_FromRegistry(jsii.String("golang:1.26")),
				User:  jsii.String("root"),
				Command: &[]*string{
					jsii.String("bash"),
					jsii.String("-c"),
					jsii.String("make build && cp bootstrap /asset-output/"),
				},
			},
		},
	)

	authLambda := awslambda.NewFunction(
		stack,
		jsii.String("Rate-Limiter-Authorizer"),
		&awslambda.FunctionProps{
			Runtime:        awslambda.Runtime_PROVIDED_AL2023(),
			Architecture:   awslambda.Architecture_ARM_64(),
			Handler:        jsii.String("bootstrap"),
			Code:           lambdaCode,
			SecurityGroups: &[]awsec2.ISecurityGroup{lambdaSG},
			Vpc:            props.Vpc,
			VpcSubnets: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},
			Environment: &map[string]*string{
				"RATE_LIMITER_URL": jsii.String("http://rate-limiter.local:8080"),
			},
		},
	)

	return &RateLimiterStack{
		Stack:            stack,
		AuthorizerLambda: authLambda,
	}
}
