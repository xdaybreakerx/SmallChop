package repository

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// SetupTestMongoRepo initializes a test MongoRepo instance using MongoDB's in-memory test framework.
func SetupTestMongoRepo(t *testing.T) (*MongoRepo, context.Context) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	ctx := context.TODO()

	repo := &MongoRepo{
		client:     mt.Client,
		collection: mt.Coll,
	}

	return repo, ctx
}

// TestSaveURL tests the SaveURL function for inserting a URL document.
func TestSaveURL(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test save URL", func(mt *mtest.T) {
		// Set up mock MongoDB responses
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		repo := &MongoRepo{
			client:     mt.Client,
			collection: mt.Coll,
		}

		// Call SaveURL
		shortURL := "short123"
		longURL := "https://example.com"
		id, err := repo.SaveURL(context.TODO(), shortURL, longURL)
		if err != nil {
			t.Fatalf("Failed to save URL: %v", err)
		}

		// Assert the returned ID is not empty
		if id == primitive.NilObjectID {
			t.Errorf("Expected a valid ObjectID, got NilObjectID")
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
			client:     mt.Client,
			collection: mt.Coll,
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

// TestIncrementAccessCount tests the IncrementAccessCount function for incrementing the access count.
func TestIncrementAccessCount(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test increment access count", func(mt *mtest.T) {
		// Set up mock MongoDB responses for the update
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		repo := &MongoRepo{
			client:     mt.Client,
			collection: mt.Coll,
		}

		// Call IncrementAccessCount
		err := repo.IncrementAccessCount(context.TODO(), "short123")
		if err != nil {
			t.Fatalf("Failed to increment access count: %v", err)
		}
	})
}
