package tunnel

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// bastion is an in-process SSH server that accepts one user key and forwards
// direct-tcpip channels, which is all a tunnel asks of a real bastion.
type bastion struct {
	address string
	hostKey string
	userKey string

	mu    sync.Mutex
	conns []net.Conn
}

func startBastion(t *testing.T) *bastion {
	t.Helper()
	hostSigner, _ := newKey(t)
	userSigner, userKey := newKey(t)
	server := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if bytes.Equal(key.Marshal(), userSigner.PublicKey().Marshal()) {
			return nil, nil
		}
		return nil, errors.New("unknown key")
	}}
	server.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	b := &bastion{
		address: listener.Addr().String(),
		hostKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostSigner.PublicKey()))),
		userKey: userKey,
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			b.mu.Lock()
			b.conns = append(b.conns, conn)
			b.mu.Unlock()
			go serveBastion(conn, server)
		}
	}()
	return b
}

func serveBastion(conn net.Conn, config *ssh.ServerConfig) {
	_, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	go ssh.DiscardRequests(requests)
	for request := range channels {
		var target struct {
			Host       string
			Port       uint32
			OriginHost string
			OriginPort uint32
		}
		if request.ChannelType() != "direct-tcpip" || ssh.Unmarshal(request.ExtraData(), &target) != nil {
			_ = request.Reject(ssh.UnknownChannelType, "only direct-tcpip")
			continue
		}
		upstream, err := net.Dial("tcp", net.JoinHostPort(target.Host, strconv.Itoa(int(target.Port))))
		if err != nil {
			_ = request.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}
		channel, channelRequests, err := request.Accept()
		if err != nil {
			_ = upstream.Close()
			continue
		}
		go ssh.DiscardRequests(channelRequests)
		go func() { _, _ = io.Copy(channel, upstream); _ = channel.Close() }()
		go func() { _, _ = io.Copy(upstream, channel); _ = upstream.Close() }()
	}
}

// drop closes every SSH connection the bastion holds.
func (b *bastion) drop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, conn := range b.conns {
		_ = conn.Close()
	}
	b.conns = nil
}

// ssh describes this bastion as a tunnel reads it.
func (b *bastion) ssh() SSH {
	return SSH{Address: b.address, User: "filament", PrivateKey: b.userKey, HostKey: b.hostKey}
}

func newKey(t *testing.T) (ssh.Signer, string) {
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
	return signer, string(pem.EncodeToMemory(block))
}

// startEcho stands in for the database: it echoes what it reads.
func startEcho(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(conn, conn); _ = conn.Close() }()
		}
	}()
	return listener.Addr().String()
}

func echo(t *testing.T, conn net.Conn) {
	t.Helper()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil || string(reply) != "ping" {
		t.Fatalf("echo = %q, %v", reply, err)
	}
}

func TestTunnelDialsThroughTheBastion(t *testing.T) {
	b := startBastion(t)
	database := startEcho(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tunnel, err := OpenSSH(b.ssh())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tunnel.Close() }()
	conn, err := tunnel.DialContext(ctx, "tcp", database)
	if err != nil {
		t.Fatal(err)
	}
	echo(t, conn)
	// A private hostname is left for the bastion to resolve.
	if hosts, err := tunnel.LookupHost(ctx, "db.internal"); err != nil || len(hosts) != 1 || hosts[0] != "db.internal" {
		t.Fatalf("lookup = %v, %v", hosts, err)
	}

	// A driver cancels a blocked read by setting a deadline.
	_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	var timeout net.Error
	if _, err := conn.Read(make([]byte, 1)); !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("read past the deadline: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	echo(t, conn)

	// A dropped SSH connection is replaced on the next dial.
	b.drop()
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("read on a dropped tunnel succeeded")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		next, err := tunnel.DialContext(ctx, "tcp", database)
		if err == nil {
			echo(t, next)
			_ = next.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no reconnect: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	_ = tunnel.Close()
	if _, err := tunnel.DialContext(ctx, "tcp", database); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("dial on a closed tunnel: %v", err)
	}
}

func TestTunnelChecksTheBastion(t *testing.T) {
	b := startBastion(t)
	database := startEcho(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	other := startBastion(t)
	_, strangerKey := newKey(t)

	anotherHost, unknownUser := b.ssh(), b.ssh()
	anotherHost.HostKey, unknownUser.PrivateKey = other.hostKey, strangerKey
	for name, bastion := range map[string]SSH{"another host key": anotherHost, "an unknown user": unknownUser} {
		tunnel, err := OpenSSH(bastion)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := tunnel.DialContext(ctx, "tcp", database); err == nil {
			t.Fatalf("%s: dial succeeded", name)
		}
		_ = tunnel.Close()
	}

	// ssh-keyscan prints the host in front of the key.
	scanned := b.ssh()
	scanned.HostKey = "bastion.internal " + b.hostKey
	tunnel, err := OpenSSH(scanned)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tunnel.Close() }()
	conn, err := tunnel.DialContext(ctx, "tcp", database)
	if err != nil {
		t.Fatal(err)
	}
	echo(t, conn)

	unreadableKey, unreadableHostKey := b.ssh(), b.ssh()
	unreadableKey.PrivateKey, unreadableHostKey.HostKey = "not a key", "not a key"
	for name, bastion := range map[string]SSH{"private key": unreadableKey, "host key": unreadableHostKey} {
		if _, err := OpenSSH(bastion); err == nil {
			t.Fatalf("unreadable %s accepted", name)
		}
	}
}

func TestReverseTunnelDialsItsListener(t *testing.T) {
	tunnel := OpenReverse(startEcho(t))
	defer func() { _ = tunnel.Close() }()
	// The driver asks for the target's own address, which only the far end knows.
	conn, err := tunnel.DialContext(context.Background(), "tcp", "db.internal:5432")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	echo(t, conn)
}

func TestNilTunnelClosesCleanly(t *testing.T) {
	var none *Tunnel
	if err := none.Close(); err != nil {
		t.Fatal(err)
	}
}
