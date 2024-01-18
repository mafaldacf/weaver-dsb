package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"socialnetwork/services/model"
	"socialnetwork/services/utils"

	"github.com/ServiceWeaver/weaver"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type ComposePost interface {
	UploadCreator(ctx context.Context, reqID int64, creator model.Creator) error
	UploadText(ctx context.Context, reqID int64, text string) error
	UploadMedia(ctx context.Context, reqID int64, media []model.Media) error
	UploadUniqueId(ctx context.Context, reqID int64, postID int64, postType model.PostType) error
	UploadUrls(ctx context.Context, reqID int64, urls []model.URL) error 
	UploadUserMentions(ctx context.Context, reqID int64, user_mentions []model.UserMention) error
}

const NUM_COMPONENTS int = 6 // corresponds to the number of exposed methods
const REDIS_EXPIRE_TIME int = 12

type composePost struct {
	weaver.Implements[ComposePost]
	weaver.WithConfig[composePostOptions]
	postStorage   weaver.Ref[PostStorage]
	userTimeline  weaver.Ref[UserTimeline]
	amqChannel    *amqp.Channel
	amqConnection *amqp.Connection
	redisClient   *redis.Client
}

type composePostOptions struct {
	RabbitMQAddr     string   `toml:"rabbitmq_address"`
	RabbitMQPort     int      `toml:"rabbitmq_port"`
	RabbitMQUsername string   `toml:"rabbitmq_username"`
	RabbitMQPassword string   `toml:"rabbitmq_password"`
	RedisAddr        string   `toml:"redis_address"`
	RedisPort        int      `toml:"redis_port"`
	Region           string   `toml:"region"`
	Regions          []string `toml:"regions"`
}

func (c *composePost) Init(ctx context.Context) error {
	logger := c.Logger(ctx)
	logger.Debug("initializing compose post service...")

	var err error
	c.amqChannel, c.amqConnection, err = utils.RabbitMQClient(ctx, c.Config().RabbitMQUsername, c.Config().RabbitMQPassword, c.Config().RabbitMQAddr, c.Config().RabbitMQPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	c.redisClient = utils.RedisClient(c.Config().RedisAddr, c.Config().RedisPort)

	logger.Info("compose post service running!", "region", c.Config().Region, "rabbitmq_addr", c.Config().RabbitMQAddr, "rabbitmq_port", c.Config().RabbitMQPort)
	return nil
}

func (c *composePost) uploadComponent(ctx context.Context, reqID int64, key string, value interface{}) error {
	logger := c.Logger(ctx)
	logger.Info("uploading component", "key", key)
	reqIDStr := strconv.FormatInt(reqID, 10)

	cmds, err := c.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, reqIDStr, key, value)
		pipe.HIncrBy(ctx, reqIDStr, "num_components", 1)
		pipe.Expire(ctx, reqIDStr, time.Second * time.Duration(REDIS_EXPIRE_TIME))
		return nil
	})

	if err != nil || len(cmds) != 3 /* sanity check */ {
		logger.Error("error writing creator to redis", "msg", err.Error())
		return err
	}

	numComponents, err := cmds[1].(*redis.IntCmd).Result()
	if err != nil {
		logger.Error("error reading number of components from redis", "msg", err.Error())
		return err
	}

	if numComponents == 3 {
		return c.composeAndUpload(ctx, reqID)
	}

	return nil
}

func (c *composePost) UploadText(ctx context.Context, reqID int64, text string) error {
	return c.uploadComponent(ctx, reqID, "text", text)
}


func (c *composePost) UploadMedia(ctx context.Context, reqID int64, media []model.Media) error {
	return c.uploadComponent(ctx, reqID, "media", media)
}


func (c *composePost) UploadUniqueId(ctx context.Context, reqID int64, postID int64, postType model.PostType) error {
	return c.uploadComponent(ctx, reqID, "post_id", postID)
	// TODO posttype
}


func (c *composePost) UploadUrls(ctx context.Context, reqID int64, urls []model.URL) error {
	return c.uploadComponent(ctx, reqID, "urls", urls)
}

func (c *composePost) UploadUserMentions(ctx context.Context, reqID int64, userMentions []model.UserMention) error {
	return c.uploadComponent(ctx, reqID, "user_mentions", userMentions)
}

func (c *composePost) UploadCreator(ctx context.Context, reqID int64, creator model.Creator) error {
	return c.uploadComponent(ctx, reqID, "creator", creator)
}

func (c *composePost) composeAndUpload(ctx context.Context, reqID int64) error {
	logger := c.Logger(ctx)
	logger.Info("entering ComposeAndUpload for ComposePost service", "reqid", reqID)

	var text string
	var creator model.Creator
	var media []model.Media
	var postID int64
	var urls []model.URL
	var userMentions []model.UserMention
	var postType model.PostType

	var errs []error
	var wg sync.WaitGroup
	wg.Add(7)

	reqIDStr := strconv.FormatInt(reqID, 10)
	redisHGetAndParse := func(key string, value interface{}) error {
		cmd := c.redisClient.HGet(ctx, reqIDStr, key)
		if cmd.Err() != nil {
			return cmd.Err()
		}
		result, err := cmd.Bytes()
		if err != nil {
			return err
		}
		return json.Unmarshal(result, &value)
	}

	logger.Info("fetching data from redis")
	go func() {
		defer wg.Done()
		errs[0] = redisHGetAndParse("text", &text)
	}()
	go func() {
		defer wg.Done()
		errs[1] = redisHGetAndParse("creator", &creator)
	}()
	go func() {
		defer wg.Done()
		errs[2] = redisHGetAndParse("media", &media)
	}()
	go func() {
		defer wg.Done()
		errs[3] = redisHGetAndParse("post_id", &postID)
	}()
	go func() {
		defer wg.Done()
		errs[4] = redisHGetAndParse("urls", &urls)
	}()
	go func() {
		defer wg.Done()
		errs[5] = redisHGetAndParse("user_mentions", &userMentions)
	}()
	go func() {
		defer wg.Done()
		errs[6] = redisHGetAndParse("post_type", &postType)
	}()
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			logger.Error("error reading from redis", "msg", err.Error())
			return err
		}
	}

	logger.Info("parsing post data")
	timestamp := time.Now().UnixMilli()
	post := model.Post {
		PostID: 		postID,
		ReqID: 			reqID,
		Text: 			text,
		UserMentions: 	userMentions,
		Media: 			media,
		URLs: 			urls,
		Timestamp: 		timestamp,
		PostType: 		postType,
	}
	var userMentionIDs []int64
	for _, mention := range userMentions {
		userMentionIDs = append(userMentionIDs, mention.UserID)
	}

	// --- Post Storage
	logger.Info("remotely calling PostStorage")

	err := c.postStorage.Get().StorePost(ctx, reqID, post)
	if err != nil {
		logger.Warn("error calling post storage service", "msg", err.Error())
		return err
	}

	// --- Evaluation
	trace.SpanFromContext(ctx).AddEvent("composing post",
		trace.WithAttributes(
			attribute.Int64("postID", postID),
			attribute.Int64("queue_start_ms", time.Now().UnixMilli()),
		))

	// --- Write Home Timeline
	logger.Info("queueing message to rabbitmq")
	c.uploadHomeTimelineHelper(ctx, reqID, postID, timestamp, userMentionIDs)

	// --- User Timeline
	c.userTimeline.Get().WriteUserTimeline(ctx, reqID, postID, post.Creator.UserID, timestamp)

	logger.Info("done!")
	return err
}

func (c *composePost) uploadHomeTimelineHelper(ctx context.Context, reqID int64, postID int64, timestamp int64, userMentionIDs []int64) error {
	logger := c.Logger(ctx)
	err := c.amqChannel.ExchangeDeclare("write-home-timeline", "topic", false, false, false, false, nil)
	if err != nil {
		logger.Error("error declaring exchange for rabbitmq", "msg", err.Error())
		return err
	}

	msgJSON, err := json.Marshal(model.Message{
		ReqID: reqID,
		PostID: postID,
		Timestamp: timestamp,
		UserMentionIDs: userMentionIDs,
	})
	if err != nil {
		logger.Error("error converting rabbitmq message to json", "msg", err.Error())
		return err
	}

	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(msgJSON),
	}
	for _, region := range c.Config().Regions {
		routingKey := fmt.Sprintf("write-home-timeline-%s", region)
		c.amqChannel.PublishWithContext(ctx, "write-home-timeline", routingKey, false, false, msg)
	}
	return nil
}
