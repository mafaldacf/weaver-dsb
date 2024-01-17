package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/ServiceWeaver/weaver"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type ComposePost interface {
	ComposeAndUpload(context.Context, string) error
}

type composePost struct {
	weaver.Implements[ComposePost]
	weaver.WithConfig[composePostOptions]
	postStorage weaver.Ref[PostStorage]
	amqChannel  *amqp.Channel
}

type composePostOptions struct {
	RabbitMQAddr 		string 	 `toml:"rabbitmq_address"`
	RabbitMQPort 		int 	 `toml:"rabbitmq_port"`
	RabbitMQUsername 	string 	 `toml:"rabbitmq_username"`
	RabbitMQPassword 	string 	 `toml:"rabbitmq_password"`
	Region         		string 	 `toml:"region"`
	Regions         	[]string `toml:"regions"`
}

func (c *composePost) Init(ctx context.Context) error {
	logger := c.Logger(ctx)

	uri := fmt.Sprintf("amqp://%s:%s@%s:%d/", c.Config().RabbitMQUsername, c.Config().RabbitMQPassword, c.Config().RabbitMQAddr, c.Config().RabbitMQPort)
	conn, err := amqp.Dial(uri)
	if err != nil {
		logger.Error("error establishing connection with rabbitmq", "msg", err.Error())
		return err
	}

	c.amqChannel, err = conn.Channel()
	if err != nil {
		logger.Error("error openning channel for rabbitmq", "msg", err.Error())
		conn.Close()
		return err
	}

	logger.Info("ComposePost service running!", "region", c.Config().Region, "rabbitmq_addr", c.Config().RabbitMQAddr, "rabbitmq_port", c.Config().RabbitMQPort)
	return nil
}

func (c *composePost) ComposeAndUpload(ctx context.Context, username string) error {
	logger := c.Logger(ctx)
	logger.Info("entering ComposeAndUpload for ComposePost service", "username", username)

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	postid := rand.Int63()

	// --- Post Storage
	trace.SpanFromContext(ctx).AddEvent("remotely calling post storage",
		trace.WithAttributes(
			attribute.String("username", username),
			attribute.Int64("postid", postid),
		))

	logger.Info("remotely calling PostStorage")

	err := c.postStorage.Get().StorePost(ctx, username, postid)
	if err != nil {
		logger.Warn("error in PostStorage rpc", "msg", err.Error())
		return err
	}

	// --- Write Home Timeline
	trace.SpanFromContext(ctx).AddEvent("publishing message to rabbitmq",
		trace.WithAttributes(
			attribute.String("username", username),
			attribute.Int64("postid", postid),
		))

	logger.Info("queueing message to rabbitmq")

	err = c.amqChannel.ExchangeDeclare("write-home-timeline", "topic", false, false, false, false, nil)
	if err != nil {
		logger.Error("error declaring exchange for rabbitmq", "msg", err.Error())
		return err
	}

	msgJSON, err := json.Marshal(Message {
		PostID:   postid,
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

	logger.Info("done!")
	return err
}
