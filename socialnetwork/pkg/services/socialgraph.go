package services

import (
	"context"
	"strconv"
	"sync"
	"time"

	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type SocialGraphService interface {
	GetFollowers(ctx context.Context, reqID int64, userID int64) ([]int64, error)
	GetFollowees(ctx context.Context, reqID int64, userID int64) ([]int64, error)
	Follow(ctx context.Context, reqID int64, userID int64, followeeID int64) error
	Unfollow(ctx context.Context, reqID int64, userID int64, followeeID int64) error
	FollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error
	UnfollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error
	InsertUser(ctx context.Context, reqID int64, userID int64) error
}

type socialGraphService struct {
	weaver.Implements[SocialGraphService]
	weaver.WithConfig[socialGraphServiceOptions]
	mongoClient   		*mongo.Client
	redisClient   		*redis.Client
}

type socialGraphServiceOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
	RedisAddr   string `toml:"redis_address"`
	RedisPort   int    `toml:"redis_port"`
}

func (s *socialGraphService) Init(ctx context.Context) error {
	logger := s.Logger(ctx)
	var err error
	s.mongoClient, err = storage.MongoDBClient(ctx, s.Config().MongoDBAddr, s.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	s.redisClient = storage.RedisClient(s.Config().RedisAddr, s.Config().RedisPort)

	logger.Info("social graph service running!", 
		"mongodb_addr", s.Config().MongoDBAddr, "mongodb_port", s.Config().MongoDBPort, 
		"redis_addr", s.Config().RedisAddr, "redis_port", s.Config().RedisPort,
	)
	return nil
}

func (s *socialGraphService) GetFollowers(ctx context.Context, reqID int64, userID int64) ([]int64, error) {
	//TODO
	return []int64{0}, nil
}


func (s *socialGraphService) GetFollowees(ctx context.Context, reqID int64, userID int64) ([]int64, error) {
	//TODO
	return []int64{0}, nil
}

func (s *socialGraphService) Follow(ctx context.Context, reqID int64, userID int64, followeeID int64) error {
	logger := s.Logger(ctx)
	logger.Debug("entering Follow", "req_id", reqID, "user_id", userID, "followeeID", followeeID)
	timestamp := time.Now()
	userIDstr := strconv.FormatInt(userID, 10)
	followeeIDstr := strconv.FormatInt(followeeID, 10)
	var mongoUpdateFollowerErr, mongoUpdateFolloweeErr, redisUpdateErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		// Update follower->followee edges
		defer wg.Done()
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		searchNotExist := `{"$and": [{"user_id":` + userIDstr + `}, {"followees": {"$not": {"$elemMatch": {"user_id": ` + followeeIDstr + `}}}}]}`
		update := `{"$push": {"followees": {"user_id": ` + userIDstr + `,"Timestamp": "` + timestamp.String() + `"}}}`
		_, mongoUpdateFollowerErr = collection.UpdateOne(ctx, searchNotExist, update)
		if mongoUpdateFollowerErr != nil {
			logger.Error("error updating followees in mongodb")
		}
	}()
	go func() {
		// Update followee->follower edges
		defer wg.Done()
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		searchNotExist := `{"$and": [{"user_id":` + followeeIDstr + `}, {"followers": {"$not": {"$elemMatch": {"user_id": ` + followeeIDstr + `}}}}]}`
		update := `{"$push": {"followers": {"user_id": ` + followeeIDstr + `,"Timestamp": "` + timestamp.String() + `"}}}`
		_, mongoUpdateFolloweeErr = collection.UpdateOne(ctx, searchNotExist, update)
		if mongoUpdateFolloweeErr != nil {
			logger.Error("error updating followers in mongodb")
		}
	}()
	go func() {
		defer wg.Done()
		_, redisUpdateErr = s.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			intCmdFollowees := pipe.ZCard(ctx, userIDstr + ":followees")
			intCmdFollowers := pipe.ZCard(ctx, followeeIDstr + ":followers")
			_, err := intCmdFollowees.Result() 
			if err != nil {
				logger.Error("error getting followees from redis")
				return err
			}
			pipe.ZAddNX(ctx, userIDstr + ":followees", redis.Z {
				Member: followeeID,
				Score: float64(timestamp.Unix()),
			})
			_, err = intCmdFollowers.Result() 
			if err != nil {
				logger.Error("error getting followers from redis")
				return err
			}
			pipe.ZAddNX(ctx, userIDstr + ":followers", redis.Z {
				Member: followeeID,
				Score: float64(timestamp.Unix()),
			})
			return nil
		})
		if redisUpdateErr != nil {
			logger.Error("error writing followees and followers to redis")
		}
	}()
	wg.Wait()
	if mongoUpdateFollowerErr != nil {
		return mongoUpdateFollowerErr
	}
	if mongoUpdateFolloweeErr != nil {
		return mongoUpdateFolloweeErr
	}
	return redisUpdateErr
}

func (s *socialGraphService) Unfollow(ctx context.Context, reqID int64, userID int64, followeeID int64) error {
	//TODO
	return nil
}

func (s *socialGraphService) FollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error {
	//TODO
	return nil
}

func (s *socialGraphService) UnfollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error {
	//TODO
	return nil
}

func (s *socialGraphService) InsertUser(ctx context.Context, reqID int64, userID int64) error {
	logger := s.Logger(ctx)
	logger.Debug("entering InsertUser", "req_id", reqID, "user_id", userID)
	collection := s.mongoClient.Database("social-graph").Collection("social-graph")
	doc := bson.D {
		{Key: "user_id", Value: userID},
		{Key: "followers", Value: bson.A{}},
		{Key: "followees", Value: bson.A{}},
	}
	_, err := collection.InsertOne(ctx, doc)
	return err
}

