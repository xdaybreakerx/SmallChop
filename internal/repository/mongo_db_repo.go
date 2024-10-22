package repository

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoRepo struct holds the MongoDB client
type MongoRepo struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// URL struct represents a URL document in MongoDB
type URL struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	CreatedAt time.Time          `bson:"createdAt"`
	ShortURL  string             `bson:"shortURL"`
	LongURL   string             `bson:"longURL"`
}

// NewMongoRepo creates a new instance of MongoRepo and establishes the connection
func NewMongoRepo(ctx context.Context) (*MongoRepo, error) {
	// Set MongoDB connection options
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017/")

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the MongoDB server to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	log.Println("Connected to MongoDB!")

	// Initialize the collection
	collection := client.Database("url_shortener").Collection("urls")

	return &MongoRepo{
		client:     client,
		collection: collection,
	}, nil
}

// SaveURL saves a new URL document into the MongoDB collection
func (repo *MongoRepo) SaveURL(ctx context.Context, shortURL, longURL string) (primitive.ObjectID, error) {
	urlDoc := URL{
		ID:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		ShortURL:  shortURL,
		LongURL:   longURL,
	}

	// Insert the document into the collection
	result, err := repo.collection.InsertOne(ctx, urlDoc)
	if err != nil {
		return primitive.NilObjectID, err
	}

	log.Printf("Saved URL with short URL: %s, long URL: %s\n", shortURL, longURL)

	return result.InsertedID.(primitive.ObjectID), nil
}

// FindURL retrieves a URL document based on the short URL
func (repo *MongoRepo) FindURL(ctx context.Context, shortURL string) (URL, error) {
	var urlDoc URL
	err := repo.collection.FindOne(ctx, primitive.M{"shortURL": shortURL}).Decode(&urlDoc)
	if err != nil {
		return URL{}, err
	}

	return urlDoc, nil
}
