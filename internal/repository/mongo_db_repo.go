package repository

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoRepo struct holds the MongoDB client
type MongoRepo struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}

// URL struct represents a URL document in MongoDB
type URL struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt"`
	ShortURL    string             `bson:"shortURL"`
	LongURL     string             `bson:"longURL"`
	AccessCount int                `bson:"accessCount"`
}

// NewMongoRepo creates a new instance of MongoRepo and establishes the connection
func NewMongoRepo(ctx context.Context) (*MongoRepo, error) {
	// Fetch MongoDB credentials and URI from environment variables
	mongoURI := "mongodb://" + os.Getenv("MONGO_APP_USERNAME") + ":" +
		os.Getenv("MONGO_APP_PASSWORD") + "@mongo:27017/" + os.Getenv("MONGO_DB_NAME")

	// Set MongoDB connection options
	clientOptions := options.Client().ApplyURI(mongoURI)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the MongoDB server to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	log.Println("Connected to MongoDB with RBAC credentials!")

	// Initialize the collection
	collection := client.Database(os.Getenv("MONGO_DB_NAME")).Collection("urls")

	return &MongoRepo{
		Client:     client,
		Collection: collection,
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
	result, err := repo.Collection.InsertOne(ctx, urlDoc)
	if err != nil {
		return primitive.NilObjectID, err
	}

	log.Printf("Saved URL with short URL: %s, long URL: %s\n", shortURL, longURL)

	return result.InsertedID.(primitive.ObjectID), nil
}

// FindURL retrieves a URL document based on the short URL
func (repo *MongoRepo) FindURL(ctx context.Context, shortURL string) (URL, error) {
	var urlDoc URL
	err := repo.Collection.FindOne(ctx, primitive.M{"shortURL": shortURL}).Decode(&urlDoc)
	if err != nil {
		return URL{}, err
	}

	return urlDoc, nil
}

// Increment Access Count tracks how often URLs are accessed, for Redis caching of top frequent n accessed URLs
func (repo *MongoRepo) IncrementAccessCount(ctx context.Context, shortURL string) error {
	filter := bson.M{"shortURL": shortURL}
	update := bson.M{"$inc": bson.M{"accessCount": 1}} // Increment access count by 1
	_, err := repo.Collection.UpdateOne(ctx, filter, update)
	return err
}
