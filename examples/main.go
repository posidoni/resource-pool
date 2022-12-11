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
		5,             // <-- pool capacity
		3*time.Second, // <-- wait for resource for this long before getting `pool.ErrResourceUnavailable`
		func() (*amqp.Channel, error) {
			return conn.Channel() // <-- we are able to capture anything in constructor
		},
		func(c *amqp.Channel) {
			c.Close() // <-- destructors are called for each resource that pool owns
		},
		true,
	)

	// calls destructor for each obj currently in pool
	defer p.Cleanup()

	ch, err := conn.Channel()
	p.Put(ch) // we can preallocate some resources

	_, _ = p.Get()  // get preallocated resources
	c, _ := p.Get() // this time pool will create resource from scratch
	p.Put(c)        // in order to avoid this we need to return resources to the pool
}
