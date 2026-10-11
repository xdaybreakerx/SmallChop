package repository

import (
	"context"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"gochop-it/internal/utils"
)

// MongoRepo struct holds the MongoDB client
type MongoRepo struct {
	Client        *mongo.Client
	Collection    *mongo.Collection
	GetNextIDFunc func(ctx context.Context, counterName string) (int64, error)
}

// URL struct represents a URL document in MongoDB
type URL struct {
	ID          int64     `bson:"_id,omitempty"`
	CreatedAt   time.Time `bson:"createdAt"`
	LongURL     string    `bson:"longURL"`
	AccessCount int       `bson:"accessCount,omitempty"` // Legacy values are retained but no longer updated.
}

// NewMongoRepo creates a new instance of MongoRepo and establishes the connection
func NewMongoRepo(ctx context.Context) (*MongoRepo, error) {
	// Fetch MongoDB credentials and URI from environment variables
	mongoURI := "mongodb://" + os.Getenv("MONGO_APP_USERNAME") + ":" +
		os.Getenv("MONGO_APP_PASSWORD") + "@mongo:27017/" + os.Getenv("MONGO_DB_NAME")

	// Set MongoDB connection options
	clientOptions := options.Client().ApplyURI(mongoURI)

	// Connect to MongoDB
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the MongoDB server to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Disconnect(closeCtx)
		return nil, databaseError(err)
	}

	// Initialize the collection
	collection := client.Database(os.Getenv("MONGO_DB_NAME")).Collection("urls")

	repo := &MongoRepo{
		Client:        client,
		Collection:    collection,
		GetNextIDFunc: nil,
	}

	if repo.GetNextIDFunc == nil {
		repo.GetNextIDFunc = repo.GetNextID
	}

	return repo, nil
}

// SaveURL saves a new URL document into the MongoDB collection or returns the existing short URL if the long URL already exists
func (repo *MongoRepo) SaveURL(ctx context.Context, longURL string) (string, error) {
	// Sanitize the URL
	sanitizedURL, err := utils.SanitizeURL(longURL)
	if err != nil {
		return "", err
	}

	// Check if the long URL already exists
	existingURL, err := repo.FindURLByLongURL(ctx, sanitizedURL)
	if err != nil {
		return "", err
	}
	if existingURL != nil {
		// Return existing short code
		shortCode := utils.Encode(existingURL.ID)
		return shortCode, nil
	}

	// Generate a new ID and encode it
	id, err := repo.GetNextIDFunc(ctx, "url_counter")
	if err != nil {
		return "", err
	}
	shortCode := utils.Encode(id)

	urlDoc := URL{
		ID:        id,
		CreatedAt: time.Now(),
		LongURL:   sanitizedURL,
	}

	// Insert the new URL document
	_, err = repo.Collection.InsertOne(ctx, urlDoc)
	if err != nil {
		return "", databaseError(err)
	}

	return shortCode, nil
}

// FindURL retrieves a URL document based on the short URL
func (repo *MongoRepo) FindURL(ctx context.Context, shortURL string) (URL, error) {
	var urlDoc URL
	err := repo.Collection.FindOne(ctx, bson.M{"shortURL": shortURL}).Decode(&urlDoc)
	if err != nil {
		return URL{}, databaseError(err)
	}

	return urlDoc, nil
}

// FindURLByLongURL checks if the long URL already exists in the database and returns the corresponding short URL if found
func (repo *MongoRepo) FindURLByLongURL(ctx context.Context, longURL string) (*URL, error) {
	var existingURL URL
	err := repo.Collection.FindOne(ctx, bson.M{"longURL": longURL}).Decode(&existingURL)
	if err == mongo.ErrNoDocuments {
		return nil, nil // URL does not exist
	} else if err != nil {
		return nil, databaseError(err)
	}
	return &existingURL, nil
}

// FindURLByID searches by id field
func (repo *MongoRepo) FindURLByID(ctx context.Context, id int64) (*URL, error) {
	var urlDoc URL
	err := repo.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&urlDoc)
	if err != nil {
		return nil, databaseError(err)
	}
	return &urlDoc, nil
}

// GetNextID is used for encoding based on ID, returns ID
func (repo *MongoRepo) GetNextID(ctx context.Context, counterName string) (int64, error) {
	counters := repo.Collection.Database().Collection("counters")
	filter := bson.M{"_id": counterName}
	update := bson.M{"$inc": bson.M{"seq": 1}}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var result struct {
		Seq int64 `bson:"seq"`
	}
	err := counters.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return 0, databaseError(err)
	}
	return result.Seq, nil
}

// Ping checks required storage without reading URL mappings.
func (repo *MongoRepo) Ping(ctx context.Context) error {
	return databaseError(repo.Client.Ping(ctx, nil))
}
