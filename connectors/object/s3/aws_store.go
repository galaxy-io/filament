package s3

import (
	"context"
	"encoding/base64"
	"encoding/binary"
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

func (s *awsStore) CreateMultipart(ctx context.Context, bucket, key string, metadata objectMetadata) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), ContentType: aws.String(metadata.contentType),
		// A full-object CRC32C equals the CRC32C of the whole stream, which is
		// what the manifest publishes, so S3 can check it at completion.
		ChecksumAlgorithm: types.ChecksumAlgorithmCrc32c, ChecksumType: types.ChecksumTypeFullObject,
	}
	if metadata.contentEncoding != "" {
		input.ContentEncoding = aws.String(metadata.contentEncoding)
	}
	out, err := s.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}
	return aws.ToString(out.UploadId), nil
}

func (s *awsStore) UploadPart(ctx context.Context, bucket, key, uploadID string, number int32, body io.ReadSeeker, size int64, crc32c uint32) (string, error) {
	out, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(number), Body: body, ContentLength: aws.Int64(size),
		ChecksumCRC32C: crc32cHeader(crc32c),
	})
	if err != nil {
		return "", err
	}
	if err := checkEcho("part", crc32c, out.ChecksumCRC32C); err != nil {
		return "", err
	}
	return aws.ToString(out.ETag), nil
}

func (s *awsStore) CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []completedPart, crc32c uint32) error {
	awsParts := make([]types.CompletedPart, len(parts))
	for i, part := range parts {
		awsParts[i] = types.CompletedPart{
			PartNumber: aws.Int32(part.number), ETag: aws.String(part.token), ChecksumCRC32C: crc32cHeader(part.crc32c),
		}
	}
	out, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: awsParts},
		ChecksumType:    types.ChecksumTypeFullObject, ChecksumCRC32C: crc32cHeader(crc32c),
	})
	if err != nil {
		return err
	}
	return checkEcho("object", crc32c, out.ChecksumCRC32C)
}

func (s *awsStore) AbortMultipart(ctx context.Context, bucket, key, uploadID string) error {
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
	})
	return err
}

func (s *awsStore) PutObject(ctx context.Context, bucket, key string, metadata objectMetadata, body io.ReadSeeker, size int64, crc32c uint32) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(key), ContentType: aws.String(metadata.contentType),
		Body: body, ContentLength: aws.Int64(size), ChecksumCRC32C: crc32cHeader(crc32c),
	}
	if metadata.contentEncoding != "" {
		input.ContentEncoding = aws.String(metadata.contentEncoding)
	}
	out, err := s.client.PutObject(ctx, input)
	if err != nil {
		return err
	}
	return checkEcho("object", crc32c, out.ChecksumCRC32C)
}

// crc32cHeader encodes a checksum the way S3 expects it: base64 of the
// big-endian bytes.
func crc32cHeader(crc32c uint32) *string {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], crc32c)
	return aws.String(base64.StdEncoding.EncodeToString(raw[:]))
}

// checkEcho compares the checksum S3 reports for what it stored with the one
// sent. S3 already rejects a mismatched write, so this guards against an
// S3-compatible store that accepts the header without enforcing it. A store
// that echoes nothing is not checked.
func checkEcho(what string, sent uint32, echoed *string) error {
	if echoed == nil || *echoed == "" {
		return nil
	}
	if want := aws.ToString(crc32cHeader(sent)); *echoed != want {
		return fmt.Errorf("%s CRC32C mismatch: sent %s, store reports %s", what, want, *echoed)
	}
	return nil
}
