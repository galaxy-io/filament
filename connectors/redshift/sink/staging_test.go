package redshift

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type fakeS3 struct {
	objects map[string][]byte
}

func (f *fakeS3) HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return &s3.HeadBucketOutput{}, nil
}

func (f *fakeS3) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	payload, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	f.objects[aws.ToString(input.Key)] = payload
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(f.objects, aws.ToString(input.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func TestStagingStoreSplitsManifestsAfter128Files(t *testing.T) {
	client := &fakeS3{objects: make(map[string][]byte)}
	store := &stagingStore{client: client, bucket: "company-loads", prefix: "filament/staging/pipeline-123"}
	payloads := make([][]byte, maxManifestFiles+1)
	checksums := make([]uint32, len(payloads))
	for i := range payloads {
		payloads[i] = []byte{byte(i)}
	}
	staged, err := store.put(t.Context(), "orders/run-1/0/1", payloads, checksums)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.manifestKeys) != 2 || len(staged.manifestURIs) != 2 {
		t.Fatalf("staged manifests = keys %#v URIs %#v", staged.manifestKeys, staged.manifestURIs)
	}
	wantEntries := []int{maxManifestFiles, 1}
	for i, key := range staged.manifestKeys {
		var manifest copyManifest
		if err := json.Unmarshal(client.objects[key], &manifest); err != nil {
			t.Fatal(err)
		}
		if len(manifest.Entries) != wantEntries[i] {
			t.Fatalf("manifest %d entries = %d, want %d", i, len(manifest.Entries), wantEntries[i])
		}
	}
}
