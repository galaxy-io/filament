package sink

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
)

const (
	authMethodInstanceProfile = "instance_profile"
	authMethodIAMCredentials  = "iam_credentials"

	// A stream accepts 1 MiB records unless its limit was raised, up to 10 MiB.
	defaultMaxRecordBytes = 1 << 20
	maxRecordBytes        = 10 << 20
)

var streamName = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,128}$`)

type sinkConfig struct {
	region, endpoint, authMethod               string
	accessKeyID, secretAccessKey, sessionToken string
	stream, prefix, keyField                   string
	timeout                                    time.Duration
	maxRecordBytes                             int64
}

// Validate checks connection settings without contacting AWS.
func (*Sink) Validate(cfg filament.Config) error {
	_, err := connection(cfg)
	return err
}

func connection(cfg filament.Config) (sinkConfig, error) {
	c := sinkConfig{
		region:          strings.TrimSpace(cfg.String("region")),
		endpoint:        strings.TrimSpace(cfg.String("endpoint")),
		authMethod:      strings.TrimSpace(cfg.String("auth_method")),
		accessKeyID:     cfg.Secret("access_key_id"),
		secretAccessKey: cfg.Secret("secret_access_key"),
		sessionToken:    cfg.Secret("session_token"),
	}
	if c.authMethod == "" {
		c.authMethod = authMethodInstanceProfile
	}
	credentialsPresent := c.accessKeyID != "" || c.secretAccessKey != "" || c.sessionToken != ""
	switch c.authMethod {
	case authMethodInstanceProfile:
		if credentialsPresent {
			return c, fmt.Errorf("kinesis sink: IAM credential fields require auth_method %q", authMethodIAMCredentials)
		}
	case authMethodIAMCredentials:
		if c.accessKeyID == "" || c.secretAccessKey == "" {
			return c, fmt.Errorf("kinesis sink: access_key_id and secret_access_key are required for auth_method %q", authMethodIAMCredentials)
		}
	default:
		return c, fmt.Errorf("kinesis sink: unsupported auth_method %q", c.authMethod)
	}
	return c, nil
}

func resolve(cfg filament.Config) (sinkConfig, error) {
	c, err := connection(cfg)
	if err != nil {
		return c, err
	}
	c.stream, c.prefix, c.keyField = cfg.String("stream"), cfg.String("stream_prefix"), cfg.String("key_field")
	c.timeout, c.maxRecordBytes = 30*time.Second, defaultMaxRecordBytes
	if (c.stream == "") == (c.prefix == "") {
		return c, errors.New("kinesis sink: configure exactly one of stream and stream_prefix")
	}
	target := c.stream
	if target == "" {
		target = c.prefix
	}
	if err := validateStream(target); err != nil {
		return c, err
	}
	if cfg.Has("publish_timeout") {
		d, err := time.ParseDuration(cfg.String("publish_timeout"))
		if err != nil || d < time.Second {
			return c, errors.New("kinesis sink: publish_timeout must be at least 1s")
		}
		c.timeout = d
	}
	if cfg.Has("max_record_bytes") {
		n := int64(cfg.Int("max_record_bytes"))
		if n < 512 || n > maxRecordBytes {
			return c, errors.New("kinesis sink: max_record_bytes must be between 512 bytes and 10 MiB")
		}
		c.maxRecordBytes = n
	}
	return c, nil
}

func validateStream(name string) error {
	if !streamName.MatchString(name) {
		return fmt.Errorf("kinesis sink: invalid stream name %q", name)
	}
	return nil
}

func (c sinkConfig) route(resource string) (string, error) {
	s := c.stream
	if s == "" {
		s = c.prefix + "." + resource
	}
	return s, validateStream(s)
}
