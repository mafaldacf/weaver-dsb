package storage

import (
	"context"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClientPool struct {
	clients 	chan *amqp.Channel
	conn 		*amqp.Connection
	address 	string
	port 		int
	minSize 	int
	maxSize 	int
	currSize 	int
	mu          sync.Mutex
}

func NewRabbitMQClientPool (ctx context.Context, address string, port int, minSize int, maxSize int) (*RabbitMQClientPool, error) {
	uri := fmt.Sprintf("amqp://%s:%s@%s:%d/", "admin", "admin", address, port)
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("error establishing connection with rabbitmq: %s", err.Error())
	}

	pool := &RabbitMQClientPool{
		clients: 	make(chan *amqp.Channel),
		conn:       conn,
		address:    address,
		port:		port,
		minSize: 	minSize,
		maxSize: 	maxSize,
		currSize:   0,
	}
	return pool, nil
}

func (pool *RabbitMQClientPool) DestroyRabbitMQClientPool(ctx context.Context) error {
	return pool.conn.Close()
}

func (pool *RabbitMQClientPool) Pop(ctx context.Context) (*amqp.Channel, error) {
	// wait until we can pop a client from pool unless the context is cancelled
    select {
		case client := <-pool.clients:
			return client, nil
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout occurred while waiting to pop client from pool")
		// otherwise, we continue ahead
		default:
    }
	// create a new client if current pool size is less than max pool size
	pool.mu.Lock()
	if (pool.currSize < pool.maxSize) {
		client, err := pool.newClient(ctx)
		if err != nil {
			pool.mu.Unlock()
			return nil, fmt.Errorf("error while creating new client: %s", err.Error())
		}
		pool.mu.Unlock()
		return client, nil
	}
	pool.mu.Unlock()

	// if pool is full, we wait until a client becomes available to pop unless the context is cancelled
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout occurred while waiting to pop client from pool")
	case client := <- pool.clients:
		pool.mu.Lock()
		pool.currSize--
		pool.mu.Unlock()
		return client, nil
	}

}

func (pool *RabbitMQClientPool) Push(ch *amqp.Channel) error {
    select {
		case pool.clients <- ch:
			pool.mu.Lock()
			pool.currSize++
			pool.mu.Unlock()
		// if some unexpected error occurs, close connection
		default:
			ch.Close()
			return fmt.Errorf("could not push connection to queue")
		}
	return nil
}

func (pool *RabbitMQClientPool) newClient (ctx context.Context) (*amqp.Channel, error) {
	ch, err := pool.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("error openning channel for rabbitmq: %s", err.Error())
	}
	return ch, nil
}
