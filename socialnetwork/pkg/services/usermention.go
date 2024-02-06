package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserMentionService interface {
	UploadUserMentions(ctx context.Context, reqID int64, usernames []string) error
}

type userMentionService struct {
	weaver.Implements[UserMentionService]
	weaver.WithConfig[userMentionServiceOptions]
	composePost weaver.Ref[ComposePostService]
	mongoClient *mongo.Client
	redisClient *redis.Client
}

type userMentionServiceOptions struct {
	MongoDBAddr 	string `toml:"mongodb_address"`
	MongoDBPort 	int    `toml:"mongodb_port"`
	MemCachedAddr 	string `toml:"memcached_addr"`
	MemCachedPort 	int    `toml:"memcached_port"`
}

func (u *userMentionService) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	var err error
	u.mongoClient, err = storage.MongoDBClient(ctx, u.Config().MongoDBAddr, u.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	u.redisClient = storage.RedisClient(u.Config().MemCachedAddr, u.Config().MemCachedPort)

	logger.Info("user mention service running!",
		"mongodb_addr", u.Config().MongoDBAddr, "mongodb_port", u.Config().MongoDBPort,
		"memcached_addr", u.Config().MemCachedAddr, "memcached_port", u.Config().MemCachedPort,
	)
	return nil
}

func (u *userMentionService) UploadUserMentions(ctx context.Context, reqID int64, usernames []string) error {
	logger := u.Logger(ctx)
	logger.Debug("entering UploadUserMentions", "req_id", reqID, "usernames", usernames)

	usersNotCached := make(map[string]bool)
	revLookup := make(map[string]string)
	var keys []string
	for _, name := range usernames {
		usersNotCached[name] = true
		keys = append(keys, name+":user_id")
		revLookup[name+":user_id"] = name
	}
	values := make([]int64, len(keys))
	var retvals []interface{}
	for i := range values {
		retvals = append(retvals, &values[i])
	}

	if len(keys) > 0 {
		result, err := u.redisClient.MGet(ctx, keys...).Result()
		if err != nil {
			logger.Error("error reading keys from redis", "msg", err.Error())
			return err
		}
		for i, data := range result {
			err := json.Unmarshal([]byte(data.(string)), retvals[i])
			if err != nil {
				logger.Error("error parsing result from redis", "msg", err.Error())
				return err
			}
		}
	}

	var userMentions []model.UserMention
	for i, key := range keys {
		user_mention := model.UserMention{
			UserID:   values[i],
			Username: revLookup[key],
		}
		userMentions = append(userMentions, user_mention)
		delete(usersNotCached, revLookup[key])
	}
	if len(usersNotCached) != 0 {
		var names []string
		for name := range usersNotCached {
			names = append(names, name)
		}
		collection := u.mongoClient.Database("user").Collection("user")
		filter := `{"username": {"$in": ` + strings.Join(strings.Fields(fmt.Sprint(names)), ",") + `}}`
		cur, err := collection.Find(ctx, filter)
		if err != nil {
			return err
		}
		var newUserMentions []model.UserMention
		cur.Decode(&newUserMentions) // ignore errors
		userMentions = append(userMentions, newUserMentions...)
	}
	return u.composePost.Get().UploadUserMentions(ctx, reqID, userMentions)
}
