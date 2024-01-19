package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type ComposePostService interface {
	UploadCreator(ctx context.Context, reqID int64, creator model.Creator) error
	UploadText(ctx context.Context, reqID int64, text string) error
	UploadMedia(ctx context.Context, reqID int64, medias []model.Media) error
	UploadUniqueId(ctx context.Context, reqID int64, postID int64, postType model.PostType) error
	UploadUrls(ctx context.Context, reqID int64, urls []model.URL) error 
	UploadUserMentions(ctx context.Context, reqID int64, userMentions []model.UserMention) error
}

const NUM_COMPONENTS int = 6 // corresponds to the number of exposed methods
const REDIS_EXPIRE_TIME int = 12

type composePostService struct {
	weaver.Implements[ComposePostService]
	weaver.WithConfig[composePostServiceOptions]
	postStorageService   	weaver.Ref[PostStorageService]
	userTimelineService  	weaver.Ref[UserTimelineService]
	_             			weaver.Ref[WriteHomeTimelineService]
	amqChannel    			*amqp.Channel
	amqConnection 			*amqp.Connection
	redisClient   			*redis.Client
}

type composePostServiceOptions struct {
	RabbitMQAddr     string   `toml:"rabbitmq_address"`
	RabbitMQPort     int      `toml:"rabbitmq_port"`
	RabbitMQUsername string   `toml:"rabbitmq_username"`
	RabbitMQPassword string   `toml:"rabbitmq_password"`
	RedisAddr        string   `toml:"redis_address"`
	RedisPort        int      `toml:"redis_port"`
	Region           string   `toml:"region"`
	Regions          []string `toml:"regions"`
}

func (c *composePostService) Init(ctx context.Context) error {
	logger := c.Logger(ctx)
	logger.Debug("initializing compose post service...")

	var err error
	c.amqChannel, c.amqConnection, err = storage.RabbitMQClient(ctx, c.Config().RabbitMQUsername, c.Config().RabbitMQPassword, c.Config().RabbitMQAddr, c.Config().RabbitMQPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	c.redisClient = storage.RedisClient(c.Config().RedisAddr, c.Config().RedisPort)

	logger.Info("compose post service running!", "region", c.Config().Region, "rabbitmq_addr", c.Config().RabbitMQAddr, "rabbitmq_port", c.Config().RabbitMQPort)
	return nil
}

func (c *composePostService) uploadComponent(ctx context.Context, reqID int64, fieldsValues ...interface{}) error {
	logger := c.Logger(ctx)
	reqIDStr := strconv.FormatInt(reqID, 10)
	cmds, err := c.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, reqIDStr, fieldsValues)
		pipe.HIncrBy(ctx, reqIDStr, "num_components", 1)
		pipe.Expire(ctx, reqIDStr, time.Second * time.Duration(REDIS_EXPIRE_TIME))
		return nil
	})

	if err != nil || len(cmds) != 3 /* sanity check */ {
		logger.Error("error writing component to redis", "fieldValues", fieldsValues, "msg", err.Error())
		return err
	}

	numComponents, err := cmds[1].(*redis.IntCmd).Result()
	if err != nil {
		logger.Error("error reading number of written components from redis", "fieldValues", fieldsValues, "msg", err.Error())
		return err
	}

	if numComponents == int64(NUM_COMPONENTS) {
		return c.composeAndUpload(ctx, reqID)
	}

	return nil
}

func (c *composePostService) UploadText(ctx context.Context, reqID int64, text string) error {
	logger := c.Logger(ctx)
	logger.Info("uploading text", "text", text)
	textJSON, err := json.Marshal(text)
	if err != nil {
		logger.Error("error converting text to json", "text", text)
		return err
	}
	return c.uploadComponent(ctx, reqID, "text", textJSON)
}


func (c *composePostService) UploadMedia(ctx context.Context, reqID int64, medias []model.Media) error {
	logger := c.Logger(ctx)
	logger.Info("uploading medias", "medias", medias)
	mediasJSON, err := json.Marshal(medias)
	if err != nil {
		logger.Error("error converting medias to json", "medias", medias)
		return err
	}
	return c.uploadComponent(ctx, reqID, "media", mediasJSON)
}


func (c *composePostService) UploadUniqueId(ctx context.Context, reqID int64, postID int64, postType model.PostType) error {
	logger := c.Logger(ctx)
	logger.Info("uploading unique id", "post_id", postID, "post_type", postType)
	postIDJSON, err := json.Marshal(postID)
	if err != nil {
		logger.Error("error converting post_id to json", "post_id", postID)
		return err
	}
	postTypeJSON, err := json.Marshal(postType)
	if err != nil {
		logger.Error("error converting medias to json", "post_type", postType)
		return err
	}
	return c.uploadComponent(ctx, reqID, "post_id", postIDJSON, "post_type", postTypeJSON)
}


func (c *composePostService) UploadUrls(ctx context.Context, reqID int64, urls []model.URL) error {
	logger := c.Logger(ctx)
	logger.Info("uploading urls", "urls", urls)
	urlsJSON, err := json.Marshal(urls)
	if err != nil {
		logger.Error("error converting urls to json", "urls", urls)
		return err
	}
	return c.uploadComponent(ctx, reqID, "urls", urlsJSON)
}

func (c *composePostService) UploadUserMentions(ctx context.Context, reqID int64, userMentions []model.UserMention) error {
	logger := c.Logger(ctx)
	logger.Info("uploading user mentions", "user_mentions", userMentions)
	userMentionsJSON, err := json.Marshal(userMentions)
	if err != nil {
		logger.Error("error converting user mentions to json", "user_mentions", userMentions)
		return err
	}
	return c.uploadComponent(ctx, reqID, "user_mentions", userMentionsJSON)
}

func (c *composePostService) UploadCreator(ctx context.Context, reqID int64, creator model.Creator) error {
	logger := c.Logger(ctx)
	logger.Info("uploading creator", "creator", creator)
	creatorJSON, err := json.Marshal(creator)
	if err != nil {
		logger.Error("error converting creator to json", "user_mentions", creatorJSON)
		return err
	}
	return c.uploadComponent(ctx, reqID, "creator", creatorJSON)
}

func (c *composePostService) composeAndUpload(ctx context.Context, reqID int64) error {
	logger := c.Logger(ctx)
	logger.Info("entering ComposeAndUpload for ComposePostService service", "reqid", reqID)

	var text string
	var creator model.Creator
	var medias []model.Media
	var postID int64
	var urls []model.URL
	var userMentions []model.UserMention
	var postType model.PostType

	var errs [7]error
	var wg sync.WaitGroup
	wg.Add(7)

	reqIDStr := strconv.FormatInt(reqID, 10)
	loadComponent := func(key string, value interface{}) error {
		logger.Info("loading component", "reqid", reqIDStr, "key", key)
		cmd := c.redisClient.HGet(ctx, reqIDStr, key)
		if cmd == nil || cmd.Err() != nil {
			return cmd.Err()
		}
		result, err := cmd.Bytes()
		if err != nil {
			return err
		}
		return json.Unmarshal(result, &value)
	}

	go func() {
		defer wg.Done()
		errs[0] = loadComponent("text", &text)
	}()
	go func() {
		defer wg.Done()
		errs[1] = loadComponent("creator", &creator)
	}()
	go func() {
		defer wg.Done()
		errs[2] = loadComponent("media", &medias)
	}()
	go func() {
		defer wg.Done()
		errs[3] = loadComponent("post_id", &postID)
	}()
	go func() {
		defer wg.Done()
		errs[4] = loadComponent("urls", &urls)
	}()
	go func() {
		defer wg.Done()
		errs[5] = loadComponent("user_mentions", &userMentions)
	}()
	go func() {
		defer wg.Done()
		errs[6] = loadComponent("post_type", &postType)
	}()
	wg.Wait()
	logger.Info("got all components from redis")

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
		Media: 			medias,
		URLs: 			urls,
		Timestamp: 		timestamp,
		PostType: 		postType,
	}
	var userMentionIDs []int64
	for _, mention := range userMentions {
		userMentionIDs = append(userMentionIDs, mention.UserID)
	}

	// --- Post Storage
	logger.Info("remotely calling PostStorageService")

	err := c.postStorageService.Get().StorePost(ctx, reqID, post)
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
	c.userTimelineService.Get().WriteUserTimeline(ctx, reqID, postID, post.Creator.UserID, timestamp)

	logger.Info("done!")
	return nil
}

func (c *composePostService) uploadHomeTimelineHelper(ctx context.Context, reqID int64, postID int64, timestamp int64, userMentionIDs []int64) error {
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
