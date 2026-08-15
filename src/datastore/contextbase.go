package datastore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// ConnectMongo creates one shared MongoDB client for the application. The
// caller owns the client and must disconnect it during graceful shutdown.
func ConnectMongo(ctx context.Context, uri, databaseName string) (*mongo.Client, *mongo.Database, error) {
	if strings.TrimSpace(uri) == "" || strings.TrimSpace(databaseName) == "" {
		return nil, nil, fmt.Errorf("MongoDB URI and database name are required")
	}
	clientOptions := options.Client().
		ApplyURI(uri).
		SetAppName("quizbattle").
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("create MongoDB client: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("connect to MongoDB: %w", err)
	}
	return client, client.Database(databaseName), nil
}
