package sink

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
)

type client interface {
	PutRecords(context.Context, *kinesis.PutRecordsInput, ...func(*kinesis.Options)) (*kinesis.PutRecordsOutput, error)
	DescribeStreamSummary(context.Context, *kinesis.DescribeStreamSummaryInput, ...func(*kinesis.Options)) (*kinesis.DescribeStreamSummaryOutput, error)
}

func newClient(ctx context.Context, c sinkConfig) (*kinesis.Client, error) {
	var opts []func(*config.LoadOptions) error
	if c.region != "" {
		opts = append(opts, config.WithRegion(c.region))
	}
	if c.authMethod == authMethodIAMCredentials {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.accessKeyID, c.secretAccessKey, c.sessionToken),
		))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("kinesis sink: load aws config: %w", err)
	}
	return kinesis.NewFromConfig(cfg, func(o *kinesis.Options) {
		if c.endpoint != "" {
			o.BaseEndpoint = aws.String(c.endpoint)
		}
	}), nil
}

// writable reports whether the stream exists and accepts records.
func writable(ctx context.Context, cl client, stream string) error {
	out, err := cl.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(stream)})
	if err != nil {
		return fmt.Errorf("kinesis sink: describe stream %q: %w", stream, err)
	}
	status := out.StreamDescriptionSummary.StreamStatus
	if status != types.StreamStatusActive && status != types.StreamStatusUpdating {
		return fmt.Errorf("kinesis sink: stream %q is %s", stream, status)
	}
	return nil
}
