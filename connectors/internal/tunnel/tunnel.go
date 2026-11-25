// Package tunnel dials for a connector whose target cannot be reached
// directly. It knows nothing about a connector's config or its driver: it
// returns a dial function any driver that accepts one can use.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	sshHandshakeTimeout = 30 * time.Second
	// A NAT gateway drops a connection that stays idle, AWS after 350 seconds,
	// so the bastion connection sends TCP keep-alive probes well inside that.
	sshKeepAlive = 30 * time.Second
)

// Tunnel dials a target that cannot be reached directly. One SSH connection
// carries every connection opened through it.
type Tunnel struct {
	address string
	// config is nil for a reverse tunnel, whose listener is dialed as is.
	config *ssh.ClientConfig

	mu     sync.Mutex
	client *ssh.Client
	closed bool
}

// SSH describes a bastion to dial through.
type SSH struct {
	// Address is the bastion's host:port.
	Address string
	User    string
	// PrivateKey is an OpenSSH private key the bastion user accepts.
	PrivateKey string
	// HostKey pins the bastion, in authorized_keys form or as ssh-keyscan
	// prints it with the host in front.
	HostKey string
}

// OpenSSH returns a tunnel through a bastion. Nothing connects until the first
// dial.
func OpenSSH(bastion SSH) (*Tunnel, error) {
	signer, err := ssh.ParsePrivateKey([]byte(bastion.PrivateKey))
	if err != nil {
		var passphrase *ssh.PassphraseMissingError
		if errors.As(err, &passphrase) {
			return nil, errors.New("private key is passphrase-protected, which is not supported")
		}
		return nil, fmt.Errorf("private key: %w", err)
	}
	hostKey, err := parseHostKey(bastion.HostKey)
	if err != nil {
		return nil, fmt.Errorf("host key: %w", err)
	}
	return &Tunnel{address: bastion.Address, config: &ssh.ClientConfig{
		User:            bastion.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		// The bastion has to present the pinned key, not another one it holds.
		HostKeyAlgorithms: hostKeyAlgorithms(hostKey),
		Timeout:           sshHandshakeTimeout,
	}}, nil
}

// OpenReverse returns a tunnel that dials the listener a reverse SSH tunnel
// opened, whatever address the driver asks for.
func OpenReverse(address string) *Tunnel {
	return &Tunnel{address: address}
}

func parseHostKey(raw string) (ssh.PublicKey, error) {
	raw = strings.TrimSpace(raw)
	if key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(raw)); err == nil {
		return key, nil
	}
	_, _, key, _, _, err := ssh.ParseKnownHosts([]byte(raw))
	return key, err
}

func hostKeyAlgorithms(key ssh.PublicKey) []string {
	if key.Type() == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	}
	return []string{key.Type()}
}

// DialContext opens a connection to addr through the tunnel. It has the
// signature drivers take as a dial function.
func (t *Tunnel) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if t.config == nil {
		// The listener already ends at the target.
		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, network, t.address)
		if err != nil {
			return nil, fmt.Errorf("reverse ssh tunnel: dial %s: %w", t.address, err)
		}
		return conn, nil
	}
	client, err := t.connect(ctx)
	if err != nil {
		return nil, err
	}
	remote, err := client.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("ssh tunnel: dial %s: %w", addr, err)
	}
	return bridge(remote), nil
}

// LookupHost leaves name resolution to the far end of the tunnel, where a
// private hostname resolves.
func (t *Tunnel) LookupHost(_ context.Context, host string) ([]string, error) {
	return []string{host}, nil
}

// Close drops the SSH connection and every connection on it.
func (t *Tunnel) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	if t.client == nil {
		return nil
	}
	err := t.client.Close()
	t.client = nil
	return err
}

// connect returns the SSH connection, opening it when there is none. A dropped
// connection is forgotten, so the next dial opens a new one.
func (t *Tunnel) connect(ctx context.Context) (*ssh.Client, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil, fmt.Errorf("ssh tunnel: %w", net.ErrClosed)
	}
	if t.client != nil {
		return t.client, nil
	}
	dialer := net.Dialer{KeepAlive: sshKeepAlive}
	conn, err := dialer.DialContext(ctx, "tcp", t.address)
	if err != nil {
		return nil, fmt.Errorf("ssh tunnel: connect %s: %w", t.address, err)
	}
	// The handshake takes no context, so a deadline bounds it.
	deadline := time.Now().Add(sshHandshakeTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)
	sshConn, channels, requests, err := ssh.NewClientConn(conn, t.address, t.config)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh tunnel: connect %s: %w", t.address, err)
	}
	_ = conn.SetDeadline(time.Time{})
	client := ssh.NewClient(sshConn, channels, requests)
	t.client = client
	go func() {
		_ = client.Wait()
		t.mu.Lock()
		if t.client == client {
			t.client = nil
		}
		t.mu.Unlock()
	}()
	return client, nil
}

// bridge gives a driver a connection that honours deadlines, which an SSH
// channel does not. Drivers cancel blocked reads by setting one.
func bridge(remote net.Conn) net.Conn {
	local, far := net.Pipe()
	go func() {
		_, _ = io.Copy(far, remote)
		_ = far.Close()
		_ = remote.Close()
	}()
	go func() {
		_, _ = io.Copy(remote, far)
		_ = remote.Close()
		_ = far.Close()
	}()
	return local
}
