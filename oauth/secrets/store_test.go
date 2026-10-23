package secrets_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/oauth"
	oauthsecrets "github.com/galaxy-io/filament/oauth/secrets"
)

const tenant = filament.TenantID("11111111-1111-4111-8111-111111111111")

type recordingProvider struct {
	filament.Secrets
	value         filament.Secret
	reads, writes int
	fail          bool
}

func (p *recordingProvider) Read(context.Context, string) (filament.Secret, error) {
	p.reads++
	return p.value, nil
}

func (p *recordingProvider) RotateSecret(_ context.Context, _ string, v filament.Secret) error {
	p.writes++
	if p.fail {
		return errors.New("save failed")
	}
	p.value = v
	return nil
}

func bound(t *testing.T, p filament.Secrets) *oauthsecrets.Store {
	t.Helper()
	s, err := oauthsecrets.FromRefreshToken(p, tenant, filament.ConnectionSecretRef(string(tenant), "connection", "refresh_token", 1), "binding")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRunScopedWriteBack(t *testing.T) {
	p := &recordingProvider{value: filament.Secret{Value: []byte("initial"), Meta: map[string]string{"field": "refresh_token"}}}
	s := bound(t, p)
	token := oauth.Token{AccessToken: "access", RefreshToken: "initial", ExpiresAt: time.Now().Add(time.Hour)}
	for range 2 {
		got, err := s.Update(t.Context(), func(_ context.Context, current oauth.Token) (oauth.Token, error) {
			if current.RefreshToken != "initial" {
				t.Fatal("lost refresh token")
			}
			return token, nil
		})
		if err != nil || got != token {
			t.Fatalf("update: %v", err)
		}
	}
	if p.reads != 1 || p.writes != 0 {
		t.Fatalf("reads=%d writes=%d", p.reads, p.writes)
	}
	token.RefreshToken = "rotated"
	if _, err := s.Update(t.Context(), func(context.Context, oauth.Token) (oauth.Token, error) { return token, nil }); err != nil {
		t.Fatal(err)
	}
	if p.writes != 1 || string(p.value.Value) != "rotated" || p.value.Meta["field"] != "refresh_token" {
		t.Fatal("rotation did not persist only refresh token")
	}
	got, err := bound(t, p).Update(t.Context(), func(_ context.Context, v oauth.Token) (oauth.Token, error) { return v, nil })
	if err != nil || got != (oauth.Token{RefreshToken: "rotated"}) {
		t.Fatal("access token leaked into another run")
	}
	if p.writes != 1 {
		t.Fatal("unchanged refresh token was written")
	}
}

func TestSaveFailureRetainsExchange(t *testing.T) {
	p := &recordingProvider{value: filament.Secret{Value: []byte("initial")}, fail: true}
	s := bound(t, p)
	next := oauth.Token{AccessToken: "access", RefreshToken: "rotated"}
	if _, err := s.Update(t.Context(), func(context.Context, oauth.Token) (oauth.Token, error) { return next, nil }); err == nil {
		t.Fatal("save failure hidden")
	}
	p.fail = false
	got, err := s.Update(t.Context(), func(context.Context, oauth.Token) (oauth.Token, error) {
		t.Fatal("repeated exchange after save failure")
		return oauth.Token{}, nil
	})
	if err != nil || got != next || p.reads != 1 || p.writes != 2 {
		t.Fatalf("retry: %v, reads=%d writes=%d", err, p.reads, p.writes)
	}
}

func TestConcurrentCallersShareRunMemory(t *testing.T) {
	p := &recordingProvider{value: filament.Secret{Value: []byte("initial")}}
	s := bound(t, p)
	var exchanges atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, err := s.Update(t.Context(), func(_ context.Context, v oauth.Token) (oauth.Token, error) {
				if v.AccessToken == "" {
					exchanges.Add(1)
					v.AccessToken = "access"
					v.RefreshToken = "rotated"
				}
				return v, nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if exchanges.Load() != 1 || p.reads != 1 || p.writes != 1 {
		t.Fatal("run callers raced refresh")
	}
}

func TestCallbackFailureAndCancellation(t *testing.T) {
	p := &recordingProvider{value: filament.Secret{Value: []byte("initial")}}
	s := bound(t, p)
	rejected := errors.New("rejected")
	_, err := s.Update(t.Context(), func(context.Context, oauth.Token) (oauth.Token, error) {
		return oauth.Token{RefreshToken: "bad"}, rejected
	})
	if !errors.Is(err, rejected) || p.writes != 0 {
		t.Fatal("failed callback committed")
	}
	got, err := s.Update(t.Context(), func(_ context.Context, v oauth.Token) (oauth.Token, error) { return v, nil })
	if err != nil || got.RefreshToken != "initial" || p.reads != 1 {
		t.Fatal("failed callback lost seed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = s.Update(ctx, func(context.Context, oauth.Token) (oauth.Token, error) {
		t.Fatal("canceled callback ran")
		return oauth.Token{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
}

func TestLegacyPayloadUsesOnlyRefreshToken(t *testing.T) {
	p := &recordingProvider{value: filament.Secret{Value: []byte(`{"token":{"access_token":"old-access","refresh_token":"initial"},"binding":"binding"}`), Meta: map[string]string{"oauth_format": "v1"}}}
	s := bound(t, p)
	_, err := s.Update(t.Context(), func(_ context.Context, v oauth.Token) (oauth.Token, error) {
		if v.AccessToken != "" || v.RefreshToken != "initial" {
			t.Fatal("legacy access token reused")
		}
		return oauth.Token{AccessToken: "new-access", RefreshToken: "rotated"}, nil
	})
	if err != nil || string(p.value.Value) != "rotated" || p.value.Meta["oauth_format"] != "" {
		t.Fatal("legacy payload was not replaced on rotation")
	}
}
