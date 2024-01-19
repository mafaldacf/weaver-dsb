package storage

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func MongoDBClient (ctx context.Context, address string, port int) (*mongo.Client, error) {
	uri := fmt.Sprintf("mongodb://%s:%d/?directConnection=true", address, port)
	clientOptions := options.Client().ApplyURI(uri)

	var err error
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("error connecting to mongodb: %s", err.Error())
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mongodb cannot be reached after connecting: %s", err.Error())
	}
	return client, nil
}
