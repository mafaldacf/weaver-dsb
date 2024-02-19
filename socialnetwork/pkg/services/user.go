package services

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"
	"socialnetwork/pkg/utils"
	"strconv"
	"sync"
	"time"

	"github.com/ServiceWeaver/weaver"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dgrijalva/jwt-go"
	"go.mongodb.org/mongo-driver/bson"
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

type LoginInfo struct {
	UserID   int64  `bson:"user_id"`
	Password string `bson:"password"`
	Salt     string `bson:"salt"`
}

type Claims struct {
	Username  string `bson:"username"`
	UserID    string `bson:"user_id"`
	Timestamp int64  `bson:"timestamp"`
	jwt.StandardClaims
}

type userService struct {
	weaver.Implements[UserService]
	weaver.WithConfig[userServiceOptions]
	socialGraphService weaver.Ref[SocialGraphService]
	composePostService weaver.Ref[ComposePostService]
	machineID          string
	counter            int64
	currentTimestamp   int64
	secret             string
	mongoClient        *mongo.Client
	memCachedClient    *memcache.Client
	mu                 sync.Mutex
}

type userServiceOptions struct {
	MongoDBAddr   string `toml:"mongodb_address"`
	MemCachedAddr string `toml:"memcached_address"`
	MongoDBPort   int    `toml:"mongodb_port"`
	MemCachedPort int    `toml:"memcached_port"`
	Region    	  string `toml:"region"`
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
	u.machineID = utils.GetMachineID()
	u.currentTimestamp = -1
	u.counter = 0
	u.mongoClient, err = storage.MongoDBClient(ctx, u.Config().MongoDBAddr, u.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	u.memCachedClient = storage.MemCachedClient(u.Config().MemCachedAddr, u.Config().MemCachedPort)
	logger.Info("user service running!", "region", u.Config().Region,
		"mongodb_addr", u.Config().MongoDBAddr, "mongodb_port", u.Config().MongoDBPort,
		"memcached_addr", u.Config().MemCachedAddr, "memcached_port", u.Config().MemCachedPort,
		"machine_id", u.machineID,
	)
	return nil
}

func (u *userService) Login(ctx context.Context, reqID int64, username string, password string) (string, error) {
	logger := u.Logger(ctx)
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	var loginInfo LoginInfo
	item, err := u.memCachedClient.Get(username + ":login")
	if err != nil && err != memcache.ErrCacheMiss {
		// error reading cache
		logger.Error("error reading user login info from cache", "msg", err.Error())
		return "", err
	}
	if err == nil {
		// user login info found in cache
		err := json.Unmarshal(item.Value, &loginInfo)
		if err != nil {
			logger.Error("error parsing post from cache result", "msg", err.Error())
			return "", err
		}
	} else {
		// user login info not cached
		// so we get it from db
		user := model.User{
			UserID: -1,
		}
		collection := u.mongoClient.Database("user").Collection("user")
		filter := bson.D{
			{Key: "username", Value: username},
		}
		cur, err := collection.Find(ctx, filter)
		if err != nil {
			logger.Error("error finding user in mongodb", "msg", err.Error())
			return "", err
		}
		exists := cur.TryNext(ctx)
		if !exists {
			msg := fmt.Sprintf("username %s does not exist", username)
			logger.Debug(msg)
			return "", fmt.Errorf(msg)
		}
		err = cur.Decode(&user)
		if err != nil {
			logger.Error("error parsing user from mongodb result", "msg", err.Error())
			return "", err
		}
		loginInfo.Password = user.PwdHashed
		loginInfo.Salt = user.Salt
		loginInfo.UserID = user.UserID
	}
	var tokenStr string
	hashed_pwd := u.hashPwd([]byte(password + loginInfo.Salt))
	if hashed_pwd != loginInfo.Password {
		return "", fmt.Errorf("invalid credentials")
	} else {
		expiration_time := time.Now().Add(6 * time.Minute)
		claims := &Claims{
			Username:       username,
			UserID:         strconv.FormatInt(loginInfo.UserID, 10),
			Timestamp:      timestamp,
			StandardClaims: jwt.StandardClaims{ExpiresAt: expiration_time.Unix()},
		}
		var err error
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err = token.SignedString([]byte(u.secret))
		if err != nil {
			return "", fmt.Errorf("failed to create login token")
		}
	}
	loginInfoJson, err := json.Marshal(loginInfo)
	if err != nil {
		logger.Error("error converting login info to json", "login_info", loginInfo)
		return "", err
	}
	err = u.memCachedClient.Set(&memcache.Item{Key: username + ":login", Value: loginInfoJson})
	if err != nil {
		logger.Error("error caching login info", "login_info", loginInfo)
		return "", err
	}
	return tokenStr, nil
}

func (u *userService) RegisterUserWithId(ctx context.Context, reqID int64, firstName string, lastName string, username string, password string, userID int64) error {
	logger := u.Logger(ctx)
	logger.Debug("entering RegisterUserWithId", "req_id", reqID, "first_name", firstName, "last_name", lastName, "username", username, "password", password, "user_id", userID)

	collection := u.mongoClient.Database("user").Collection("user")
	filter := bson.D{
		{Key: "username", Value: username},
	}
	cur, err := collection.Find(ctx, filter)
	if err != nil {
		logger.Error("error finding user in mongodb", "msg", err.Error())
		return err
	}
	exists := cur.TryNext(ctx)
	if exists {
		errMsg := fmt.Sprintf("username %s already registered", username)
		logger.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	salt := u.genRandomStr(32)
	hashedPwd := u.hashPwd([]byte(password + salt))
	user := model.User{
		UserID:    userID,
		FirstName: firstName,
		LastName:  lastName,
		Username:  username,
		PwdHashed: hashedPwd,
		Salt:      salt,
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

	timestamp := time.Now().UnixMilli() - utils.CUSTOM_EPOCH
	counter, err := u.getCounter(timestamp)
	if err != nil {
		logger.Error("error getting counter", "msg", err.Error())
		return err
	}
	id, err := utils.GenUniqueID(u.machineID, timestamp, counter)
	if err != nil {
		return err
	}
	return u.RegisterUserWithId(ctx, reqID, firstName, lastName, username, password, id)
}

// UploadCreatorWithUserId returns a new creator object
func (u *userService) UploadCreatorWithUserId(ctx context.Context, reqID int64, userID int64, username string) error {
	logger := u.Logger(ctx)
	logger.Debug("entering UploadCreatorWithUserId", "req_id", reqID, "user_id", userID, "username", username)
	creator := model.Creator{
		UserID:   userID,
		Username: username,
	}
	return u.composePostService.Get().UploadCreator(ctx, reqID, creator)
}

// UploadCreatorWithUsername attempts to read the user id from cache and return it
// If not found, it fetches the user from the db and uploads it to cache
func (u *userService) UploadCreatorWithUsername(ctx context.Context, reqID int64, username string) error {
	logger := u.Logger(ctx)
	logger.Debug("entering UploadCreatorWithUsername", "req_id", reqID, "username", username)

	var userID int64
	item, err := u.memCachedClient.Get(username + ":user_id")
	if err != nil && err != memcache.ErrCacheMiss {
		// error reading cache
		logger.Error("error reading user id from cache", "msg", err.Error())
		return err
	}
	if err == nil {
		// user id info found in cache
		err := json.Unmarshal(item.Value, &userID)
		if err != nil {
			logger.Error("error parsing user id from cache result", "msg", err.Error())
			return err
		}
	} else {
		// user not found in cache
		// so we get it from db and write to cache
		var user model.User
		collection := u.mongoClient.Database("user").Collection("user")
		filter := bson.D{
			{Key: "username", Value: username},
		}
		cur, err := collection.Find(ctx, filter)
		if err != nil {
			logger.Debug("error finding user in mongodb", "msg", err.Error())
			return err
		}
		exists := cur.TryNext(ctx)
		if !exists {
			msg := fmt.Sprintf("username %s does not exist", username)
			logger.Debug(msg)
			return fmt.Errorf(msg)
		}
		err = cur.Decode(&user)
		if err != nil {
			logger.Error("error parsing user from mongodb result", "msg", err.Error())
			return err
		}
		userID = user.UserID
		userIDJson, err := json.Marshal(userID)
		if err != nil {
			logger.Error("error converting user ID to json", "userID", userID)
			return err
		}
		err = u.memCachedClient.Set(&memcache.Item{Key: username + ":user_id", Value: userIDJson})
		if err != nil {
			logger.Error("error caching user id", "userID", userID)
			return err
		}
	}
	return u.UploadCreatorWithUserId(ctx, reqID, userID, username)
}

// GetUserId attempts to read the user id from cache and return it
// If not found, it fetches the user from the db and uploads it to cache
func (u *userService) GetUserId(ctx context.Context, reqID int64, username string) (int64, error) {
	logger := u.Logger(ctx)
	logger.Debug("entering GetUserId", "req_id", reqID, "username", username)

	var userID int64
	item, err := u.memCachedClient.Get(username + ":user_id")
	if err != nil && err != memcache.ErrCacheMiss {
		// error reading cache
		logger.Error("error reading user id from cache", "msg", err.Error())
		return 0, err
	}
	if err == nil {
		// user id info found in cache
		err := json.Unmarshal(item.Value, &userID)
		if err != nil {
			logger.Error("error parsing user id from cache result", "msg", err.Error())
			return 0, err
		}
	} else {
		// user not found in cache
		// so we get it from db and write to cache
		user := model.User{
			UserID: -1,
		}
		collection := u.mongoClient.Database("user").Collection("user")
		filter := bson.D{
			{Key: "username", Value: username},
		}
		cur, err := collection.Find(ctx, filter)
		if err != nil {
			logger.Debug("error finding user in mongodb", "msg", err.Error())
			return 0, err
		}
		exists := cur.TryNext(ctx)
		if !exists {
			msg := fmt.Sprintf("username %s does not exist", username)
			logger.Debug(msg)
			return 0, fmt.Errorf(msg)
		}
		err = cur.Decode(&user)
		if err != nil {
			logger.Error("error parsing user from mongodb result", "msg", err.Error())
			return 0, err
		}
		userID = user.UserID
		userIDJson, err := json.Marshal(userID)
		if err != nil {
			logger.Error("error converting user ID to json", "userID", userID)
			return 0, err
		}
		err = u.memCachedClient.Set(&memcache.Item{Key: username + ":user_id", Value: userIDJson})
		if err != nil {
			logger.Error("error caching user id", "userID", userID)
			return 0, err
		}
	}
	return userID, nil
}
