package dbconfig

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"maps"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/galaxy-io/filament"
)

// sshSection is a complete SSH tunnel section with the overrides applied.
func sshSection(t *testing.T, overrides map[string]any) filament.Config {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(private, "")
	if err != nil {
		t.Fatal(err)
	}
	section := map[string]any{
		tunnelMethodField: TunnelSSH, "bastion_host": "bastion.internal", "bastion_user": "filament",
		"private_key": string(pem.EncodeToMemory(block)),
		"host_key":    strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))),
	}
	maps.Copy(section, overrides)
	return filament.NewConfig(map[string]any{TunnelField: section})
}

func TestOpenTunnelReadsItsConfig(t *testing.T) {
	for _, cfg := range []map[string]any{nil, {TunnelField: map[string]any{tunnelMethodField: TunnelNone}}} {
		tunnel, err := OpenTunnel(filament.NewConfig(cfg), "")
		if err != nil || tunnel != nil {
			t.Fatalf("no tunnel configured: %v, %v", tunnel, err)
		}
	}
	if tunnel, err := OpenTunnel(sshSection(t, nil), ""); err != nil || tunnel == nil {
		t.Fatalf("ssh tunnel: %v, %v", tunnel, err)
	}
	for name, overrides := range map[string]map[string]any{
		"unknown method":  {tunnelMethodField: "vpn"},
		"no host":         {"bastion_host": ""},
		"bad port":        {"bastion_port": 70000},
		"no user":         {"bastion_user": ""},
		"no private key":  {"private_key": ""},
		"bad private key": {"private_key": "not a key"},
		"no host key":     {"host_key": ""},
		"bad host key":    {"host_key": "not a key"},
	} {
		if _, err := OpenTunnel(sshSection(t, overrides), ""); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

func TestReverseTunnelNeedsAListenerHost(t *testing.T) {
	reverse := func(port any) filament.Config {
		return filament.NewConfig(map[string]any{TunnelField: map[string]any{tunnelMethodField: TunnelReverse, "tunnel_port": port}})
	}
	methods := func(host string) []string {
		var values []string
		for _, option := range TunnelConfigField(host).Fields[0].Enum {
			values = append(values, option.Value)
		}
		return values
	}

	// A deployment without a listener host does not offer the method.
	if got := methods(""); !slices.Equal(got, []string{TunnelNone, TunnelSSH}) {
		t.Fatalf("methods = %v", got)
	}
	if _, err := OpenTunnel(reverse(15432), ""); err == nil {
		t.Fatal("reverse tunnel accepted without a listener host")
	}

	const host = "tunnel.internal"
	if got := methods(host); !slices.Equal(got, []string{TunnelNone, TunnelSSH, TunnelReverse}) {
		t.Fatalf("methods = %v", got)
	}
	fields := TunnelConfigField(host).Fields
	if port := fields[len(fields)-1]; port.Name != "tunnel_port" || !port.Required {
		t.Fatalf("last field = %+v", port)
	}
	if tunnel, err := OpenTunnel(reverse(15432), host); err != nil || tunnel == nil {
		t.Fatalf("reverse tunnel: %v, %v", tunnel, err)
	}
	for _, port := range []any{nil, 0, 70000} {
		if _, err := OpenTunnel(reverse(port), host); err == nil {
			t.Fatalf("port %v accepted", port)
		}
	}
}
