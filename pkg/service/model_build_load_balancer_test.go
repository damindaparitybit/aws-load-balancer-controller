package service

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mock_networking "sigs.k8s.io/aws-load-balancer-controller/mocks/networking"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/annotations"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/model/elbv2"
	"testing"
)

func Test_defaultModelBuilderTask_buildLBAttributes(t *testing.T) {
	tests := []struct {
		testName  string
		svc       *corev1.Service
		wantError bool
		wantValue []elbv2.LoadBalancerAttribute
	}{
		{
			testName: "Default values",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-type": "nlb-ip",
					},
				},
			},
			wantError: false,
			wantValue: []elbv2.LoadBalancerAttribute{
				{
					Key:   lbAttrsAccessLogsS3Enabled,
					Value: "false",
				},
				{
					Key:   lbAttrsAccessLogsS3Bucket,
					Value: "",
				},
				{
					Key:   lbAttrsAccessLogsS3Prefix,
					Value: "",
				},
				{
					Key:   lbAttrsLoadBalancingCrossZoneEnabled,
					Value: "false",
				},
			},
		},
		{
			testName: "Annotation specified",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-type":                              "nlb-ip",
						"service.beta.kubernetes.io/aws-load-balancer-access-log-enabled":                "true",
						"service.beta.kubernetes.io/aws-load-balancer-access-log-s3-bucket-name":         "nlb-bucket",
						"service.beta.kubernetes.io/aws-load-balancer-access-log-s3-bucket-prefix":       "bkt-pfx",
						"service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled": "true",
					},
				},
			},
			wantError: false,
			wantValue: []elbv2.LoadBalancerAttribute{
				{
					Key:   lbAttrsAccessLogsS3Enabled,
					Value: "true",
				},
				{
					Key:   lbAttrsAccessLogsS3Bucket,
					Value: "nlb-bucket",
				},
				{
					Key:   lbAttrsAccessLogsS3Prefix,
					Value: "bkt-pfx",
				},
				{
					Key:   lbAttrsLoadBalancingCrossZoneEnabled,
					Value: "true",
				},
			},
		},
		{
			testName: "Annotation invalid",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-type":                              "nlb-ip",
						"service.beta.kubernetes.io/aws-load-balancer-access-log-enabled":                "FalSe",
						"service.beta.kubernetes.io/aws-load-balancer-access-log-s3-bucket-name":         "nlb-bucket",
						"service.beta.kubernetes.io/aws-load-balancer-access-log-s3-bucket-prefix":       "bkt-pfx",
						"service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled": "true",
					},
				},
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			parser := annotations.NewSuffixAnnotationParser("service.beta.kubernetes.io")
			builder := &defaultModelBuildTask{
				service:                              tt.svc,
				annotationParser:                     parser,
				defaultAccessLogsS3Bucket:            "",
				defaultAccessLogsS3Prefix:            "",
				defaultLoadBalancingCrossZoneEnabled: false,
				defaultProxyProtocolV2Enabled:        false,
				defaultHealthCheckProtocol:           elbv2.ProtocolTCP,
				defaultHealthCheckPort:               healthCheckPortTrafficPort,
				defaultHealthCheckPath:               "/",
				defaultHealthCheckInterval:           10,
				defaultHealthCheckTimeout:            10,
				defaultHealthCheckHealthyThreshold:   3,
				defaultHealthCheckUnhealthyThreshold: 3,
			}
			lbAttributes, err := builder.buildLoadBalancerAttributes(context.Background())
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tt.wantValue, lbAttributes)
			}
		})
	}
}

func Test_defaultModelBuilderTask_buildSubnetMappings(t *testing.T) {
	tests := []struct {
		name    string
		subnets []*ec2.Subnet
		want    []elbv2.SubnetMapping
		svc     *corev1.Service
		wantErr error
	}{
		{
			name: "Multiple subnets",
			subnets: []*ec2.Subnet{
				{
					SubnetId:         aws.String("subnet-1"),
					AvailabilityZone: aws.String("us-west-2a"),
					VpcId:            aws.String("vpc-1"),
				},
				{
					SubnetId:         aws.String("subnet-2"),
					AvailabilityZone: aws.String("us-west-2b"),
					VpcId:            aws.String("vpc-1"),
				},
			},
			svc: &corev1.Service{},
			want: []elbv2.SubnetMapping{
				{
					SubnetID: "subnet-1",
				},
				{
					SubnetID: "subnet-2",
				},
			},
		},
		{
			name: "When EIP allocation is configured",
			subnets: []*ec2.Subnet{
				{
					SubnetId:         aws.String("subnet-1"),
					AvailabilityZone: aws.String("us-west-2a"),
					VpcId:            aws.String("vpc-1"),
				},
				{
					SubnetId:         aws.String("subnet-2"),
					AvailabilityZone: aws.String("us-west-2b"),
					VpcId:            aws.String("vpc-1"),
				},
			},
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-eip-allocations": "eip1, eip2",
					},
				},
			},
			want: []elbv2.SubnetMapping{
				{
					SubnetID:     "subnet-1",
					AllocationID: aws.String("eip1"),
				},
				{
					SubnetID:     "subnet-2",
					AllocationID: aws.String("eip2"),
				},
			},
		},
		{
			name: "When EIP allocation and subnet mismatch",
			subnets: []*ec2.Subnet{
				{
					SubnetId:         aws.String("subnet-1"),
					AvailabilityZone: aws.String("us-west-2a"),
					VpcId:            aws.String("vpc-1"),
				},
				{
					SubnetId:         aws.String("subnet-2"),
					AvailabilityZone: aws.String("us-west-2b"),
					VpcId:            aws.String("vpc-1"),
				},
			},
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-eip-allocations": "eip1",
					},
				},
			},
			wantErr: errors.New("number of EIP allocations (1) and subnets (2) must match"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			annotationParser := annotations.NewSuffixAnnotationParser("service.beta.kubernetes.io")
			builder := &defaultModelBuildTask{service: tt.svc, annotationParser: annotationParser}
			got, err := builder.buildLoadBalancerSubnetMappings(context.Background(), tt.subnets)
			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_defaultModelBuilderTask_resolveLoadBalancerSubnets(t *testing.T) {
	type resolveSubnetResults struct {
		subnets []*ec2.Subnet
		err     error
	}
	tests := []struct {
		name                     string
		svc                      *corev1.Service
		scheme                   elbv2.LoadBalancerScheme
		resolveViaDiscovery      []resolveSubnetResults
		resolveViaNameOrIDSlilce []resolveSubnetResults
	}{
		{
			name:   "subnet auto-discovery",
			svc:    &corev1.Service{},
			scheme: elbv2.LoadBalancerSchemeInternal,
			resolveViaDiscovery: []resolveSubnetResults{
				{
					subnets: []*ec2.Subnet{
						{
							SubnetId:  aws.String("subnet-1"),
							CidrBlock: aws.String("192.168.0.0/19"),
						},
					},
				},
			},
		},
		{
			name: "subnet annotation",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-subnets": "subnet-abc, Subnet Name XYZ",
					},
				},
			},
			scheme: elbv2.LoadBalancerSchemeInternal,
			resolveViaNameOrIDSlilce: []resolveSubnetResults{
				{
					subnets: []*ec2.Subnet{
						{
							SubnetId:  aws.String("subnet-abc"),
							CidrBlock: aws.String("192.168.0.0/19"),
						},
						{
							SubnetId:  aws.String("subnet-xyz"),
							CidrBlock: aws.String("192.168.0.0/19"),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			subnetsResolver := mock_networking.NewMockSubnetsResolver(ctrl)
			for _, call := range tt.resolveViaDiscovery {
				subnetsResolver.EXPECT().ResolveViaDiscovery(gomock.Any(), gomock.Any()).Return(call.subnets, call.err)
			}
			for _, call := range tt.resolveViaNameOrIDSlilce {
				subnetsResolver.EXPECT().ResolveViaNameOrIDSlice(gomock.Any(), gomock.Any(), gomock.Any()).Return(call.subnets, call.err)
			}
			annotationParser := annotations.NewSuffixAnnotationParser("service.beta.kubernetes.io")
			builder := &defaultModelBuildTask{service: tt.svc, annotationParser: annotationParser, subnetsResolver: subnetsResolver}

			builder.resolveLoadBalancerSubnets(context.Background(), tt.scheme)
		})
	}
}
