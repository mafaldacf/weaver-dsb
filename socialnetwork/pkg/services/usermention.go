package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/bradfitz/gomemcache/memcache"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserMentionService interface {
	UploadUserMentions(ctx context.Context, reqID int64, usernames []string) error
}

type userMentionService struct {
	weaver.Implements[UserMentionService]
	weaver.WithConfig[userMentionServiceOptions]
	composePost 	weaver.Ref[ComposePostService]
	mongoClient 	*mongo.Client
	memCachedClient *memcache.Client
}

type userMentionServiceOptions struct {
	MongoDBAddr 	string 	`toml:"mongodb_address"`
	MemCachedAddr 	string 	`toml:"memcached_address"`
	MongoDBPort 	int    	`toml:"mongodb_port"`
	MemCachedPort 	int    	`toml:"memcached_port"`
	Region    		string  `toml:"region"`
}

func (u *userMentionService) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	var err error
	u.mongoClient, err = storage.MongoDBClient(ctx, u.Config().MongoDBAddr, u.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	u.memCachedClient = storage.MemCachedClient(u.Config().MemCachedAddr, u.Config().MemCachedPort)

	logger.Info("user mention service running!", "region", u.Config().Region,
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

	result, err := u.memCachedClient.GetMulti(keys)
	if err != nil {
		logger.Error("error reading keys from redis", "msg", err.Error())
		return err
	}
	userIDsCached := []int64{}
	for _, key := range keys {
		if val, ok := result[key]; ok {
			var userID int64
			err := json.Unmarshal(val.Value, &userID)
			if err != nil {
				logger.Error("error parsing ids from memcached result", "msg", err.Error())
				return nil
			}
			userIDsCached = append(userIDsCached, userID)
		}
	}

	var userMentions []model.UserMention
	for i, key := range keys {
		user_mention := model.UserMention{
			UserID:   userIDsCached[i],
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
