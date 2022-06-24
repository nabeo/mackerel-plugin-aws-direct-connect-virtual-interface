package mpawsdxvif

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	mp "github.com/mackerelio/go-mackerel-plugin"
)

// AwsDxVifPlugin struct
type AwsDxVifPlugin struct {
	Prefix      string
	AccessKeyID string
	SecretKeyID string
	Region      string
	RoleArn     string
	DxVif       string
	DxConId     string
	CloudWatch  *cloudwatch.Client
}

const (
	namespace = "AWS/DX"
)

type metrics struct {
	Name string
	Type types.Statistic
}

// GraphDefinition : return graph definition
func (p AwsDxVifPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := strings.Title(p.Prefix)
	labelPrefix = strings.Replace(labelPrefix, "-", " ", -1)

	// https://docs.aws.amazon.com/directconnect/latest/UserGuide/monitoring-cloudwatch.html#viewing-metrics
	return map[string]mp.Graphs{
		"Bps": {
			Label: labelPrefix + " bps",
			Unit:  mp.UnitBitsPerSecond,
			Metrics: []mp.Metrics{
				// The bitrate for outbound data from the AWS side of the virtual interface.
				{Name: "VirtualInterfaceBpsEgress", Label: "bps out"},

				// The bitrate for inbound data to the AWS side of the virtual interface.
				{Name: "VirtualInterfaceBpsIngress", Label: "bps in"},
			},
		},

		"Pps": {
			Label: labelPrefix + " pps",
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				// The packet rate for outbound data from the AWS side of the virtual interface.
				{Name: "VirtualInterfacePpsEgress", Label: "pps out"},

				// The packet rate for inbound data to the AWS side of the virtual interface.
				{Name: "VirtualInterfacePpsIngress", Label: "pps in"},
			},
		},
	}
}

// MetricKeyPrefix : interface for PluginWithPrefix
func (p AwsDxVifPlugin) MetricKeyPrefix() string {
	if p.Prefix == "" {
		p.Prefix = "DxVif"
	}
	return p.Prefix
}

// FetchMetrics : fetch metric
func (p AwsDxVifPlugin) FetchMetrics() (map[string]float64, error) {
	stat := make(map[string]float64)
	for _, met := range []metrics{
		{Name: "VirtualInterfaceBpsEgress", Type: types.StatisticAverage},
		{Name: "VirtualInterfaceBpsIngress", Type: types.StatisticAverage},
		{Name: "VirtualInterfacePpsEgress", Type: types.StatisticAverage},
		{Name: "VirtualInterfacePpsIngress", Type: types.StatisticAverage},
	} {
		v, err := p.getLastPoint(met)
		if err != nil {
			log.Printf("%s : %s", met, err)
		}
		stat[met.Name] = v
	}

	return stat, nil
}

// getLastPoint ...
func (p AwsDxVifPlugin) getLastPoint(metric metrics) (float64, error) {
	now := time.Now()

	// https://docs.aws.amazon.com/directconnect/latest/UserGuide/monitoring-cloudwatch.html#metrics-dimensions
	dimensions := []types.Dimension{
		{
			Name:  aws.String("ConnectionId"),
			Value: aws.String(p.DxConId),
		},
		{
			Name:  aws.String("VirtualInterfaceId"),
			Value: aws.String(p.DxVif),
		},
	}

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		Dimensions: dimensions,
		StartTime:  aws.Time(now.Add(time.Duration(180) * time.Second * -1)), // 3 min (to fetch at least 1 data-point)
		EndTime:    aws.Time(now),
		Period:     aws.Int32(60),
		MetricName: aws.String(metric.Name),
		Statistics: []types.Statistic{metric.Type},
	}

	response, err := p.CloudWatch.GetMetricStatistics(context.Background(), input)
	if err != nil {
		return 0, err
	}

	datapoints := response.Datapoints
	if len(datapoints) == 0 {
		return 0, errors.New("fetch no datapoints : " + p.DxVif)
	}

	// get least recently datapoint.
	// because a most recently datapoint is not stable.
	least := time.Now()
	var latestVal float64
	for _, dp := range datapoints {
		if dp.Timestamp.Before(least) {
			least = *dp.Timestamp
			if metric.Type == types.StatisticAverage {
				latestVal = *dp.Average
			}
		}
	}

	return latestVal, nil
}

func (p *AwsDxVifPlugin) prepare() error {
	var opts []func(*config.LoadOptions) error

	if p.RoleArn != "" {
		cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
		if err != nil {
			return err
		}
		stsclient := sts.NewFromConfig(cfg)

		appCreds := stscreds.NewAssumeRoleProvider(stsclient, p.RoleArn)
		opts = append(opts, config.WithCredentialsProvider(appCreds))
	} else if p.AccessKeyID != "" && p.SecretKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.AccessKeyID, p.SecretKeyID, "")))
	}

	if p.Region != "" {
		opts = append(opts, config.WithRegion(p.Region))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return err
	}

	p.CloudWatch = cloudwatch.NewFromConfig(cfg)
	return nil
}

// Do: Do plugin
func Do() {
	optPrefix := flag.String("metric-key-prefix", "", "Metric Key Prefix")
	optAccessKeyID := flag.String("access-key-id", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS Access Key ID")
	optSecretKeyID := flag.String("secret-key-id", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS Secret Access Key ID")
	optRegion := flag.String("region", os.Getenv("AWS_DEFAULT_REGION"), "AWS Region")
	optRoleArn := flag.String("role-arn", "", "IAM Role ARN for assume role")
	optDxVif := flag.String("virtual-interface-id", "", "Resource ID of Direct Connect Virtual Interface")
	optDxCon := flag.String("direct-connect-connection", "", "Resource ID of Direct Connect")
	flag.Parse()

	var AwsDxVifPlugin AwsDxVifPlugin

	AwsDxVifPlugin.Prefix = *optPrefix
	AwsDxVifPlugin.AccessKeyID = *optAccessKeyID
	AwsDxVifPlugin.SecretKeyID = *optSecretKeyID
	AwsDxVifPlugin.Region = *optRegion
	AwsDxVifPlugin.RoleArn = *optRoleArn
	AwsDxVifPlugin.DxVif = *optDxVif
	AwsDxVifPlugin.DxConId = *optDxCon

	err := AwsDxVifPlugin.prepare()
	if err != nil {
		log.Fatalln(err)
	}

	helper := mp.NewMackerelPlugin(AwsDxVifPlugin)
	helper.Run()
}
