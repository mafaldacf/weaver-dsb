package services

import (
	"context"
	
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserTimelineService interface {
	WriteUserTimeline(ctx context.Context, reqID int64, postID int64, userID int64, timestamp int64) error
	// TODO: read user timeline!!
}

var _ weaver.NotRetriable = UserTimelineService.WriteUserTimeline

type userTimelineServiceOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
	RedisAddr   string `toml:"redis_address"`
	RedisPort   int    `toml:"redis_port"`
}

type userTimelineService struct {
	weaver.Implements[UserTimelineService]
	weaver.WithConfig[userTimelineServiceOptions]
	mongoClient *mongo.Client
	redisClient *redis.Client
}

type UserPost struct {
	PostID    int64
	Timestamp int64
}

type Timeline struct {
	UserID int64
	Posts  []UserPost
}

func (u *userTimelineService) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	var err error
	u.mongoClient, err = storage.MongoDBClient(ctx, u.Config().MongoDBAddr, u.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	u.redisClient = storage.RedisClient(u.Config().RedisAddr, u.Config().RedisPort)
	logger.Info("user timeline service running!", "mongodb_addr", u.Config().MongoDBAddr, "mongodb_port", u.Config().MongoDBPort)
	return nil
}

func (u *userTimelineService) WriteUserTimeline(ctx context.Context, reqID int64, postID int64, userID int64, timestamp int64) error {
	logger := u.Logger(ctx)
	logger.Info("entering WriteUserTimeline for userTimelineService service", "reqID", reqID, "postID", postID, "userID", userID, "timestamp", timestamp)

	collection := u.mongoClient.Database("user-timeline").Collection("user-timeline")

	var timeline Timeline
	filter := bson.D{{Key: "userID", Value: userID}}
	err := collection.FindOne(ctx, filter, nil).Decode(&timeline)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			logger.Error("error finding user timeline", "msg", err.Error())
			return err
		}
		timeline.UserID = userID
	}
	timeline.Posts = append(timeline.Posts, UserPost{
		PostID:    postID,
		Timestamp: timestamp,
	})

	// TODO UPDATE
	logger.Info("updating current user timeline")
	return nil
}
