package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/galaxy-io/filament"
)

// prepareKeyPairs settles every key-pair field before the config is validated
// and stored. A supplied private key shows its own public half, a stored key
// keeps the half already shown, and a field with neither gets a new pair.
func prepareKeyPairs(schema []filament.ConfigField, cfg map[string]any, refs map[string]string, stored map[string]any, parent string) error {
	for _, field := range schema {
		path := joinConfigPath(parent, field.Name)
		if field.PublicKeyField == "" {
			nested, ok := cfg[field.Name].(map[string]any)
			if !ok || len(field.Fields) == 0 {
				continue
			}
			storedNested, _ := stored[field.Name].(map[string]any)
			if err := prepareKeyPairs(field.Fields, nested, refs, storedNested, path); err != nil {
				return err
			}
			continue
		}
		if !fieldIsVisible(field, filament.NewConfig(cfg)) {
			continue
		}
		private, _ := cfg[field.Name].(string)
		switch {
		case strings.TrimSpace(private) != "":
			public, err := sshPublicKey(private)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			cfg[field.PublicKeyField] = public
		case refs[path] != "":
			delete(cfg, field.PublicKeyField)
			if public, ok := stored[field.PublicKeyField]; ok {
				cfg[field.PublicKeyField] = public
			}
		default:
			private, public, err := generateSSHKey()
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			cfg[field.Name], cfg[field.PublicKeyField] = private, public
		}
	}
	return nil
}

// generateSSHKey returns a new ed25519 key as an OpenSSH private key and the
// authorized_keys line of its public half.
func generateSSHKey() (private, public string, err error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	block, err := ssh.MarshalPrivateKey(key, "")
	if err != nil {
		return "", "", err
	}
	private = string(pem.EncodeToMemory(block))
	public, err = sshPublicKey(private)
	return private, public, err
}

func sshPublicKey(private string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(private))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), nil
}
