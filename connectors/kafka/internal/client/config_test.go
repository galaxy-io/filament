package client

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

func TestValidation(t *testing.T) {
	for _, config := range []map[string]any{{}, {"brokers": "host:9092"}, {"brokers": []string{""}}, {"brokers": []string{"host:9092"}, "tls_enabled": "true"}, {"brokers": []string{"host:9092"}, "sasl_mechanism": "PLAIN"}, {"brokers": []string{"host:9092"}, "sasl_password": "secret"}, {"brokers": []string{"host:9092"}, "tls_cert_file": "cert"}} {
		if err := Validate(filament.NewConfig(config)); err == nil {
			t.Fatalf("invalid config accepted: %v", config)
		}
	}
	for _, mechanism := range []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		_, err := Options(filament.NewConfig(map[string]any{"brokers": []string{"host:9092"}, "tls_enabled": true, "sasl_mechanism": mechanism, "sasl_username": "user", "sasl_password": "secret"}))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestTopicValidation(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a b", "a/b", "orders.*"} {
		if ValidateTopic(name) == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	for _, name := range []string{"orders", "out.orders", "a_B-1"} {
		if err := ValidateTopic(name); err != nil {
			t.Fatal(err)
		}
	}
}

func pemPair(t *testing.T) (string, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, private)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))
}

func TestInlineTLSCredentials(t *testing.T) {
	cert, key := pemPair(t)
	cfg := map[string]any{"brokers": []string{"localhost:9092"}, "tls_enabled": true, "tls_ca_pem": cert, "tls_cert_pem": cert, "tls_key_pem": key}
	if err := Validate(filament.NewConfig(cfg)); err != nil {
		t.Fatal(err)
	}
	tlsCfg, err := tlsConfig(filament.NewConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if tlsCfg.InsecureSkipVerify || tlsCfg.MinVersion != tls.VersionTLS12 || tlsCfg.RootCAs == nil || len(tlsCfg.Certificates) != 1 {
		t.Fatal("inline TLS settings were not applied securely")
	}
	leaf, err := x509.ParseCertificate(tlsCfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: tlsCfg.RootCAs, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		t.Fatal(err)
	}
	defaults, err := tlsConfig(filament.NewConfig(map[string]any{"tls_enabled": true}))
	if err != nil || defaults.RootCAs != nil || len(defaults.Certificates) != 0 {
		t.Fatal("TLS without PEM must retain system trust", err)
	}
	_, wrongKey := pemPair(t)
	for _, invalid := range []map[string]any{
		{"tls_cert_pem": cert}, {"tls_key_pem": key},
		{"tls_enabled": false, "tls_ca_pem": cert},
		{"tls_ca_pem": "not PEM"},
		{"tls_cert_pem": cert, "tls_key_pem": wrongKey},
		{"tls_cert_pem": "not PEM", "tls_key_pem": key},
		{"tls_ca_file": "/tmp/ca.pem"}, {"tls_cert_file": "/tmp/cert.pem"}, {"tls_key_file": "/tmp/key.pem"},
		{"sasl_mechanism": "none", "sasl_username": "user", "sasl_password": "password"},
	} {
		values := map[string]any{"brokers": []string{"localhost:9092"}, "tls_enabled": true}
		for k, v := range invalid {
			values[k] = v
		}
		if err := Validate(filament.NewConfig(values)); err == nil {
			t.Fatal("invalid TLS/auth configuration accepted")
		} else if strings.Contains(err.Error(), key) {
			t.Fatal("private key leaked into error")
		}
	}
}

func TestAuthenticationCatalog(t *testing.T) {
	fields := map[string]filament.ConfigField{}
	for _, field := range Fields() {
		if strings.HasSuffix(field.Name, "_file") {
			t.Fatal("file input exposed")
		}
		fields[field.Name] = field
	}
	for _, name := range []string{"tls_ca_pem", "tls_cert_pem", "tls_key_pem", "sasl_password"} {
		if fields[name].Type != filament.FieldSecret {
			t.Fatalf("%s is not stored as a secret", name)
		}
	}
	if fields["sasl_mechanism"].Type != filament.FieldEnum || fields["sasl_mechanism"].Default != "none" {
		t.Fatal("missing authentication dropdown")
	}
	for _, name := range []string{"sasl_username", "sasl_password"} {
		field := fields[name]
		if !field.Required || field.VisibleWhen == nil || field.VisibleWhen.Field != "sasl_mechanism" || len(field.VisibleWhen.Values) != 3 {
			t.Fatalf("incorrect SASL visibility for %s", name)
		}
	}
	for _, mode := range []string{"", "none"} {
		if _, err := Options(filament.NewConfig(map[string]any{"brokers": []string{"localhost:9092"}, "sasl_mechanism": mode})); err != nil {
			t.Fatal(err)
		}
	}
}
