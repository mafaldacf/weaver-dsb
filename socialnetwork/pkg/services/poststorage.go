package services

import (
	"context"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PostStorageService interface {
	StorePost(ctx context.Context, reqID int64, post model.Post) error
}

var _ weaver.NotRetriable = PostStorageService.StorePost

type postStorageServiceOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
	Region      string `toml:"region"`
}

type postStorageService struct {
	weaver.Implements[PostStorageService]
	weaver.WithConfig[postStorageServiceOptions]
	mongoClient *mongo.Client
}

func (p *postStorageService) Init(ctx context.Context) error {
	logger := p.Logger(ctx)
	logger.Debug("initializing post storage service...")

	var err error
	p.mongoClient, err = storage.MongoDBClient(ctx, p.Config().MongoDBAddr, p.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	logger.Info("post storage service running!", "region", p.Config().Region, "mongodb_addr", p.Config().MongoDBAddr, "mongodb_port", p.Config().MongoDBPort)
	return nil
}

func (p *postStorageService) StorePost(ctx context.Context, reqID int64, post model.Post) error {
	logger := p.Logger(ctx)
	logger.Info("entering StorePost for PostStorage service", "reqid", reqID, "post", post)

	poststorage_start_ms := time.Now().UnixMilli()

	db := p.mongoClient.Database("poststorage")
	collection := db.Collection("posts")
	r, err := collection.InsertOne(ctx, post)
	if err != nil {
		logger.Error("error writing post", "msg", err.Error())
	}
	logger.Debug("inserted post", "objectid", r.InsertedID)

	trace.SpanFromContext(ctx).AddEvent("reading post in mongodb",
		trace.WithAttributes(
			attribute.Int64("poststorage_start_ms", poststorage_start_ms),
			attribute.Int64("poststorage_end_ms", time.Now().UnixMilli()),
		))

	return nil
}
