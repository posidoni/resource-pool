package main

import (
	"log"
	"time"

	pool "github.com/posidoni/resource-pool"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Println("Error while connecting to RMQ, is RMQ running?")
	}
	defer conn.Close()

	p := pool.New(
		5,
		3*time.Second,
		func() (*amqp.Channel, error) {
			log.Println("Creating new channel")
			return conn.Channel()
		},
		func(c *amqp.Channel) {
			log.Println("Cleaning up channel")
			c.Close()
		},
		true,
	)
	defer p.Cleanup()

	for i := 0; i < 5; i++ {
		ch, err := conn.Channel()
		if err != nil {
			continue
		}
		hasPut := p.Put(ch)
		if hasPut {
			log.Println("Put channel into the pool")
		}
	}
}

// Pool contains mutex, thus pool may be passed only by pointer.
func PoolUser(p *pool.Pool[*amqp.Channel]) {
	_, _ = p.Get()
	_, _ = p.Get()
	_, _ = p.Get()
}
