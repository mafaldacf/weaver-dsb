package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PostStorageService interface {
	StorePost(ctx context.Context, reqID int64, post model.Post) error
	ReadPosts(ctx context.Context, reqID int64, postIDs []int64) ([]model.Post, error)
}

var _ weaver.NotRetriable = PostStorageService.StorePost

type postStorageServiceOptions struct {
	MongoDBAddr string 		`toml:"mongodb_address"`
	MongoDBPort int    		`toml:"mongodb_port"`
	RedisAddr   string   	`toml:"redis_address"`
	RedisPort  	int     	`toml:"redis_port"`
	Region      string 		`toml:"region"`
}

type postStorageService struct {
	weaver.Implements[PostStorageService]
	weaver.WithConfig[postStorageServiceOptions]
	mongoClient *mongo.Client
	redisClient *redis.Client
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

func (p *postStorageService) ReadPosts(ctx context.Context, reqID int64, postIDs []int64) ([]model.Post, error) {
	logger := p.Logger(ctx)
	
	unique_post_ids := make(map[int64]bool)
	for _, pid := range postIDs {
		unique_post_ids[pid] = true
	}

	var keys []string
	for _, pid := range postIDs {
		keys = append(keys, strconv.FormatInt(pid, 10))
	}
	values := make([]model.Post, len(keys))
	var retvals []interface{}
	for idx := range values {
		retvals = append(retvals, &values[idx])
	}

	sliceCmd := p.redisClient.MGet(ctx, keys...)
	result, err := sliceCmd.Result()
	if err != nil {
		logger.Error("error reading keys from redis", "msg", err.Error())
		return nil, err
	}
	for i, data := range result {
		err := json.Unmarshal([]byte(data.(string)), retvals[i])
		if err != nil {
			logger.Error("error parsing ids from redis result", "msg", err.Error())
			return nil, err
		}
	}
	for _, post := range values {
		delete(unique_post_ids, post.PostID)
	}
	if len(unique_post_ids) != 0 {
		var new_posts []model.Post
		var unique_pids []int64
		for k := range unique_post_ids {
			unique_pids = append(unique_pids, k)
		}
		collection := p.mongoClient.Database("post").Collection("post")
		delim := ","
		filter := `{"PostID": {"$in": ` + strings.Join(strings.Fields(fmt.Sprint(unique_pids)), delim) + `}}`
		cur, err := collection.Find(ctx, filter)
		if err != nil {
			logger.Error("error reading posts from mongodb", "msg", err.Error())
			return []model.Post{}, err
		}
		err = cur.Decode(&new_posts)
		if err != nil {
			logger.Error("error parsing posts from mongodb result", "msg", err.Error())
			return []model.Post{}, err
		}
		values = append(values, new_posts...)

		var wg sync.WaitGroup
		for _, newPost := range new_posts {
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
	return values, nil
}

