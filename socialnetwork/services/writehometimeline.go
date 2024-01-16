package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ServiceWeaver/weaver"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Message struct {
	PostID int64 `json:"postid"`
}

type WriteHomeTimeline interface {
	// WriteHomeTimeline service does not expose any rpc methods
}

type writeHomeTimelineOptions struct {
	RabbitMQAddr     string `toml:"rabbitmq_address"`
	RabbitMQPort     int    `toml:"rabbitmq_port"`
	RabbitMQUsername string `toml:"rabbitmq_username"`
	RabbitMQPassword string `toml:"rabbitmq_password"`
	MongoDBAddr      string `toml:"mongodb_address"`
	MongoDBPort      int    `toml:"mongodb_port"`
	NumWorkers       int    `toml:"num_workers"`
	Region           string `toml:"region"`
}

type writeHomeTimeline struct {
	weaver.Implements[WriteHomeTimeline]
	weaver.WithConfig[writeHomeTimelineOptions]
	mongoClient *mongo.Client
}

func (w *writeHomeTimeline) Init(ctx context.Context) error {
	logger := w.Logger(ctx)

	uri := fmt.Sprintf("mongodb://%s:%d/?directConnection=true", w.Config().MongoDBAddr, w.Config().MongoDBPort)
	clientOptions := options.Client().ApplyURI(uri)

	var err error
	w.mongoClient, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		logger.Error("error connecting to mongodb", "msg", err.Error())
		return err
	}
	err = w.mongoClient.Ping(ctx, nil)
	if err != nil {
		logger.Error("error validating connecting to mongodb", "msg", err.Error())
		return err
	}

	logger.Info("initializing workers for WriteHomeTimeline service", "region", w.Config().Region, "nworkers", w.Config().NumWorkers, "rabbitmq_addr", w.Config().RabbitMQAddr, "rabbitmq_port", w.Config().RabbitMQPort)
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

func (w *writeHomeTimeline) onReceivedWorker(ctx context.Context, body []byte) error {
	logger := w.Logger(ctx)

	var msg Message
	err := json.Unmarshal(body, &msg)
	if err != nil {
		logger.Error("error parsing json message", "msg", err.Error())
		return err
	}

	logger.Debug("received rabbitmq message", "postid", msg.PostID)

	trace.SpanFromContext(ctx).AddEvent("reading post",
		trace.WithAttributes(
			attribute.Int64("postid", msg.PostID),
		))

	db := w.mongoClient.Database("poststorage")
	collection := db.Collection("posts")

	var post Post
	filter := bson.D{{Key: "postid", Value: msg.PostID}}
	err = collection.FindOne(ctx, filter, nil).Decode(&post)
	if err != nil {
		logger.Error("error reading post from mongodb", "msg", err.Error())
		return err
	}

	logger.Debug("found post! :)", "postid", post.PostID, "username", post.Username)

	return nil
}

func (w *writeHomeTimeline) workerThread(ctx context.Context) error {
	logger := w.Logger(ctx)

	uri := fmt.Sprintf("amqp://%s:%s@%s:%d/", w.Config().RabbitMQUsername, w.Config().RabbitMQPassword, w.Config().RabbitMQAddr, w.Config().RabbitMQPort)
	conn, err := amqp.Dial(uri)
	if err != nil {
		logger.Error("error establishing connection with rabbitmq", "msg", err.Error())
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		logger.Error("error openning channel for rabbitmq", "msg", err.Error())
		return err
	}
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
