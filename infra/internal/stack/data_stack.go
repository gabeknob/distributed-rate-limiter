package stack

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticache"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type DataStackProps struct {
	awscdk.StackProps
	Vpc awsec2.IVpc
}

type DataStack struct {
	Stack               awscdk.Stack
	RedisEndpointAdress *string
	RedisEndpointPort   *string
	RedisSecurityGroup  awsec2.ISecurityGroup
}

func NewDataStack(scope constructs.Construct, id string, props *DataStackProps) *DataStack {
	stack := awscdk.NewStack(scope, &id, &props.StackProps)

	redisSG := awsec2.NewSecurityGroup(stack, jsii.String("Redis-SG"), &awsec2.SecurityGroupProps{
		Vpc: props.Vpc,
	})

	subnets := props.Vpc.SelectSubnets(&awsec2.SubnetSelection{SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS}).SubnetIds
	subnetIds := make([]interface{}, 0)
	for _, subnetId := range *subnets {
		subnetIds = append(subnetIds, subnetId)
	}

	subnetGroup := awselasticache.NewCfnSubnetGroup(
		stack,
		jsii.String("Redis-Subnet-Group"),
		&awselasticache.CfnSubnetGroupProps{
			Description: jsii.String("Subnets privadas para cluster Redis/Valkey"),
			SubnetIds:   &subnetIds,
		},
	)

	cache := awselasticache.NewCfnCacheCluster(
		stack,
		jsii.String("Redis-Cache"),
		&awselasticache.CfnCacheClusterProps{
			Engine:               jsii.String("valkey"),
			CacheNodeType:        jsii.String("cache.t3.micro"),
			NumCacheNodes:        jsii.Number(1),
			CacheSubnetGroupName: subnetGroup.Ref(),
			VpcSecurityGroupIds:  &[]*string{redisSG.SecurityGroupId()},
		},
	)

	return &DataStack{
		Stack:               stack,
		RedisEndpointAdress: cache.AttrRedisEndpointAddress(),
		RedisEndpointPort:   cache.AttrRedisEndpointPort(),
		RedisSecurityGroup:  redisSG,
	}
}
