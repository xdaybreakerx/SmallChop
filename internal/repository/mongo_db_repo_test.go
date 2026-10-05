package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"

	"gochop-it/internal/utils"
)

// newMockMongoRepo replaces the mtest helper removed from the v2 public packages.
// The driver mock is confined to tests; production uses a normal MongoDB client.
func newMockMongoRepo(t *testing.T, responses ...bson.D) *MongoRepo {
	t.Helper()
	return newMockMongoRepoWithMonitor(t, nil, responses...)
}

func newMockMongoRepoWithMonitor(t *testing.T, monitor *event.CommandMonitor, responses ...bson.D) *MongoRepo {
	t.Helper()
	deployment := drivertest.NewMockDeployment(responses...)
	opts := options.Client().SetMonitor(monitor)
	opts.Deployment = deployment //nolint:staticcheck // The driver's test-only mock requires this internal option.
	client, err := mongo.Connect(opts)
	if err != nil {
		t.Fatalf("Connect mock MongoDB: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("Disconnect mock MongoDB: %v", err)
		}
	})
	return &MongoRepo{
		Client:     client,
		Collection: client.Database("url_shortener").Collection("urls"),
	}
}

func cursorResponse(documents ...bson.D) bson.D {
	batch := make(bson.A, len(documents))
	for i, document := range documents {
		batch[i] = document
	}
	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "cursor", Value: bson.D{
			{Key: "id", Value: int64(0)},
			{Key: "ns", Value: "url_shortener.urls"},
			{Key: "firstBatch", Value: batch},
		}},
	}
}

func TestSaveURL(t *testing.T) {
	destination := "https://example.com/a%2Fb?q=a%2Bb&x=2&x=1#installation"
	var lookedUp, inserted string
	var writes int
	var hasAccessCount bool
	monitor := &event.CommandMonitor{Started: func(_ context.Context, e *event.CommandStartedEvent) {
		switch e.CommandName {
		case "find":
			lookedUp = e.Command.Lookup("filter").Document().Lookup("longURL").StringValue()
		case "insert":
			writes++
			_, accessErr := e.Command.Lookup("documents").Array().Index(0).Document().LookupErr("accessCount")
			hasAccessCount = accessErr == nil
			inserted = e.Command.Lookup("documents").Array().Index(0).Document().Lookup("longURL").StringValue()
		}
	}}
	repo := newMockMongoRepoWithMonitor(t, monitor,
		cursorResponse(),
		bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}},
	)
	repo.GetNextIDFunc = func(context.Context, string) (int64, error) { return 12345, nil }

	shortCode, err := repo.SaveURL(context.Background(), destination)
	if err != nil {
		t.Fatalf("Failed to save URL: %v", err)
	}
	if expected := utils.Encode(12345); shortCode != expected {
		t.Errorf("Expected short code %s, got %s", expected, shortCode)
	}
	if writes != 1 || hasAccessCount {
		t.Fatalf("writes %d, retired count persisted %v", writes, hasAccessCount)
	}
	if lookedUp != destination || inserted != destination {
		t.Fatalf("destination changed: lookup %q, stored %q", lookedUp, inserted)
	}
}

func TestSaveURLValidationBeforeDatabase(t *testing.T) {
	for _, destination := range []string{"https:///path", "javascript:alert(1)", "https://user@example.com"} {
		if _, err := (&MongoRepo{}).SaveURL(context.Background(), destination); !errors.Is(err, utils.ErrInvalidURL) {
			t.Errorf("SaveURL(%q) = %v; want validation error", destination, err)
		}
	}
}

func TestFindURLByID(t *testing.T) {
	repo := newMockMongoRepo(t, cursorResponse(bson.D{
		{Key: "_id", Value: int64(12345)},
		{Key: "createdAt", Value: time.Now()},
		{Key: "longURL", Value: "https://example.com"},
		{Key: "accessCount", Value: 0},
	}))

	urlDoc, err := repo.FindURLByID(context.Background(), 12345)
	if err != nil {
		t.Fatalf("Failed to find URL: %v", err)
	}
	if urlDoc.LongURL != "https://example.com" {
		t.Errorf("Unexpected long URL: %s", urlDoc.LongURL)
	}
}

func TestFindURLByLongURL(t *testing.T) {
	repo := newMockMongoRepo(t, cursorResponse(bson.D{
		{Key: "_id", Value: int64(12345)},
		{Key: "createdAt", Value: time.Now()},
		{Key: "longURL", Value: "https://example.com"},
		{Key: "accessCount", Value: 0},
	}))

	urlDoc, err := repo.FindURLByLongURL(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Failed to find URL by long URL: %v", err)
	}
	if urlDoc.ID != 12345 {
		t.Errorf("Expected ID 12345, got %d", urlDoc.ID)
	}
}

func TestMongoErrorContracts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response bson.D
		want     error
	}{
		{"absent", cursorResponse(), ErrNotFound},
		{"server unavailable", bson.D{{Key: "ok", Value: 0}, {Key: "code", Value: 91}, {Key: "errmsg", Value: "shutdown"}}, ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockMongoRepo(t, tc.response)
			_, err := repo.FindURLByID(context.Background(), 1)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v; want %v", err, tc.want)
			}
		})
	}
}

func TestSaveURLPassesCancellationToIDAllocation(t *testing.T) {
	repo := newMockMongoRepo(t, cursorResponse())
	ctx, cancel := context.WithCancel(context.Background())
	called := false
	repo.GetNextIDFunc = func(received context.Context, name string) (int64, error) {
		called = true
		if received != ctx || name != "url_counter" {
			t.Fatal("ID allocation lost request context")
		}
		cancel()
		return 0, received.Err()
	}
	_, err := repo.SaveURL(ctx, "https://example.com/")
	if !called || !errors.Is(err, context.Canceled) {
		t.Fatalf("called %v, error %v", called, err)
	}
}

func TestGetNextIDHonorsCancellation(t *testing.T) {
	repo := newMockMongoRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := repo.GetNextID(ctx, "url_counter")
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error %v", err)
	}
}

func TestStoredDocumentErrorsAreInternal(t *testing.T) {
	repo := newMockMongoRepo(t, cursorResponse(bson.D{{Key: "_id", Value: "not an integer"}}))
	_, err := repo.FindURLByID(context.Background(), 1)
	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("decode error classification: %v", err)
	}
}

func TestSaveURLServerFailure(t *testing.T) {
	repo := newMockMongoRepo(t, bson.D{{Key: "ok", Value: 0}, {Key: "code", Value: 91}, {Key: "errmsg", Value: "shutdown"}})
	if _, err := repo.SaveURL(context.Background(), "https://example.com/"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("save error %v", err)
	}
}

func TestPermanentServerErrorIsInternal(t *testing.T) {
	repo := newMockMongoRepo(t, bson.D{{Key: "ok", Value: 0}, {Key: "code", Value: 2}, {Key: "errmsg", Value: "bad command"}})
	_, err := repo.FindURLByID(context.Background(), 1)
	if err == nil || errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) {
		t.Fatalf("permanent error classification: %v", err)
	}
}
