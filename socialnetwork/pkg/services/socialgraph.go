package services

import (
	"context"
	"strconv"
	"sync"
	"time"

	"socialnetwork/pkg/storage"
	"socialnetwork/pkg/utils"

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
	userService weaver.Ref[UserService]
	mongoClient *mongo.Client
	redisClient *redis.Client
}

type socialGraphServiceOptions struct {
	MongoDBAddr map[string]string 	`toml:"mongodb_address"`
	RedisAddr   map[string]string 	`toml:"redis_address"`
	MongoDBPort map[string]int    	`toml:"mongodb_port"`
	RedisPort   map[string]int    	`toml:"redis_port"`
	Region 	 	string
}

type FollowerInfo struct {
	FollowerID int64 `bson:"follower_id"`
	Timestamp  int64 `bson:"timestamp"`
}

type FolloweeInfo struct {
	FolloweeID int64 `bson:"followee_id"`
	Timestamp  int64 `bson:"timestamp"`
}

type UserInfo struct {
	UserID    int64          `bson:"user_id"`
	Followers []FollowerInfo `bson:"followers"`
	Followees []FolloweeInfo `bson:"followees"`
}

func (s *socialGraphService) Init(ctx context.Context) error {
	logger := s.Logger(ctx)

	region, err := utils.Region()
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	s.Config().Region = region

	s.mongoClient, err = storage.MongoDBClient(ctx, s.Config().MongoDBAddr[region], s.Config().MongoDBPort[region])
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	s.redisClient = storage.RedisClient(s.Config().RedisAddr[region], s.Config().RedisPort[region])

	logger.Info("social graph service running!", "region", s.Config().Region,
		"mongodb_addr", s.Config().MongoDBAddr[region], "mongodb_port", s.Config().MongoDBPort[region],
		"redis_addr", s.Config().RedisAddr[region], "redis_port", s.Config().RedisPort[region],
	)
	return nil
}

func (s *socialGraphService) GetFollowers(ctx context.Context, reqID int64, userID int64) ([]int64, error) {
	logger := s.Logger(ctx)
	logger.Debug("entering GetFollowers", "req_id", reqID, "user_id", userID)

	var followerIDs []int64
	userIDStr := strconv.FormatInt(userID, 10)
	numFollowers, err := s.redisClient.ZCard(ctx, userIDStr+":followers").Result()
	if err != nil {
		logger.Error("error reading number of followers from cache", "msg", err.Error())
	}
	if numFollowers > 0 {
		// followers are cached in redis so we retrieve them
		result, err := s.redisClient.ZRange(ctx, userIDStr+":followers", 0, -1).Result()
		if err != nil {
			logger.Error("error reading followers from cache", "msg", err.Error())
			return nil, err
		}
		for _, r := range result {
			followerID, err := strconv.ParseInt(r, 10, 64)
			if err != nil {
				logger.Error("error parsing follower id from redis to int64", "msg", err.Error())
				return nil, err
			}
			followerIDs = append(followerIDs, followerID)
		}
		return followerIDs, nil
	} else {
		// did not find followers in redis
		// look up in mongodb and update redis
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		filter := bson.D{
			{Key: "user_id", Value: userIDStr},
		}
		var userInfo UserInfo
		err := collection.FindOne(ctx, filter).Decode(&userInfo)
		if err != nil {
			if err != mongo.ErrNoDocuments {
				logger.Error("error reading followers from mongodb", "msg", err.Error())
				return nil, err
			}
			// return empty array of ids
			return followerIDs, nil
		}
		// add ids to return value
		for _, f := range userInfo.Followers {
			followerIDs = append(followerIDs, f.FollowerID)
		}
		// update redis
		_, err = s.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			for _, f := range userInfo.Followers {
				err = pipe.ZAddNX(ctx, userIDStr+":followers", redis.Z{
					Member: f.FollowerID,
					Score:  float64(f.Timestamp),
				}).Err()
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			logger.Error("error getting updating redis with followers from mongodb")
			return nil, err
		}
	}
	return followerIDs, nil
}

// GetFollowees attempts to get the ids from redis if cached
// Otherwise, it gets the followees from mongodb and updates redis with the ids
func (s *socialGraphService) GetFollowees(ctx context.Context, reqID int64, userID int64) ([]int64, error) {
	logger := s.Logger(ctx)
	logger.Debug("entering GetFollowees", "req_id", reqID, "user_id", userID)

	var followerIDs []int64
	var followerInfos []FollowerInfo
	userIDStr := strconv.FormatInt(userID, 10)
	numFollowees, err := s.redisClient.ZCard(ctx, userIDStr+":followees").Result()
	if err != nil {
		logger.Error("error reading number of followees from redis", "msg", err.Error())
	}
	if numFollowees > 0 {
		// followees are cached in redis so we retrieve them
		result, err := s.redisClient.ZRange(ctx, userIDStr+":followees", 0, -1).Result()
		if err != nil {
			logger.Error("error reading followees from redis", "msg", err.Error())
			return nil, err
		}
		for _, r := range result {
			followerID, err := strconv.ParseInt(r, 10, 64)
			if err != nil {
				logger.Error("error parsing follower id from redis to int64", "msg", err.Error())
				return nil, err
			}
			followerIDs = append(followerIDs, followerID)
		}
		return followerIDs, nil
	} else {
		// did not find followees in redis
		// look up in mongodb and update redis
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		filter := bson.D{
			{Key: "user_id", Value: userIDStr},
		}
		var userInfo UserInfo
		err := collection.FindOne(ctx, filter).Decode(&userInfo)
		if err != nil {
			logger.Error("error reading followees from mongodb", "msg", err.Error())
			return nil, err
		}
		// add ids to return value
		for _, f := range followerInfos {
			followerIDs = append(followerIDs, f.FollowerID)
		}
		// update redis
		_, err = s.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			for _, f := range followerInfos {
				err = pipe.ZAddNX(ctx, userIDStr+":followees", redis.Z{
					Member: f.FollowerID,
					Score:  float64(f.Timestamp),
				}).Err()
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			logger.Error("error getting updating redis with followees from mongodb")
			return nil, err
		}
	}
	return followerIDs, nil
}

func (s *socialGraphService) Follow(ctx context.Context, reqID int64, userID int64, followeeID int64) error {
	logger := s.Logger(ctx)
	logger.Debug("entering Follow", "req_id", reqID, "user_id", userID, "followee_id", followeeID)

	timestamp := time.Now()
	userIDStr := strconv.FormatInt(userID, 10)
	followeeIDstr := strconv.FormatInt(followeeID, 10)
	var mongoUpdateFollowerErr, mongoUpdateFolloweeErr, redisUpdateErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		// Update follower->followee edges
		defer wg.Done()
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		searchNotExist := bson.M{
			"$and": []bson.M{
				{"user_id": userID},
				{"followees": bson.M{"$not": bson.M{"$elemMatch": bson.M{"user_id": followeeID}}}},
			},
		}
		pushFollower := bson.M{
			"$push": bson.M{
				"followees": bson.M{
					"user_id":   followeeID,
					"timestamp": timestamp.String(),
				},
			},
		}
		updateResult, mongoUpdateFollowerErr := collection.UpdateOne(ctx, searchNotExist, pushFollower)
		if mongoUpdateFollowerErr != nil {
			logger.Error("error updating followees in mongodb")
		}
		logger.Debug("updated follower->followee edges in mongodb", "#matched", updateResult.MatchedCount, "#modified", updateResult.ModifiedCount)
	}()
	go func() {
		// Update followee->follower edges
		defer wg.Done()
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		searchNotExist := bson.M{
			"$and": []bson.M{
				{"user_id": followeeID},
				{"followers": bson.M{"$not": bson.M{"$elemMatch": bson.M{"user_id": userID}}}},
			},
		}
		pushFollowees := bson.M{
			"$push": bson.M{
				"followers": bson.M{
					"user_id":   userID,
					"timestamp": timestamp.String(),
				},
			},
		}
		updateResult, mongoUpdateFolloweeErr := collection.UpdateOne(ctx, searchNotExist, pushFollowees)
		if mongoUpdateFolloweeErr != nil {
			logger.Error("error updating followers in mongodb")
		}
		logger.Debug("updated followee->follower edges in mongodb", "#matched", updateResult.MatchedCount, "#modified", updateResult.ModifiedCount)
	}()
	go func() {
		defer wg.Done()
		_, redisUpdateErr = s.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			// followees
			_, err := pipe.ZCard(ctx, userIDStr+":followees").Result()
			if err == nil {
				pipe.ZAddNX(ctx, userIDStr+":followees", redis.Z{
					Member: followeeID,
					Score:  float64(timestamp.Unix()),
				})
			}
			// followers
			_, err = pipe.ZCard(ctx, followeeIDstr+":followers").Result()
			if err == nil {
				pipe.ZAddNX(ctx, followeeIDstr+":followers", redis.Z{
					Member: userID,
					Score:  float64(timestamp.Unix()),
				})
			}
			return nil
		})
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

// Unfollow removed the follower (from userID) and followee in mongodb and then in redis
func (s *socialGraphService) Unfollow(ctx context.Context, reqID int64, userID int64, followeeID int64) error {
	logger := s.Logger(ctx)
	logger.Debug("entering Unfollow", "req_id", reqID, "user_id", userID, "followee_id", followeeID)

	userIDStr := strconv.FormatInt(userID, 10)
	followeeIDstr := strconv.FormatInt(followeeID, 10)
	var err1, err2, err3 error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		// update follower->followee edges
		defer wg.Done()
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		filter := bson.D{
			{Key: "user_id", Value: userID},
		}
		update := bson.M{
			"$pull": bson.M{
				"followees": bson.M{
					"user_id": followeeID,
				},
			},
		}
		updateResult, err1 := collection.UpdateOne(ctx, filter, update)
		if err1 != nil {
			logger.Error("error pulling followee in mongodb")
		}
		logger.Debug("updated followee->follower edges in mongodb", "#matched", updateResult.MatchedCount, "#modified", updateResult.ModifiedCount)
	}()
	go func() {
		// update followee->follower edges
		defer wg.Done()
		collection := s.mongoClient.Database("social-graph").Collection("social-graph")
		filter := bson.D{
			{Key: "user_id", Value: followeeID},
		}
		update := bson.M{
			"$pull": bson.M{
				"followers": bson.M{
					"user_id": userID,
				},
			},
		}
		updateResult, err2 := collection.UpdateOne(ctx, filter, update)
		if err2 != nil {
			logger.Error("error pulling follower in mongodb")
		}
		logger.Debug("updated followee->follower edges in mongodb", "#matched", updateResult.MatchedCount, "#modified", updateResult.ModifiedCount)
	}()
	go func() {
		defer wg.Done()
		_, err3 = s.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			// followees
			_, err := pipe.ZCard(ctx, userIDStr+":followees").Result()
			if err == nil {
				pipe.ZRem(ctx, userIDStr+":followees", followeeID)
			}
			// followers
			_, err = pipe.ZCard(ctx, followeeIDstr+":followers").Result()
			if err == nil {
				pipe.ZRem(ctx, followeeIDstr+":followers", userID)
			}
			return nil
		})
	}()
	wg.Wait()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return err3
}

// FollowWithUsername
func (s *socialGraphService) FollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error {
	var userId, followeeId int64
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		userId, err1 = s.userService.Get().GetUserId(ctx, reqID, userUsername)
	}()
	go func() {
		defer wg.Done()
		followeeId, err2 = s.userService.Get().GetUserId(ctx, reqID, followeeUsername)
	}()
	wg.Wait()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return s.Follow(ctx, reqID, userId, followeeId)
}

// UnfollowWithUsername
func (s *socialGraphService) UnfollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error {
	var userId, followeeId int64
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		userId, err1 = s.userService.Get().GetUserId(ctx, reqID, userUsername)
	}()
	go func() {
		defer wg.Done()
		followeeId, err2 = s.userService.Get().GetUserId(ctx, reqID, followeeUsername)
	}()
	wg.Wait()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return s.Unfollow(ctx, reqID, userId, followeeId)
}

// InsertUser writes the user to mongodb
func (s *socialGraphService) InsertUser(ctx context.Context, reqID int64, userID int64) error {
	logger := s.Logger(ctx)
	logger.Debug("entering InsertUser", "req_id", reqID, "user_id", userID)
	collection := s.mongoClient.Database("social-graph").Collection("social-graph")
	doc := bson.D{
		{Key: "user_id", Value: userID},
		{Key: "followers", Value: bson.A{}},
		{Key: "followees", Value: bson.A{}},
	}
	_, err := collection.InsertOne(ctx, doc)
	return err
}
