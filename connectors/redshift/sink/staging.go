package redshift

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type s3API interface {
	HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type stagingStore struct {
	client s3API
	bucket string
	prefix string
}

type stagedBatch struct {
	dataKeys     []string
	manifestKeys []string
	manifestURIs []string
}

const maxManifestFiles = 128

type copyManifest struct {
	Entries []copyManifestEntry `json:"entries"`
}

type copyManifestEntry struct {
	URL       string           `json:"url"`
	Mandatory bool             `json:"mandatory"`
	Meta      copyManifestMeta `json:"meta"`
}

type copyManifestMeta struct {
	ContentLength int64 `json:"content_length"`
}

func newStagingStore(ctx context.Context, bucket, region, prefix string) (*stagingStore, error) {
	var opts []func(*awscfg.LoadOptions) error
	if region != "" {
		opts = append(opts, awscfg.WithRegion(region))
	}
	cfg, err := awscfg.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS configuration: %w", err)
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("AWS region is required (set staging_bucket_region or configure the AWS SDK default chain)")
	}
	return &stagingStore{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		prefix: strings.Trim(prefix, "/"),
	}, nil
}

func pipelineStagingPrefix(prefix, pipelineID string) (string, error) {
	pipelineID = strings.TrimSpace(pipelineID)
	if pipelineID == "" {
		return "", fmt.Errorf("pipeline ID is required")
	}
	if strings.ContainsAny(pipelineID, "/\\\r\n") {
		return "", fmt.Errorf("pipeline ID must be a single S3 key segment")
	}
	return path.Join(strings.Trim(prefix, "/"), pipelineID), nil
}

func (s *stagingStore) test(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return fmt.Errorf("access bucket %q: %w", s.bucket, err)
	}
	return nil
}

func (s *stagingStore) put(ctx context.Context, objectPrefix string, payloads [][]byte, checksums []uint32) (stagedBatch, error) {
	if len(payloads) == 0 || len(payloads) != len(checksums) {
		return stagedBatch{}, fmt.Errorf("stage payload and checksum counts must match and be nonzero")
	}
	staged := stagedBatch{
		dataKeys:     make([]string, 0, len(payloads)),
		manifestKeys: make([]string, 0, (len(payloads)+maxManifestFiles-1)/maxManifestFiles),
		manifestURIs: make([]string, 0, (len(payloads)+maxManifestFiles-1)/maxManifestFiles),
	}
	entries := make([]copyManifestEntry, 0, len(payloads))
	for i, payload := range payloads {
		filename := fmt.Sprintf("data-%05d.parquet", i)
		dataKey := path.Join(s.prefix, objectPrefix, filename)
		if err := s.putObject(ctx, dataKey, "application/vnd.apache.parquet", payload, checksums[i]); err != nil {
			s.cleanup(context.WithoutCancel(ctx), staged)
			return stagedBatch{}, fmt.Errorf("upload parquet part %d: %w", i, err)
		}
		staged.dataKeys = append(staged.dataKeys, dataKey)
		entries = append(entries, copyManifestEntry{
			URL: s.uri(dataKey), Mandatory: true, Meta: copyManifestMeta{ContentLength: int64(len(payload))},
		})
	}
	for start, manifestPart := 0, 0; start < len(entries); start, manifestPart = start+maxManifestFiles, manifestPart+1 {
		end := min(start+maxManifestFiles, len(entries))
		manifest, err := json.Marshal(copyManifest{Entries: entries[start:end]})
		if err != nil {
			s.cleanup(context.WithoutCancel(ctx), staged)
			return stagedBatch{}, fmt.Errorf("encode COPY manifest %d: %w", manifestPart, err)
		}
		manifestKey := path.Join(s.prefix, objectPrefix, fmt.Sprintf("manifest-%05d.json", manifestPart))
		staged.manifestKeys = append(staged.manifestKeys, manifestKey)
		manifestCRC := crc32.Checksum(manifest, encodedCRCTable)
		if err := s.putObject(ctx, manifestKey, "application/json", manifest, manifestCRC); err != nil {
			s.cleanup(context.WithoutCancel(ctx), staged)
			return stagedBatch{}, fmt.Errorf("upload COPY manifest %d: %w", manifestPart, err)
		}
		staged.manifestURIs = append(staged.manifestURIs, s.uri(manifestKey))
	}
	return staged, nil
}

func (s *stagingStore) putObject(ctx context.Context, key, contentType string, payload []byte, crc uint32) error {
	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(s.bucket),
		Key:               aws.String(key),
		Body:              bytes.NewReader(payload),
		ContentLength:     aws.Int64(int64(len(payload))),
		ContentType:       aws.String(contentType),
		ChecksumAlgorithm: types.ChecksumAlgorithmCrc32c,
		ChecksumCRC32C:    aws.String(base64.StdEncoding.EncodeToString(checksum)),
	})
	return err
}

func (s *stagingStore) cleanup(ctx context.Context, staged stagedBatch) {
	for _, key := range staged.manifestKeys {
		_ = s.delete(ctx, key)
	}
	for _, key := range staged.dataKeys {
		_ = s.delete(ctx, key)
	}
}

func (s *stagingStore) delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	return err
}

func (s *stagingStore) uri(key string) string {
	return "s3://" + s.bucket + "/" + key
}
