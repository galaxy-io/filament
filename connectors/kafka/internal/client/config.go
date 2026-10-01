// Package client supplies shared Kafka connection configuration.
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/galaxy-io/filament"
)

// Shared catalog branding for Kafka sources and sinks.
const (
	DisplayName  = "Kafka"
	DarkLogoURL  = "https://cdn.getgalaxy.io/sources/source-icon-kafka-dark.svg"
	LightLogoURL = "https://cdn.getgalaxy.io/sources/source-icon-kafka-light.svg"
)

// Fields describes shared connection settings for the connector catalog.
func Fields() []filament.ConfigField {
	sasl := &filament.FieldCondition{Field: "sasl_mechanism", Values: []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"}}
	return []filament.ConfigField{
		{Name: "brokers", Type: filament.FieldList, Required: true, Scope: filament.ScopeConnection, Help: "Kafka bootstrap host:port addresses"},
		{Name: "client_id", Type: filament.FieldString, Default: "filament", Scope: filament.ScopeConnection},
		{Name: "tls_enabled", Type: filament.FieldBool, Scope: filament.ScopeConnection, Help: "Enable TLS with system-trusted certificates; no files required"},
		{Name: "tls_ca_pem", Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "Optional PEM CA certificates for a private CA; paste the full contents"},
		{Name: "tls_cert_pem", Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "Optional PEM client certificate chain for mutual TLS; supply with its private key"},
		{Name: "tls_key_pem", Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "PEM private key matching the client certificate; stored as a secret"},
		{Name: "sasl_mechanism", Type: filament.FieldEnum, Default: "none", Scope: filament.ScopeConnection, Help: "Authentication method; select None for the local test broker or certificate-only authentication", Enum: []filament.EnumOption{{Value: "none", Label: "None"}, {Value: "PLAIN", Label: "Username / Password (PLAIN)"}, {Value: "SCRAM-SHA-256", Label: "Username / Password (SCRAM-SHA-256)"}, {Value: "SCRAM-SHA-512", Label: "Username / Password (SCRAM-SHA-512)"}}},
		{Name: "sasl_username", Type: filament.FieldString, Required: true, VisibleWhen: sasl, Scope: filament.ScopeConnection},
		{Name: "sasl_password", Type: filament.FieldSecret, Required: true, VisibleWhen: sasl, Scope: filament.ScopeConnection},
	}
}

// Strings decodes a list of nonempty configuration strings.
func Strings(cfg filament.Config, name string) ([]string, error) {
	raw, err := json.Marshal(cfg.Raw()[name])
	if err != nil {
		return nil, err
	}
	var values []string
	if err = json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("kafka: %s must be a list of strings", name)
	}
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("kafka: empty %s entry", name)
		}
	}
	return values, nil
}

// Validate checks configuration without contacting the broker.
func Validate(cfg filament.Config) error {
	brokers, err := Strings(cfg, "brokers")
	if err != nil {
		return err
	}
	if len(brokers) == 0 {
		return fmt.Errorf("kafka: brokers is required")
	}
	if cfg.Has("tls_enabled") {
		if _, ok := cfg.Raw()["tls_enabled"].(bool); !ok {
			return fmt.Errorf("kafka: tls_enabled must be boolean")
		}
	}
	for _, field := range []string{"tls_ca_file", "tls_cert_file", "tls_key_file"} {
		if cfg.Has(field) {
			return fmt.Errorf("kafka: %s is no longer supported; paste the certificate/key contents into the matching *_pem field", field)
		}
	}
	cert, key := cfg.Secret("tls_cert_pem"), cfg.Secret("tls_key_pem")
	if (cert == "") != (key == "") {
		return fmt.Errorf("kafka: TLS certificate and key must be paired")
	}
	if !cfg.Bool("tls_enabled") && (cert != "" || cfg.Secret("tls_ca_pem") != "") {
		return fmt.Errorf("kafka: TLS PEM credentials require tls_enabled")
	}
	mechanism := cfg.String("sasl_mechanism")
	switch mechanism {
	case "", "none", "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
	default:
		return fmt.Errorf("kafka: unsupported SASL mechanism")
	}
	user, pass := cfg.String("sasl_username"), cfg.Secret("sasl_password")
	if mechanism == "" || mechanism == "none" {
		if user != "" || pass != "" {
			return fmt.Errorf("kafka: SASL credentials require a mechanism")
		}
	} else if user == "" || pass == "" {
		return fmt.Errorf("kafka: SASL username and password required")
	}
	_, err = tlsConfig(cfg)
	return err
}

// Options builds authenticated Kafka client options with certificate verification.
func Options(cfg filament.Config) ([]kgo.Opt, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	brokers, _ := Strings(cfg, "brokers")
	id := cfg.String("client_id")
	if id == "" {
		id = "filament"
	}
	opts := []kgo.Opt{kgo.SeedBrokers(brokers...), kgo.ClientID(id), kgo.DialTimeout(5 * time.Second)}
	if cfg.Bool("tls_enabled") {
		t, err := tlsConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.DialTLSConfig(t))
	}

	user, pass := cfg.String("sasl_username"), cfg.Secret("sasl_password")
	switch cfg.String("sasl_mechanism") {
	case "PLAIN":
		opts = append(opts, kgo.SASL(plain.Auth{User: user, Pass: pass}.AsMechanism()))
	case "SCRAM-SHA-256":
		opts = append(opts, kgo.SASL(scram.Auth{User: user, Pass: pass}.AsSha256Mechanism()))
	case "SCRAM-SHA-512":
		opts = append(opts, kgo.SASL(scram.Auth{User: user, Pass: pass}.AsSha512Mechanism()))
	}
	return opts, nil
}

// tlsConfig parses inline credentials without reading files. With no custom CA,
// Go uses the system trust store and normal hostname verification.
func tlsConfig(cfg filament.Config) (*tls.Config, error) {
	if !cfg.Bool("tls_enabled") {
		return nil, nil
	}
	t := &tls.Config{MinVersion: tls.VersionTLS12}
	if pem := cfg.Secret("tls_ca_pem"); pem != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(pem)) {
			return nil, fmt.Errorf("kafka: tls_ca_pem must contain PEM CA certificates")
		}
		t.RootCAs = pool
	}
	if certPEM := cfg.Secret("tls_cert_pem"); certPEM != "" {
		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(cfg.Secret("tls_key_pem")))
		if err != nil {
			return nil, fmt.Errorf("kafka: invalid TLS certificate/private key PEM pair: %w", err)
		}
		t.Certificates = []tls.Certificate{cert}
	}
	return t, nil
}

var topicPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidateTopic rejects names outside Kafka topic naming rules.
func ValidateTopic(topic string) error {
	if topic == "" || len(topic) > 249 || topic == "." || topic == ".." || !topicPattern.MatchString(topic) {
		return fmt.Errorf("kafka: invalid topic %q", topic)
	}
	return nil
}

// Metadata never permits automatic topic creation.
func Metadata(ctx context.Context, c *kgo.Client, topics []string) (*kmsg.MetadataResponse, error) {
	req := kmsg.NewPtrMetadataRequest()
	for _, topic := range topics {
		t := kmsg.NewMetadataRequestTopic()
		t.Topic = &topic
		req.Topics = append(req.Topics, t)
	}
	res, err := req.RequestWith(ctx, c)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, t := range res.Topics {
		if err := kerr.ErrorForCode(t.ErrorCode); err != nil {
			return nil, err
		}
		if t.Topic == nil {
			return nil, fmt.Errorf("kafka: missing metadata topic")
		}
		seen[*t.Topic] = true
		for _, p := range t.Partitions {
			if err := kerr.ErrorForCode(p.ErrorCode); err != nil {
				return nil, err
			}
		}
	}
	for _, t := range topics {
		if !seen[t] {
			return nil, fmt.Errorf("kafka: topic %q missing", t)
		}
	}
	return res, nil
}

// TestConnection checks authentication and metadata access without producing records.
func TestConnection(ctx context.Context, cfg filament.Config) error {
	opts, err := Options(cfg)
	if err != nil {
		return err
	}
	c, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = Metadata(ctx, c, nil)
	return err
}
