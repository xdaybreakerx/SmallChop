package repository

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"gochop-it/internal/utils"
)

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
		mt.AddMockResponses(
			// Mock response for FindOne (no document found)
			mtest.CreateCursorResponse(0, "url_shortener.urls", mtest.FirstBatch),
			// Mock response for InsertOne (success)
			mtest.CreateSuccessResponse(),
		)

		repo := &MongoRepo{
			Client:     mt.Client,
			Collection: mt.Coll,
			GetNextIDFunc: func(counterName string) (int64, error) {
				return 12345, nil // Return fixed ID for testing
			},
		}

		// Call SaveURL
		longURL := "https://example.com"
		shortCode, err := repo.SaveURL(context.TODO(), longURL)
		if err != nil {
			t.Fatalf("Failed to save URL: %v", err)
		}

		// Assert the returned short code is correct
		expectedShortCode := utils.Encode(12345)
		if shortCode != expectedShortCode {
			t.Errorf("Expected short code %s, got %s", expectedShortCode, shortCode)
		}
	})
}

// TestFindURLByID tests the FindURLByID function for retrieving a URL by its ID.
func TestFindURLByID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test find URL by ID", func(mt *mtest.T) {
		// Prepare the expected URL document
		expectedURL := URL{
			ID:          12345,
			CreatedAt:   (time.Now()),
			LongURL:     "https://example.com",
			AccessCount: 0,
		}

		// Set up mock MongoDB responses
		mt.AddMockResponses(mtest.CreateCursorResponse(1, "url_shortener.urls", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: expectedURL.ID},
			{Key: "createdAt", Value: expectedURL.CreatedAt},
			{Key: "longURL", Value: expectedURL.LongURL},
			{Key: "accessCount", Value: expectedURL.AccessCount},
		}))

		repo := &MongoRepo{
			Client:     mt.Client,
			Collection: mt.Coll,
		}

		// Call FindURLByID
		urlDoc, err := repo.FindURLByID(context.TODO(), expectedURL.ID)
		if err != nil {
			t.Fatalf("Failed to find URL: %v", err)
		}

		// Assert the long URL is correct
		if urlDoc.LongURL != expectedURL.LongURL {
			t.Errorf("Expected long URL %s, got %s", expectedURL.LongURL, urlDoc.LongURL)
		}
	})
}

// TestFindURLByLongURL tests the FindURLByLongURL function for retrieving a URL by its long URL.
func TestFindURLByLongURL(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test find URL by long URL", func(mt *mtest.T) {
		// Prepare the expected URL document
		expectedURL := URL{
			ID:          12345,
			CreatedAt:   (time.Now()),
			LongURL:     "https://example.com",
			AccessCount: 0,
		}

		// Set up mock MongoDB responses
		mt.AddMockResponses(mtest.CreateCursorResponse(1, "url_shortener.urls", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: expectedURL.ID},
			{Key: "createdAt", Value: expectedURL.CreatedAt},
			{Key: "longURL", Value: expectedURL.LongURL},
			{Key: "accessCount", Value: expectedURL.AccessCount},
		}))

		repo := &MongoRepo{
			Client:     mt.Client,
			Collection: mt.Coll,
		}

		// Call FindURLByLongURL
		urlDoc, err := repo.FindURLByLongURL(context.TODO(), expectedURL.LongURL)
		if err != nil {
			t.Fatalf("Failed to find URL by long URL: %v", err)
		}

		// Assert the ID is correct
		if urlDoc.ID != expectedURL.ID {
			t.Errorf("Expected ID %d, got %d", expectedURL.ID, urlDoc.ID)
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
		err := repo.IncrementAccessCount(context.TODO(), 12345)
		if err != nil {
			t.Fatalf("Failed to increment access count: %v", err)
		}
	})
}
