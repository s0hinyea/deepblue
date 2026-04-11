package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	Client *mongo.Client
	DB     *mongo.Database

	EntitiesCollection *mongo.Collection
	ReportsCollection  *mongo.Collection
	KnowledgeCollection *mongo.Collection
)

func Connect() {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("MONGODB_URI is not set in the environment variables")
	}

	opts := options.Client().ApplyURI(uri)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	Client, err = mongo.Connect(opts)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	err = Client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	fmt.Println("Successfully connected and pinged MongoDB!")

	// Initialize our three spec databases
	DB = Client.Database("deepblue")
	EntitiesCollection = DB.Collection("water_entities")
	ReportsCollection = DB.Collection("community_reports")
	KnowledgeCollection = DB.Collection("knowledge_chunks")

	_, err = EntitiesCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "location", Value: "2dsphere"}},
	})
	if err != nil {
		log.Printf("Warning: could not create 2dsphere index: %v", err)
	}
}
