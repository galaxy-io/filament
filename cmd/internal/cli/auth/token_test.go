package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
)

// fakeServer answers GetToken the way a Filament server does, recording how
// it was called.
type fakeServer struct {
	authv1connect.UnimplementedAuthServiceHandler
	server      *httptest.Server
	tokenCalls  int
	lastID      string
	lastSecret  string
	accessToken string
	reject      bool
}

func (f *fakeServer) GetToken(_ context.Context, req *connect.Request[authv1.GetTokenRequest]) (*connect.Response[authv1.GetTokenResponse], error) {
	f.tokenCalls++
	f.lastID, f.lastSecret = req.Msg.GetClientId(), req.Msg.GetClientSecret()
	if f.reject {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	return connect.NewResponse(&authv1.GetTokenResponse{AccessToken: f.accessToken, ExpiresIn: 3600}), nil
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	fake := &fakeServer{accessToken: "minted-token"}
	mux := http.NewServeMux()
	mux.Handle(authv1connect.NewAuthServiceHandler(fake))
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func testSource(t *testing.T, fake *fakeServer) Source {
	t.Helper()
	store := testStore(t)
	profile := testProfile()
	profile.Server = fake.server.URL
	if err := store.Put("prod", profile); err != nil {
		t.Fatalf("put: %v", err)
	}
	return Source{Store: store, Profile: "prod", Client: fake.server.Client()}
}

func TestSourceMintsAndPersists(t *testing.T) {
	fake := newFakeServer(t)
	source := testSource(t, fake)
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if token != "minted-token" {
		t.Fatalf("token %q, want minted-token", token)
	}
	if fake.lastID != testProfile().ClientID || fake.lastSecret != testProfile().ClientSecret {
		t.Fatalf("credentials %q/%q", fake.lastID, fake.lastSecret)
	}
	profile, err := source.Store.Get("prod")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if profile.Cache == nil || profile.Cache.AccessToken != "minted-token" {
		t.Fatalf("cache not persisted: %+v", profile.Cache)
	}
	if remaining := time.Until(profile.Cache.ExpiresAt); remaining < 55*time.Minute {
		t.Fatalf("cached expiry only %s away", remaining)
	}
}

func TestSourceReusesCachedToken(t *testing.T) {
	fake := newFakeServer(t)
	source := testSource(t, fake)
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatalf("first token: %v", err)
	}
	fake.accessToken = "should-not-be-minted"
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("second token: %v", err)
	}
	if token != "minted-token" {
		t.Fatalf("token %q, want cached minted-token", token)
	}
	if fake.tokenCalls != 1 {
		t.Fatalf("token endpoint called %d times, want 1", fake.tokenCalls)
	}
}

func TestSourceRefreshesNearExpiry(t *testing.T) {
	fake := newFakeServer(t)
	source := testSource(t, fake)
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatalf("first token: %v", err)
	}
	fake.accessToken = "second-token"
	source.Now = func() time.Time { return time.Now().Add(time.Hour) }
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("second token: %v", err)
	}
	if token != "second-token" {
		t.Fatalf("token %q, want second-token", token)
	}
	if fake.tokenCalls != 2 {
		t.Fatalf("token endpoint called %d times, want 2", fake.tokenCalls)
	}
}

func TestSourceUnknownProfile(t *testing.T) {
	source := Source{Store: testStore(t), Profile: "nope"}
	if _, err := source.Token(context.Background()); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestSourceRejectedCredentials(t *testing.T) {
	fake := newFakeServer(t)
	fake.reject = true
	profile := testProfile()
	profile.Server = fake.server.URL
	if _, _, err := Mint(context.Background(), profile, fake.server.Client()); !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("err %v, want ErrTokenRejected", err)
	}
}
