package services

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserTimelineService interface {
	ReadUserTimeline(ctx context.Context, reqID int64, userID int64, start int64, stop int64) ([]model.Post, error)
	WriteUserTimeline(ctx context.Context, reqID int64, postID int64, userID int64, timestamp int64) error
}

type userTimelineServiceOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
	RedisAddr   string `toml:"redis_address"`
	RedisPort   int    `toml:"redis_port"`
}

type userTimelineService struct {
	weaver.Implements[UserTimelineService]
	weaver.WithConfig[userTimelineServiceOptions]
	postStorageService 	weaver.Ref[PostStorageService]
	mongoClient 		*mongo.Client
	redisClient 		*redis.Client
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

// FIXME: panic: runtime error: hash of unhashable type bsonrw.TransitionError [recovered]
/* func (u *userTimelineService) WriteUserTimeline(ctx context.Context, reqID int64, postID int64, userID int64, timestamp int64) error {
	logger := u.Logger(ctx)
	logger.Debug("entering WriteUserTimeline", "reqID", reqID, "postID", postID, "userID", userID, "timestamp", timestamp)

	collection := u.mongoClient.Database("user-timeline").Collection("user-timeline")

	userIDStr := strconv.FormatInt(userID, 10)
	postIDstr := strconv.FormatInt(postID, 10)
	timestampstr := strconv.FormatInt(timestamp, 10)
	filter := bson.M{"user_id": userIDStr}
	update := fmt.Sprintf(`{"$push": {"posts": {"$each": [{"post_id": %s, "timestamp": %s}], "$position": 0}}}`, postIDstr, timestampstr)
	
	_, err := collection.UpdateMany(ctx, filter, update, &options.UpdateOptions {
		// a new document is inserted if filter does not match any doc
		Upsert: utils.BoolToPtr(false),
	})
	if err != nil {
		logger.Error("error updating user timeline", "msg", err.Error())
		return err
	}
	return nil
} */

func (u *userTimelineService) WriteUserTimeline(ctx context.Context, reqID int64, postID int64, userID int64, timestamp int64) error {
	logger := u.Logger(ctx)
	logger.Debug("entering WriteUserTimeline", "req_id", reqID, "post_id", postID, "user_id", userID, "timestamp", timestamp)
	
	collection := u.mongoClient.Database("user-timeline").Collection("user-timeline")
	
	userIDStr := strconv.FormatInt(userID, 10)
	filter := bson.M{"user_id": userIDStr}
	cur, err := collection.Find(ctx, filter)
	if err != nil {
		logger.Error("error reading posts from mongodb", "msg", err.Error())
		return err
	}
	var timelines []model.Timeline
	cur.Decode(&timelines) // ignore errors

	if len(timelines) == 0 {
		timeline := model.Timeline{UserID: userID, Posts: []model.TimelinePostInfo{{
				PostID: postID, 
				Timestamp: timestamp,
		}}}
		_, err := collection.InsertOne(ctx, timeline)
		if err != nil {
			logger.Error("failed to insert user timeline")
			return err
		}
	} else {
		postIDstr := strconv.FormatInt(postID, 10)
		timestampstr := strconv.FormatInt(timestamp, 10)
		update := fmt.Sprintf(`{"$push": {"Posts": {"$each": [{"PostID": %s, "Timestamp": %s}], "$position": 0}}}`, postIDstr, timestampstr)
		_, err := collection.UpdateMany(ctx, filter, update)
		if err != nil {
			logger.Error("failed to insert user timeline")
			return err
		}
	}
	return u.redisClient.ZAddNX(ctx, userIDStr, redis.Z{
		Member: postID,
		Score: float64(timestamp),
	}).Err()
}

// readTimeline is an helper function for reading timeline from redis with the same behavior as in the home timeline service
func (u *userTimelineService) readTimeline(ctx context.Context, userIDStr string, start int64, stop int64) ([]int64, error) {
	logger := u.Logger(ctx)
	
	result, err := u.redisClient.ZRevRange(ctx, userIDStr, start, stop-1).Result()
	if err != nil {
		logger.Error("error reading user timeline from redis")
		return nil, err
	}
	
	var postIDs []int64
	for _, result := range result {
		id, err := strconv.ParseInt(result, 10, 64)
		if err != nil {
			logger.Error("error parsing post id from redis result")
			return nil, err
		}
		postIDs = append(postIDs, id)
	}
	return postIDs, nil
}

func (u *userTimelineService) ReadUserTimeline(ctx context.Context, reqID int64, userID int64, start int64, stop int64) ([]model.Post, error) {
	logger := u.Logger(ctx)
	logger.Debug("entering ReadUserTimeline", "req_id", reqID, "user_id", userID, "start", start, "stop", stop)
	if stop <= start || start < 0 {
		return nil, nil
	}

	userIDStr := strconv.FormatInt(userID, 10)
	postIDs, err := u.readTimeline(ctx, userIDStr, start, stop)
	if err != nil {
		return nil, err
	}

	mongoStart := start + int64(len(postIDs))
	var postsToCache []redis.Z
	var timelinePosts []model.TimelinePostInfo
	if mongoStart < stop {
		collection := u.mongoClient.Database("user-timeline").Collection("user-timeline")
		query := fmt.Sprintf(`{"UserID": %[1]d}`, userID)
		opts := options.FindOptions{
			Projection: fmt.Sprintf(`{"projection": {"posts": {"$slice": [0, %[1]d]}}}`, stop),
		}
		cur, err := collection.Find(ctx, query, &opts)
		if err != nil {
			logger.Error("error reading posts from mongodb", "msg", err.Error())
			return nil, err
		}
		cur.Decode(&timelinePosts) // ignore errors
		for _, timelinePost := range timelinePosts {
			postIDs = append(postIDs, timelinePost.PostID)
			postsToCache = append(postsToCache, redis.Z {
				Member: timelinePost.PostID,
				Score: float64(timelinePost.Timestamp),
			})
		}
	}

	var postsErr, redisErr error
	var postsWg, redisWg sync.WaitGroup
	postsWg.Add(1)
	var posts []model.Post
	go func() {
		defer postsWg.Done()
		posts, postsErr = u.postStorageService.Get().ReadPosts(ctx, reqID, postIDs)
	}()

	if len(timelinePosts) > 0 {
		redisWg.Add(1)
		go func() {
			defer redisWg.Done()
			_, redisErr = u.redisClient.ZAddNX(ctx, userIDStr, postsToCache...).Result()
		}()
	}

	postsWg.Wait()
	if postsErr != nil {
		logger.Error("error fetching posts from post storage service", "msg", err.Error())
		return nil, err
	}
	redisWg.Wait()
	if redisErr != nil {
		logger.Error("error updating redis with new posts", "msg", err.Error())
		return nil, err
	}

	return posts, nil
}
