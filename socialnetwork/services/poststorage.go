package services

import (
	"context"
	"fmt"

	"github.com/ServiceWeaver/weaver"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Post struct {
	Username string `bson:"username"`
	PostID   int64 	`bson:"postid"`
}

type PostStorage interface {
	StorePost(context.Context, string, int64) error
}

type postStorageOptions struct {
	MongoDBAddr 		string 	`toml:"mongodb_address"`
	MongoDBPort 		int 	`toml:"mongodb_port"`
	Region         		string 	`toml:"region"`
}

type postStorage struct {
	weaver.Implements[PostStorage]
	weaver.WithConfig[postStorageOptions]
	mongoClient *mongo.Client
}

func (p *postStorage) Init(ctx context.Context) error {
	logger := p.Logger(ctx)
	uri := fmt.Sprintf("mongodb://%s:%d/?directConnection=true", p.Config().MongoDBAddr, p.Config().MongoDBPort)
	clientOptions := options.Client().ApplyURI(uri)

	var err error
	p.mongoClient, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		logger.Error("error connecting to mongodb", "msg", err.Error())
		return err
	}
	err = p.mongoClient.Ping(ctx, nil)
	if err != nil {
		logger.Error("error validating to mongodb", "msg", err.Error())
		return err
	}

	logger.Info("PostStorage service running!", "region", p.Config().Region, "mongodb_addr", p.Config().MongoDBAddr, "mongodb_port", p.Config().MongoDBPort)
	return nil
}

func (p *postStorage) StorePost(ctx context.Context, username string, postid int64) error {
	logger := p.Logger(ctx)
	logger.Info("entering StorePost for PostStorage service", "postid", postid)

	trace.SpanFromContext(ctx).AddEvent("storing post",
		trace.WithAttributes(
			attribute.String("username", username),
			attribute.Int64("postid", postid),
		))

	db := p.mongoClient.Database("poststorage")
	collection := db.Collection("posts")
	r, err := collection.InsertOne(ctx, Post{
		Username: username,
		PostID:   postid,
	})
	if err != nil {
		logger.Error("error writing post", "msg", err.Error())
	}
	logger.Debug("inserted post", "objectid", r.InsertedID)
	return nil
}
