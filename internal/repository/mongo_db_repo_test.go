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
	monitor := &event.CommandMonitor{Started: func(_ context.Context, e *event.CommandStartedEvent) {
		switch e.CommandName {
		case "find":
			lookedUp = e.Command.Lookup("filter").Document().Lookup("longURL").StringValue()
		case "insert":
			inserted = e.Command.Lookup("documents").Array().Index(0).Document().Lookup("longURL").StringValue()
		}
	}}
	repo := newMockMongoRepoWithMonitor(t, monitor,
		cursorResponse(),
		bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}},
	)
	repo.GetNextIDFunc = func(string) (int64, error) { return 12345, nil }

	shortCode, err := repo.SaveURL(context.Background(), destination)
	if err != nil {
		t.Fatalf("Failed to save URL: %v", err)
	}
	if expected := utils.Encode(12345); shortCode != expected {
		t.Errorf("Expected short code %s, got %s", expected, shortCode)
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

func TestIncrementAccessCount(t *testing.T) {
	repo := newMockMongoRepo(t, bson.D{
		{Key: "ok", Value: 1},
		{Key: "n", Value: 1},
		{Key: "nModified", Value: 1},
	})
	if err := repo.IncrementAccessCount(context.Background(), 12345); err != nil {
		t.Fatalf("Failed to increment access count: %v", err)
	}
}
