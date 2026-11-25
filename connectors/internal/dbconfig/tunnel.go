package dbconfig

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/tunnel"
)

const (
	// TunnelField is the config object describing how the database is reached.
	TunnelField = "ssh_configuration"

	// TunnelNone dials the database directly.
	TunnelNone = "none"
	// TunnelSSH dials the database through an SSH bastion.
	TunnelSSH = "ssh"
	// TunnelReverse dials the port a reverse SSH tunnel opened on this
	// deployment's listener host.
	TunnelReverse = "reverse_ssh"

	tunnelMethodField = "method"
	defaultSSHPort    = 22
)

// TunnelConfigField declares the tunnel settings shared by database connectors.
// The reverse method is offered only where the deployment has a listener host.
func TunnelConfigField(reverseTunnelHost string) filament.ConfigField {
	forward := &filament.FieldCondition{Field: tunnelMethodField, Values: []string{TunnelSSH}}
	field := filament.ConfigField{
		Name: TunnelField, Type: filament.FieldObject, Scope: filament.ScopeConnection,
		Help: "Reach a database in a private network through an SSH tunnel",
		Fields: []filament.ConfigField{
			{
				Name: tunnelMethodField, Type: filament.FieldEnum, Default: TunnelNone,
				Enum: []filament.EnumOption{
					{Value: TunnelNone, Label: "None"},
					{Value: TunnelSSH, Label: "SSH tunnel"},
				},
				Scope: filament.ScopeConnection,
				Help:  "Dial the database directly or through an SSH bastion",
			},
			{Name: "bastion_host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: forward, Help: "SSH bastion hostname"},
			{Name: "bastion_port", Type: filament.FieldInt, Default: defaultSSHPort, Scope: filament.ScopeConnection, VisibleWhen: forward, Help: "SSH bastion port"},
			{Name: "bastion_user", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: forward, Help: "SSH bastion user"},
			{Name: "private_key", Type: filament.FieldSecret, PublicKeyField: "galaxy_public_key", Scope: filament.ScopeConnection, VisibleWhen: forward, Help: "Private key the bastion user accepts. Generated when left empty."},
			{Name: "galaxy_public_key", Type: filament.FieldString, ReadOnly: true, Scope: filament.ScopeConnection, VisibleWhen: forward, Help: "Add this key to the bastion user's authorized_keys"},
			{Name: "host_key", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: forward, Help: "Bastion host key, as ssh-keyscan prints it"},
		},
	}
	if reverseTunnelHost == "" {
		return field
	}
	reverse := &filament.FieldCondition{Field: tunnelMethodField, Values: []string{TunnelReverse}}
	field.Fields[0].Enum = append(field.Fields[0].Enum, filament.EnumOption{Value: TunnelReverse, Label: "Reverse SSH tunnel"})
	field.Fields = append(field.Fields, filament.ConfigField{
		Name: "tunnel_port", Type: filament.FieldInt, Required: true, Scope: filament.ScopeConnection, VisibleWhen: reverse,
		Help: "Port assigned to your reverse tunnel",
	})
	return field
}

// OpenTunnel returns the tunnel a connection is configured with, or nil when
// the database is dialed directly. reverseTunnelHost is where this deployment's
// reverse tunnels listen, or empty. Nothing connects until the first dial.
func OpenTunnel(cfg filament.Config, reverseTunnelHost string) (*tunnel.Tunnel, error) {
	section := cfg.Sub(TunnelField)
	switch method := section.String(tunnelMethodField); method {
	case "", TunnelNone:
		return nil, nil
	case TunnelSSH:
		return openSSHTunnel(section)
	case TunnelReverse:
		if reverseTunnelHost == "" {
			return nil, fmt.Errorf("%s.%s %q is not available on this deployment", TunnelField, tunnelMethodField, TunnelReverse)
		}
		port := section.Int("tunnel_port")
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("%s.tunnel_port must be between 1 and 65535", TunnelField)
		}
		return tunnel.OpenReverse(net.JoinHostPort(reverseTunnelHost, strconv.Itoa(port))), nil
	default:
		return nil, fmt.Errorf("%s.%s must be %q, %q or %q", TunnelField, tunnelMethodField, TunnelNone, TunnelSSH, TunnelReverse)
	}
}

func openSSHTunnel(section filament.Config) (*tunnel.Tunnel, error) {
	address, err := tunnelAddress(section, "bastion_host", "bastion_port", defaultSSHPort)
	if err != nil {
		return nil, err
	}
	bastion := tunnel.SSH{Address: address, User: section.String("bastion_user"), PrivateKey: section.Secret("private_key"), HostKey: section.String("host_key")}
	for field, value := range map[string]string{"bastion_user": bastion.User, "private_key": bastion.PrivateKey, "host_key": bastion.HostKey} {
		if strings.TrimSpace(value) == "" {
			return nil, requiredTunnelField(field, TunnelSSH)
		}
	}
	opened, err := tunnel.OpenSSH(bastion)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", TunnelField, err)
	}
	return opened, nil
}

func requiredTunnelField(field, method string) error {
	return fmt.Errorf("%s.%s is required when %s.%s is %q", TunnelField, field, TunnelField, tunnelMethodField, method)
}

func tunnelAddress(section filament.Config, hostField, portField string, defaultPort int) (string, error) {
	host := strings.TrimSpace(section.String(hostField))
	if host == "" {
		return "", requiredTunnelField(hostField, section.String(tunnelMethodField))
	}
	port := section.Int(portField)
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("%s.%s must be between 1 and 65535", TunnelField, portField)
	}
	return net.JoinHostPort(strings.TrimPrefix(strings.TrimSuffix(host, "]"), "["), strconv.Itoa(port)), nil
}
