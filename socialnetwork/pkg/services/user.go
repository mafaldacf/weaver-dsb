package services

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"math/rand"
	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ServiceWeaver/weaver"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserService interface {
	Login(ctx context.Context, reqID int64, username string, password string) (string, error)
	RegisterUserWithId(ctx context.Context, reqID int64, firstName string, lastName string, username string, password string, userID int64) error
	RegisterUser(ctx context.Context, reqID int64, firstName string, lastName string, username string, password string) error
	UploadCreatorWithUserId(ctx context.Context, reqID int64, userID int64, username string) error
	UploadCreatorWithUsername(ctx context.Context, reqID int64, username string) error
	GetUserId(ctx context.Context, reqID int64, username string) (int64, error)
}

type userService struct {
	weaver.Implements[UserService]
	weaver.WithConfig[userServiceOptions]
	socialGraphService   weaver.Ref[SocialGraphService]
	composePostService   weaver.Ref[ComposePostService]
	machineID 			string
	counter 			int64
	currentTimestamp 	int64
	secret 				string
	mongoClient   		*mongo.Client
	redisClient   		*redis.Client
	mu 					sync.Mutex
}

type userServiceOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
	RedisAddr   string `toml:"redis_address"`
	RedisPort   int    `toml:"redis_port"`
}

func (u *userService) getCounter(timestamp int64) (int64, error) {
	u.mu.Lock()
    defer u.mu.Unlock()
	if u.currentTimestamp > timestamp {
		return 0, fmt.Errorf("timestamps are not incremental")
	}
	if u.currentTimestamp == timestamp {
		counter := u.counter
		u.counter += 1
		return counter, nil
	} else {
		u.currentTimestamp = timestamp
		u.counter = 1
		return u.counter, nil
	}

}

func (u *userService) genRandomStr(length int) string {
	b := make([]rune, length)
    for i := range b {
        b[i] = letterRunes[rand.Intn(len(letterRunes))]
    }
    return string(b)
}

func (u *userService) hashPwd(pwd []byte) string {
	hasher := sha1.New()
	hasher.Write(pwd)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (u *userService) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	var err error
	u.mongoClient, err = storage.MongoDBClient(ctx, u.Config().MongoDBAddr, u.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	u.redisClient = storage.RedisClient(u.Config().RedisAddr, u.Config().RedisPort)
	logger.Info("user service running!", 
		"mongodb_addr", u.Config().MongoDBAddr, "mongodb_port", u.Config().MongoDBPort, 
		"redis_addr", u.Config().RedisAddr, "redis_port", u.Config().RedisPort,
	)
	return nil
}

func (u *userService) Login(ctx context.Context, reqID int64, username string, password string) (string, error) {
	//TODO
	return "", nil
}

func (u *userService) RegisterUserWithId(ctx context.Context, reqID int64, firstName string, lastName string, username string, password string, userID int64) error {
	logger := u.Logger(ctx)
	logger.Debug("entering RegisterUserWithId", "req_id", reqID, "first_name", firstName, "last_name", lastName, "username", username, "password", password, "user_id", userID)
	
	collection := u.mongoClient.Database("user").Collection("user")
	filter := `{"Username":"` +  username + `"}`
	cur, err := collection.Find(ctx, filter)
	if err != nil {
		logger.Error("error finding user in mongodb", "msg", err.Error())
		return err
	}
	user := model.User {
		UserID: -1,
	}
	err = cur.Decode(&user)
	if err != nil {
		logger.Error("error parsing user from mongodb result", "msg", err.Error())
		return err
	}
	user.UserID = -1
	if user.UserID != -1 {
		errMsg := "username already registered"
		logger.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	salt := u.genRandomStr(32)
	hashedPwd := u.hashPwd([]byte(password + salt))
	user = model.User{
		UserID: userID, 
		FirstName: firstName, 
		LastName: lastName, 
		Username: username,
		PwdHashed: hashedPwd, 
		Salt: salt, 
	}
	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		logger.Error("error inserting new user in mongodb")
		return err
	}
	return u.socialGraphService.Get().InsertUser(ctx, reqID, userID)
}

func (u *userService) RegisterUser(ctx context.Context, reqID int64, firstName string, lastName string, username string, password string) error {
	logger := u.Logger(ctx)
	logger.Debug("entering RegisterUser", "req_id", reqID, "first_name", firstName, "last_name", lastName, "username", username, "password", password)

	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	idx, err := u.getCounter(timestamp)
	if err != nil {
		return err
	}
	timestamp_hex := strconv.FormatInt(timestamp, 16)
	if len(timestamp_hex) > 10 {
		timestamp_hex = timestamp_hex[:10]
	} else if len(timestamp_hex) < 10 {
		timestamp_hex = strings.Repeat("0", 10-len(timestamp_hex)) + timestamp_hex
	}
	counter_hex := strconv.FormatInt(idx, 16)
	if len(counter_hex) > 3 {
		counter_hex = counter_hex[:3]
	} else if len(counter_hex) < 3 {
		counter_hex = strings.Repeat("0", 3-len(counter_hex)) + counter_hex
	}
	user_id_str := u.machineID + timestamp_hex + counter_hex
	user_id, err := strconv.ParseInt(user_id_str, 10, 64)
	if err != nil {
		return err
	}
	user_id = user_id & 0x7FFFFFFFFFFFFFFF
	return u.RegisterUserWithId(ctx, reqID, firstName, lastName, username, password, user_id)
}

func (u *userService) UploadCreatorWithUserId(ctx context.Context, reqID int64, userID int64, username string) error {
	logger := u.Logger(ctx)
	logger.Debug("entering UploadCreatorWithUserId", "req_id", reqID, "user_id", userID, "username", username)
	creator := model.Creator {
		UserID: userID,
		Username: username,
	}
	return u.composePostService.Get().UploadCreator(ctx, reqID, creator)
}

func (u *userService) UploadCreatorWithUsername(ctx context.Context, reqID int64, username string) error {
	//TODO
	return nil
}

func (u *userService) GetUserId(ctx context.Context, reqID int64, username string) (int64, error) {
	//TODO
	return 0, nil
}
