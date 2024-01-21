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
	"github.com/ServiceWeaver/weaver/metrics"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type WriteHomeTimelineService interface {
	// WriteHomeTimelineService service does not expose any rpc methods
}

type writeHomeTimelineServiceOptions struct {
	RabbitMQAddr     string `toml:"rabbitmq_address"`
	RabbitMQPort     int    `toml:"rabbitmq_port"`
	MongoDBAddr      string `toml:"mongodb_address"`
	MongoDBPort      int    `toml:"mongodb_port"`
	RedisAddr   	 string `toml:"redis_address"`
	RedisPort   	 int    `toml:"redis_port"`
	NumWorkers       int    `toml:"num_workers"`
	Region           string `toml:"region"`
}

type writeHomeTimelineService struct {
	weaver.Implements[WriteHomeTimelineService]
	weaver.WithConfig[writeHomeTimelineServiceOptions]
	socialGraphService 	weaver.Ref[SocialGraphService]
	mongoClient 		*mongo.Client
	redisClient 		*redis.Client
}

var (
	inconsistencies = metrics.NewCounter(
		"inconsistencies",
		"The number of times an cross-service inconsistency has occured",
	)
)

func (w *writeHomeTimelineService) Init(ctx context.Context) error {
	logger := w.Logger(ctx)

	var err error
	w.mongoClient, err = storage.MongoDBClient(ctx, w.Config().MongoDBAddr, w.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	w.redisClient = storage.RedisClient(w.Config().RedisAddr, w.Config().RedisPort)

	var wg sync.WaitGroup
	wg.Add(w.Config().NumWorkers)
	for i := 1; i <= w.Config().NumWorkers; i++ {
		go func() {
			defer wg.Done()
			err := w.workerThread(ctx)
			logger.Error("error in worker thread", "msg", err.Error())
			}()
		}

	logger.Info("write home timeline service running!", "region", w.Config().Region, "n_workers", w.Config().NumWorkers, 
		"rabbitmq_addr", w.Config().RabbitMQAddr, "rabbitmq_port", w.Config().RabbitMQPort, 
		"mongodb_addr", w.Config().MongoDBAddr, "mongodb_port", w.Config().MongoDBPort, 
		"redis_addr", w.Config().RedisAddr, "redis_port", w.Config().RedisPort,
	)
	wg.Wait()
	return nil
}

func (w *writeHomeTimelineService) onReceivedWorker(ctx context.Context, body []byte) error {
	logger := w.Logger(ctx)

	var msg model.Message
	err := json.Unmarshal(body, &msg)
	if err != nil {
		logger.Error("error parsing json message", "msg", err.Error())
		return err
	}

	logger.Debug("received rabbitmq message", "postid", msg.PostID)

	trace.SpanFromContext(ctx).AddEvent("reading rabbitmq message",
		trace.WithAttributes(
			attribute.Int64("queue_end_ms", time.Now().UnixMilli()),
		))

	db := w.mongoClient.Database("poststorage")
	collection := db.Collection("posts")

	var post model.Post
	filter := bson.D{{Key: "post_id", Value: msg.PostID}}
	err = collection.FindOne(ctx, filter, nil).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			logger.Debug("inconsistency!")
			inconsistencies.Inc()
			return nil
		} else {
			logger.Error("error reading post from mongodb", "msg", err.Error())
			return err
		}
	}

	logger.Debug("found post! :)", "postid", post.PostID, "text", post.Text)

	followersID, err := w.socialGraphService.Get().GetFollowers(ctx, msg.ReqID, msg.UserID)
	if err != nil {
		logger.Error("error getting followers from social graph service")
		return err
	}
	uniqueIDs := make(map[int64]bool, 0)
	for _, followerID := range followersID {
		uniqueIDs[followerID] = true
	}
	for _, userMentionID := range msg.UserMentionIDs {
		uniqueIDs[userMentionID] = true
	}
	value := redis.Z {
		Member: msg.PostID,
		Score: float64(msg.Timestamp),
	}
	_, err = w.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for id := range uniqueIDs {
			idStr := strconv.FormatInt(id, 10)
			err = w.redisClient.ZAddNX(ctx, idStr, value).Err()
			if err != nil {
				return err
			}
		}
		return nil
	})
	logger.Debug("leaving write home timeline")
	return nil
}

func (w *writeHomeTimelineService) workerThread(ctx context.Context) error {
	logger := w.Logger(ctx)

	ch, conn, err := storage.RabbitMQClient(ctx, w.Config().RabbitMQAddr, w.Config().RabbitMQPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	defer conn.Close()
	defer ch.Close()

	err = ch.ExchangeDeclare("write-home-timeline", "topic", false, false, false, false, nil)
	if err != nil {
		logger.Error("error declaring exchange for rabbitmq", "msg", err.Error())
		return err
	}

	routingKey := fmt.Sprintf("write-home-timeline-%s", w.Config().Region)
	_, err = ch.QueueDeclare(routingKey, true, false, false, false, nil)
	if err != nil {
		logger.Error("error declaring queue for rabbitmq", "msg", err.Error())
		return err
	}

	err = ch.QueueBind(routingKey, routingKey, "write-home-timeline", false, nil)
	if err != nil {
		logger.Error("error binding queue for rabbitmq", "msg", err.Error())
		return err
	}

	msgs, err := ch.Consume(routingKey, "", true, false, false, false, nil)
	if err != nil {
		logger.Error("error consuming queue", "msg", err.Error())
		return err
	}

	var forever chan struct{}
	go func() {
		for msg := range msgs {
			err = w.onReceivedWorker(ctx, msg.Body)
			if err != nil {
				logger.Warn("error in worker thread", "msg", err.Error())
			}
		}
	}()
	<-forever
	return nil
}