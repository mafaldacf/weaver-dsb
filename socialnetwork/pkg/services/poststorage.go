package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PostStorageService interface {
	StorePost(ctx context.Context, reqID int64, post model.Post) error
	ReadPost(ctx context.Context, reqID int64, postID int64) (model.Post, error)
	ReadPosts(ctx context.Context, reqID int64, postIDs []int64) ([]model.Post, error)
}

var _ weaver.NotRetriable = PostStorageService.StorePost

type postStorageServiceOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
	RedisAddr   string `toml:"redis_address"`
	RedisPort   int    `toml:"redis_port"`
	Region      string `toml:"region"`
}

type postStorageService struct {
	weaver.Implements[PostStorageService]
	weaver.WithConfig[postStorageServiceOptions]
	mongoClient *mongo.Client
	redisClient *redis.Client //FIXME: should be memcached
}

func (p *postStorageService) Init(ctx context.Context) error {
	logger := p.Logger(ctx)

	var err error
	p.mongoClient, err = storage.MongoDBClient(ctx, p.Config().MongoDBAddr, p.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	p.redisClient = storage.RedisClient(p.Config().RedisAddr, p.Config().RedisPort)

	logger.Info("post storage service running!", "region", p.Config().Region,
		"mongodb_addr", p.Config().MongoDBAddr, "mongodb_port", p.Config().MongoDBPort,
		"redis_addr", p.Config().RedisAddr, "redis_port", p.Config().RedisPort,
	)
	return nil
}

func (p *postStorageService) StorePost(ctx context.Context, reqID int64, post model.Post) error {
	logger := p.Logger(ctx)
	logger.Info("entering StorePost", "reqid", reqID, "post", post)

	poststorage_start_ms := time.Now().UnixMilli()

	collection := p.mongoClient.Database("post-storage").Collection("posts")
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

func (p *postStorageService) ReadPost(ctx context.Context, reqID int64, postID int64) (model.Post, error) {
	logger := p.Logger(ctx)
	logger.Info("entering ReadPost", "req_id", reqID, "post_id", postID)

	var post model.Post
	postIDStr := strconv.FormatInt(postID, 10)
	result, err := p.redisClient.Get(ctx, postIDStr).Bytes()

	if err != nil && err != redis.Nil {
		// error reading cache
		logger.Error("error reading post from mongodb", "msg", err.Error())
		return post, err
	}
	if err == nil {
		// post found in cache
		err := json.Unmarshal(result, &post)
		if err != nil {
			logger.Error("error parsing post from cache result", "msg", err.Error())
			return post, err
		}
	} else {
		// post does not exist in cache
		// so we get it from db
		collection := p.mongoClient.Database("post-storage").Collection("posts")
		filter := bson.D{
			{Key: "PostID", Value: postID},
		}
		result := collection.FindOne(ctx, filter)
		if result.Err() != nil {
			return post, err
		}
		err = result.Decode(&post)
		if err != nil {
			errMsg := fmt.Sprintf("post_id: %s not found in mongodb", postIDStr)
			logger.Warn(errMsg)
			return post, fmt.Errorf(errMsg)
		}
	}
	return post, nil
}

func (p *postStorageService) ReadPosts(ctx context.Context, reqID int64, postIDs []int64) ([]model.Post, error) {
	logger := p.Logger(ctx)
	logger.Info("entering ReadPosts", "req_id", reqID, "post_ids", postIDs)

	if len(postIDs) == 0 { // FIXME: remove this when using memcached instead of redis
		return []model.Post{}, nil
	}

	uniquePostIDs := make(map[int64]bool)
	for _, pid := range postIDs {
		uniquePostIDs[pid] = true
	}

	var keys []string
	for _, pid := range postIDs {
		keys = append(keys, strconv.FormatInt(pid , 10))
	}
	values := make([]model.Post, len(keys))
	var retvals []interface{}
	for idx := range values {
		retvals = append(retvals, &values[idx])
	}
	result, err := p.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		logger.Error("error reading keys from redis", "msg", err.Error())
		return nil, err
	}

	for i, data := range result {
		if data != nil {
			err := json.Unmarshal([]byte(data.(string)), retvals[i])
			if err != nil {
				logger.Error("error parsing ids from redis result", "msg", err.Error())
				return nil, err
			}
		}
	}
	for _, post := range values {
		delete(uniquePostIDs, post.PostID)
	}
	if len(uniquePostIDs) != 0 {
		var newPosts []model.Post
		collection := p.mongoClient.Database("post-storage").Collection("posts")

		queryPostIDArray := bson.A{}
		for id := range uniquePostIDs {
			queryPostIDArray = append(queryPostIDArray, id)
		}
		filter := bson.D{
			{Key: "post_id", Value: bson.D{
				{Key: "$in", Value: queryPostIDArray},
			}},
		}
		cur, err := collection.Find(ctx, filter)
		if err != nil {
			logger.Error("error reading posts from mongodb", "msg", err.Error())
			return nil, err
		}

		exists := cur.TryNext(ctx)
		if exists {
			err = cur.All(ctx, &newPosts)
			if err != nil {
				logger.Error("error parsing new posts from mongodb", "msg", err.Error())
				return nil, err
			}
			values = append(values, newPosts...)

			var wg sync.WaitGroup
			for _, newPost := range newPosts {
				wg.Add(1)

				go func(newPost model.Post) error {
					defer wg.Done()
					postJson, err := json.Marshal(newPost)
					if err != nil {
						logger.Error("error converting post to json", "post", newPost)
						return err
					}
					p.redisClient.Set(ctx, strconv.FormatInt(newPost.PostID, 10), postJson, 0)
					return nil
				}(newPost)
			}
			wg.Wait()
		}
	}
	return values, nil
}
