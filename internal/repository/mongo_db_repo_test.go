package repository

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// SetupTestMongoRepo initializes a test MongoRepo instance using MongoDB's in-memory test framework.
func SetupTestMongoRepo(t *testing.T) (*MongoRepo, context.Context) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	ctx := context.TODO()

	repo := &MongoRepo{
		Client:     mt.Client,
		Collection: mt.Coll,
	}

	return repo, ctx
}

// TestSaveURL tests the SaveURL function for inserting a URL document.
func TestSaveURL(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test save URL", func(mt *mtest.T) {
		// Set up mock MongoDB responses
		// First response for the FindOne operation should return no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "url_shortener.urls", mtest.FirstBatch))
		// Second response for the InsertOne operation should be successful
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		repo := &MongoRepo{
			Client:     mt.Client,
			Collection: mt.Coll,
		}

		// Call SaveURL
		shortURL := "short123"
		longURL := "https://example.com"
		returnedShortURL, err := repo.SaveURL(context.TODO(), shortURL, longURL)
		if err != nil {
			t.Fatalf("Failed to save URL: %v", err)
		}

		// Assert the returned short URL is not empty
		if returnedShortURL == "" {
			t.Errorf("Expected a valid short URL, got an empty string")
		}
	})
}

// TestFindURL tests the FindURL function for retrieving a URL by its short URL.
func TestFindURL(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test find URL", func(mt *mtest.T) {
		// Set up mock MongoDB responses
		mt.AddMockResponses(mtest.CreateCursorResponse(1, "url_shortener.urls", mtest.FirstBatch, bson.D{
			{Key: "shortURL", Value: "short123"},
			{Key: "longURL", Value: "https://example.com"},
		}))

		repo := &MongoRepo{
			Client:     mt.Client,
			Collection: mt.Coll,
		}

		// Call FindURL
		urlDoc, err := repo.FindURL(context.TODO(), "short123")
		if err != nil {
			t.Fatalf("Failed to find URL: %v", err)
		}

		// Assert the long URL is correct
		if urlDoc.LongURL != "https://example.com" {
			t.Errorf("Expected long URL %s, got %s", "https://example.com", urlDoc.LongURL)
		}
	})
}

// TestFindByLongURL tests the FindByLongURL function for retrieving a URL by its long URL.
func TestFindByLongURL(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test find by long URL", func(mt *mtest.T) {
		// Set up mock MongoDB responses
		mt.AddMockResponses(mtest.CreateCursorResponse(1, "url_shortener.urls", mtest.FirstBatch, bson.D{
			{Key: "shortURL", Value: "short123"},
			{Key: "longURL", Value: "https://example.com"},
		}))

		repo := &MongoRepo{
			Client:     mt.Client,
			Collection: mt.Coll,
		}

		// Call FindByLongURL
		shortURL, err := repo.FindShortURLByLongURL(context.TODO(), "https://example.com")
		if err != nil {
			t.Fatalf("Failed to find URL by long URL: %v", err)
		}

		// Assert the short URL is correct
		if shortURL != "short123" {
			t.Errorf("Expected short URL %s, got %s", "short123", shortURL)
		}
	})
}

// TestIncrementAccessCount tests the IncrementAccessCount function for incrementing the access count.
func TestIncrementAccessCount(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test increment access count", func(mt *mtest.T) {
		// Set up mock MongoDB responses for the update
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		repo := &MongoRepo{
			Client:     mt.Client,
			Collection: mt.Coll,
		}

		// Call IncrementAccessCount
		err := repo.IncrementAccessCount(context.TODO(), "short123")
		if err != nil {
			t.Fatalf("Failed to increment access count: %v", err)
		}
	})
}
