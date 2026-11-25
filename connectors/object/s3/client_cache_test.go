package s3

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
)

// TestClientCacheKeysOnSettings opens a store twice with the same settings,
// as a continuous run does at each boundary, and checks that the second store
// shares the first one's client, while a rotated secret gets a new one.
func TestClientCacheKeysOnSettings(t *testing.T) {
	config := func(secret string) sinkConfig {
		cfg, err := parseConfig(filament.NewConfig(map[string]any{
			"bucket": "bucket", "region": "us-east-1", "endpoint": "http://127.0.0.1:1",
			"auth_method": "iam_credentials", "access_key_id": "key", "secret_access_key": secret,
		}))
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	ctx := context.Background()
	stores := make([]*awsStore, 0, 3)
	for _, secret := range []string{"secret", "secret", "rotated"} {
		store, err := newAWSStore(ctx, config(secret))
		if err != nil {
			t.Fatal(err)
		}
		stores = append(stores, store)
	}
	if stores[0].client != stores[1].client {
		t.Fatal("same settings built a second client")
	}
	if stores[2].client == stores[0].client {
		t.Fatal("a rotated secret reused the old client")
	}
}
