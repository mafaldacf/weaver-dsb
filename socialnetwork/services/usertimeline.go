package services

import (
	"context"
	"socialnetwork/services/utils"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserTimeline interface {
	WriteUserTimeline(ctx context.Context, reqID int64, postID int64, userID int64, timestamp int64) error
}

var _ weaver.NotRetriable = UserTimeline.WriteUserTimeline

type userTimelineOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
	RedisAddr   string `toml:"redis_address"`
	RedisPort   int    `toml:"redis_port"`
	Region      string `toml:"region"`
}

type userTimeline struct {
	weaver.Implements[UserTimeline]
	weaver.WithConfig[userTimelineOptions]
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

func (u *userTimeline) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	logger.Debug("initializing user timeline service...")

	var err error
	u.mongoClient, err = utils.MongoDBClient(ctx, u.Config().MongoDBAddr, u.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	u.redisClient = utils.RedisClient(u.Config().RedisAddr, u.Config().RedisPort)

	logger.Info("user timeline service running!", "region", u.Config().Region, "mongodb_addr", u.Config().MongoDBAddr, "mongodb_port", u.Config().MongoDBPort)
	return nil
}

func (u *userTimeline) WriteUserTimeline(ctx context.Context, reqID int64, postID int64, userID int64, timestamp int64) error {
	logger := u.Logger(ctx)
	logger.Info("entering WriteUserTimeline for userTimeline service", "reqID", reqID, "postID", postID, "userID", userID, "timestamp", timestamp)

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

	return nil
}
