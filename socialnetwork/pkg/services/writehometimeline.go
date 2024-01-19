package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"github.com/ServiceWeaver/weaver/metrics"
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
	RabbitMQUsername string `toml:"rabbitmq_username"`
	RabbitMQPassword string `toml:"rabbitmq_password"`
	MongoDBAddr      string `toml:"mongodb_address"`
	MongoDBPort      int    `toml:"mongodb_port"`
	NumWorkers       int    `toml:"num_workers"`
	Region           string `toml:"region"`
}

type writeHomeTimelineService struct {
	weaver.Implements[WriteHomeTimelineService]
	weaver.WithConfig[writeHomeTimelineServiceOptions]
	mongoClient *mongo.Client
}

var (
	inconsistencies = metrics.NewCounter(
		"inconsistencies",
		"The number of times an cross-service inconsistency has occured",
	)
)

func (w *writeHomeTimelineService) Init(ctx context.Context) error {
	logger := w.Logger(ctx)
	logger.Debug("initializing write home timeline service...")

	var err error
	w.mongoClient, err = storage.MongoDBClient(ctx, w.Config().MongoDBAddr, w.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	logger.Info("initializing workers for WriteHomeTimelineService service", "region", w.Config().Region, "nworkers", w.Config().NumWorkers, "rabbitmq_addr", w.Config().RabbitMQAddr, "rabbitmq_port", w.Config().RabbitMQPort)
	var wg sync.WaitGroup
	wg.Add(w.Config().NumWorkers)
	for i := 1; i <= w.Config().NumWorkers; i++ {
		go func() {
			defer wg.Done()
			err := w.workerThread(ctx)
			logger.Error("error in worker thread", "msg", err.Error())
		}()
	}
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
		} else {
			logger.Error("error reading post from mongodb", "msg", err.Error())
		}
	} else {
		logger.Debug("found post! :)", "postid", post.PostID, "text", post.Text)
	}

	return nil
}

func (w *writeHomeTimelineService) workerThread(ctx context.Context) error {
	logger := w.Logger(ctx)

	ch, conn, err := storage.RabbitMQClient(ctx, w.Config().RabbitMQUsername, w.Config().RabbitMQPassword, w.Config().RabbitMQAddr, w.Config().RabbitMQPort)
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
