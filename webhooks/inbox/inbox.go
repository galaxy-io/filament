// Package inbox stores accepted webhook deliveries in a dedicated JetStream stream.
//
// Each message's data is the original request body. Delivery identity travels in
// headers. Every delivery has its own subject, derived from tenant, endpoint and
// provider delivery ID, and the stream keeps one message per subject. A retry of
// an accepted delivery therefore resolves to the original instead of a copy.
//
// Deliveries carry no pipeline routing. A consumer decides where a delivery goes
// when it reads it, so a pipeline change never strands accepted deliveries.
package inbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// DefaultStream is the JetStream stream that stores webhook deliveries.
	DefaultStream = "WEBHOOK_INBOX"
	subjectRoot   = "filament.webhook.v1"

	// HeaderAllowance is reserved in every message for identity headers.
	HeaderAllowance = 1 << 10
)

const (
	hdrID         = "Filament-Delivery"
	hdrTenant     = "Filament-Tenant"
	hdrEndpoint   = "Filament-Endpoint"
	hdrProviderID = "Filament-Provider-Delivery"
	hdrResource   = "Filament-Resource"
	hdrDigest     = "Filament-Digest"
	hdrAcceptedAt = "Filament-Accepted-At"
)

var (
	// ErrConflict indicates that a delivery ID was reused with different content.
	ErrConflict = errors.New("webhook delivery ID already exists with different content")
	// ErrInvalid indicates that a delivery or inbox configuration is invalid.
	ErrInvalid = errors.New("invalid webhook delivery")
	// ErrTooLarge indicates that a delivery exceeds the configured inbox message limit.
	ErrTooLarge = errors.New("webhook delivery exceeds inbox message limit")
	// ErrNotFound indicates that an accepted delivery was not found.
	ErrNotFound = errors.New("webhook delivery not found")
)

// Config is the stream's capacity and durability contract.
type Config struct {
	MaxBytes   int64
	MaxMsgSize int32
	Replicas   int
}

// Delivery is an authenticated, immutable webhook body and its identity.
type Delivery struct {
	ID                 string
	Tenant             string
	Endpoint           string
	ProviderDeliveryID string
	Resource           string
	AcceptedAt         time.Time
	Body               []byte
	Digest             string
}

// Receipt identifies durable acceptance, not destination completion.
type Receipt struct {
	DeliveryID string
	Sequence   uint64
	Duplicate  bool
	AcceptedAt time.Time
}

// Inbox owns a single, separately managed JetStream stream.
type Inbox struct {
	js          jetstream.JetStream
	stream      jetstream.Stream
	name        string
	incarnation time.Time
	maxMsgSize  int32
}

// Open creates a missing inbox stream or validates the existing stream's
// safety-critical configuration. It never modifies a live stream.
func Open(ctx context.Context, js jetstream.JetStream, cfg Config) (*Inbox, error) {
	if js == nil || cfg.MaxBytes <= 0 || cfg.MaxMsgSize <= HeaderAllowance || cfg.Replicas < 1 || cfg.Replicas > 5 {
		return nil, fmt.Errorf("inbox: %w: stream configuration", ErrInvalid)
	}
	want := jetstream.StreamConfig{
		Name:                 DefaultStream,
		Subjects:             []string{subjectRoot + ".>"},
		Retention:            jetstream.LimitsPolicy,
		Storage:              jetstream.FileStorage,
		Discard:              jetstream.DiscardNew,
		DiscardNewPerSubject: true,
		MaxMsgsPerSubject:    1,
		MaxBytes:             cfg.MaxBytes,
		MaxMsgSize:           cfg.MaxMsgSize,
		Replicas:             cfg.Replicas,
	}
	stream, err := js.Stream(ctx, DefaultStream)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		stream, err = js.CreateStream(ctx, want)
		if errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			stream, err = js.Stream(ctx, DefaultStream)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("inbox: open stream: %w", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("inbox: inspect stream: %w", err)
	}
	got := info.Config
	if len(got.Subjects) != 1 || got.Subjects[0] != want.Subjects[0] ||
		got.Retention != want.Retention || got.Storage != want.Storage ||
		got.Discard != want.Discard || !got.DiscardNewPerSubject ||
		got.MaxMsgsPerSubject != 1 || got.MaxBytes != want.MaxBytes ||
		got.MaxMsgSize != want.MaxMsgSize || got.Replicas != want.Replicas ||
		got.MaxAge != 0 || (got.MaxMsgs != 0 && got.MaxMsgs != -1) {
		return nil, errors.New("inbox: existing stream has incompatible retention or capacity")
	}
	return &Inbox{js: js, stream: stream, name: DefaultStream, incarnation: info.Created, maxMsgSize: cfg.MaxMsgSize}, nil
}

// Attach opens the existing inbox without provisioning it.
func Attach(ctx context.Context, js jetstream.JetStream) (*Inbox, error) {
	if js == nil {
		return nil, ErrInvalid
	}
	stream, err := js.Stream(ctx, DefaultStream)
	if err != nil {
		return nil, err
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return nil, err
	}
	cfg := info.Config
	return Open(ctx, js, Config{MaxBytes: cfg.MaxBytes, MaxMsgSize: cfg.MaxMsgSize, Replicas: cfg.Replicas})
}

// StreamName returns the name of the durable inbox stream.
func (i *Inbox) StreamName() string { return i.name }

// Incarnation returns the stream creation time used to namespace positions.
func (i *Inbox) Incarnation() time.Time { return i.incarnation }

// MaxBody is the largest body the inbox stores.
func (i *Inbox) MaxBody() int64 { return int64(i.maxMsgSize) - HeaderAllowance }

// Accept publishes a new immutable delivery. A retained original with the same
// endpoint-scoped provider ID returns its original receipt. A failed publish
// with no retained original remains retryable, including when the publish
// outcome is ambiguous.
func (i *Inbox) Accept(ctx context.Context, d Delivery) (Receipt, error) {
	if i == nil || d.Tenant == "" || d.Endpoint == "" || d.ProviderDeliveryID == "" || d.Resource == "" {
		return Receipt{}, ErrInvalid
	}
	if int64(len(d.Body)) > i.MaxBody() {
		return Receipt{}, ErrTooLarge
	}
	d.ID = uuid.NewString()
	d.AcceptedAt = time.Now().UTC()
	d.Digest = digest(d.Body)
	subject := Subject(d.Tenant, d.Endpoint, d.ProviderDeliveryID)
	msg := &nats.Msg{Subject: subject, Data: d.Body, Header: nats.Header{}}
	msg.Header.Set(hdrID, d.ID)
	msg.Header.Set(hdrTenant, d.Tenant)
	msg.Header.Set(hdrEndpoint, d.Endpoint)
	msg.Header.Set(hdrProviderID, d.ProviderDeliveryID)
	msg.Header.Set(hdrResource, d.Resource)
	msg.Header.Set(hdrDigest, d.Digest)
	msg.Header.Set(hdrAcceptedAt, d.AcceptedAt.Format(time.RFC3339Nano))
	ack, publishErr := i.js.PublishMsg(ctx, msg, jetstream.WithExpectStream(i.name))
	if publishErr == nil && ack != nil && ack.Stream == i.name && !ack.Duplicate {
		return Receipt{DeliveryID: d.ID, Sequence: ack.Sequence, AcceptedAt: d.AcceptedAt}, nil
	}
	// The per-subject limit rejects duplicates. Also look after an ambiguous
	// publish error, since the server may have stored the message before the
	// reply was lost.
	lookupCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		lookupCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
	}
	receipt, lookupErr := i.retained(lookupCtx, d.Tenant, d.Endpoint, d.ProviderDeliveryID, d.Digest)
	if lookupErr == nil || errors.Is(lookupErr, ErrConflict) {
		return receipt, lookupErr
	}
	if publishErr != nil {
		return Receipt{}, fmt.Errorf("inbox: publish: %w", publishErr)
	}
	return Receipt{}, fmt.Errorf("inbox: publish result unresolved: %w", lookupErr)
}

// Resolve returns the original receipt for an authenticated retry. It never
// creates a delivery and checks the original body before returning a receipt.
func (i *Inbox) Resolve(ctx context.Context, tenant, endpoint, providerID string, body []byte) (Receipt, error) {
	if i == nil || tenant == "" || endpoint == "" || providerID == "" {
		return Receipt{}, ErrInvalid
	}
	return i.retained(ctx, tenant, endpoint, providerID, digest(body))
}

func (i *Inbox) retained(ctx context.Context, tenant, endpoint, providerID, wantDigest string) (Receipt, error) {
	raw, err := i.stream.GetLastMsgForSubject(ctx, Subject(tenant, endpoint, providerID))
	if errors.Is(err, jetstream.ErrMsgNotFound) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	d, err := decode(raw.Subject, raw.Header, raw.Data)
	if err != nil {
		return Receipt{}, err
	}
	if d.Tenant != tenant || d.Endpoint != endpoint || d.ProviderDeliveryID != providerID || d.Digest != wantDigest {
		return Receipt{}, ErrConflict
	}
	return Receipt{DeliveryID: d.ID, Sequence: raw.Sequence, Duplicate: true, AcceptedAt: d.AcceptedAt}, nil
}

// decode restores a delivery and checks that its subject and digest match.
func decode(subject string, header nats.Header, body []byte) (Delivery, error) {
	accepted, err := time.Parse(time.RFC3339Nano, header.Get(hdrAcceptedAt))
	if err != nil {
		return Delivery{}, fmt.Errorf("inbox: %w: accepted time", ErrInvalid)
	}
	d := Delivery{
		ID: header.Get(hdrID), Tenant: header.Get(hdrTenant), Endpoint: header.Get(hdrEndpoint),
		ProviderDeliveryID: header.Get(hdrProviderID), Resource: header.Get(hdrResource),
		Digest: header.Get(hdrDigest), AcceptedAt: accepted, Body: body,
	}
	if d.ID == "" || d.Tenant == "" || d.Endpoint == "" || d.ProviderDeliveryID == "" || d.Resource == "" ||
		subject != Subject(d.Tenant, d.Endpoint, d.ProviderDeliveryID) || d.Digest != digest(body) {
		return Delivery{}, fmt.Errorf("inbox: %w: retained delivery", ErrInvalid)
	}
	return d, nil
}

// Subject scopes a provider ID to one tenant and endpoint without exposing
// caller-controlled strings as NATS tokens.
func Subject(tenant, endpoint, providerID string) string {
	return endpointFilterPrefix(tenant, endpoint) + hashToken(providerID)
}

// endpointFilter matches every delivery for one endpoint.
func endpointFilter(tenant, endpoint string) string {
	return endpointFilterPrefix(tenant, endpoint) + ">"
}

func endpointFilterPrefix(tenant, endpoint string) string {
	return subjectRoot + "." + hashToken(tenant) + "." + hashToken(endpoint) + "."
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Connect dials NATS at url and attaches to the existing inbox. The returned
// close releases the connection.
func Connect(ctx context.Context, url, clientName string) (*Inbox, func(), error) {
	if url == "" {
		return nil, nil, errors.New("inbox: NATS URL is required")
	}
	conn, err := nats.Connect(url, nats.Name(clientName), nats.Timeout(5*time.Second))
	if err != nil {
		return nil, nil, fmt.Errorf("inbox: connect NATS: %w", err)
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	i, err := Attach(ctx, js)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("inbox: attach: %w", err)
	}
	return i, conn.Close, nil
}

// Usage reports how many messages and bytes the inbox holds.
func (i *Inbox) Usage(ctx context.Context) (messages, bytes uint64, err error) {
	info, err := i.stream.Info(ctx)
	if err != nil {
		return 0, 0, err
	}
	return info.State.Msgs, info.State.Bytes, nil
}
