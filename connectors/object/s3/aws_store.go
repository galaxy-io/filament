package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type awsStore struct{ client *s3.Client }

var _ multipartStore = (*awsStore)(nil)

func newAWSStore(ctx context.Context, cfg sinkConfig) (*awsStore, error) {
	var loadOpts []func(*awscfg.LoadOptions) error
	if cfg.region != "" {
		loadOpts = append(loadOpts, awscfg.WithRegion(cfg.region))
	}
	if cfg.authMethod == authMethodIAMCredentials {
		loadOpts = append(loadOpts, awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.accessKeyID, cfg.secretAccessKey, cfg.sessionToken),
		))
	}
	awsCfg, err := awscfg.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3 sink: load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if cfg.endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.endpoint)
		}
		options.UsePathStyle = cfg.pathStyle
	})
	return &awsStore{client: client}, nil
}

func (s *awsStore) HeadBucket(ctx context.Context, bucket string) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	return err
}

func (s *awsStore) CreateMultipart(ctx context.Context, bucket, key, contentType string) (string, error) {
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.UploadId), nil
}

func (s *awsStore) UploadPart(ctx context.Context, bucket, key, uploadID string, number int32, body io.ReadSeeker, size int64) (string, error) {
	out, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(number), Body: body, ContentLength: aws.Int64(size),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.ETag), nil
}

func (s *awsStore) CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []completedPart) error {
	awsParts := make([]types.CompletedPart, len(parts))
	for i, part := range parts {
		awsParts[i] = types.CompletedPart{PartNumber: aws.Int32(part.number), ETag: aws.String(part.token)}
	}
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: awsParts},
	})
	return err
}

func (s *awsStore) AbortMultipart(ctx context.Context, bucket, key, uploadID string) error {
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
	})
	return err
}

func (s *awsStore) PutObject(ctx context.Context, bucket, key, contentType string, body io.ReadSeeker, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(key), ContentType: aws.String(contentType),
		Body: body, ContentLength: aws.Int64(size),
	})
	return err
}
