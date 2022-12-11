# Resource pool

Generic type safe resources pool with custom constructors and
destructors for resources.

I didn't like `sync.Pool`, because it doesn't allow to provide custom constructor.
Sometimes the default value of resource is useless and there is no way to fix this.

## Usage

```go
// import pool "github.com/posidoni/resource-pool"

conn, _ := amqp.Dial("amqp://guest:guest@localhost:5672/") // let's say this is provider of resources we want to manage

p := pool.New(
    2,             // <-- pool capacity, (-1) for unlimited pool
    3*time.Second, // <-- wait for resource for this long before getting `pool.ErrResourceUnavailable`
    func() (*amqp.Channel, error) { // constructor closure
        return conn.Channel() // <-- we are able to capture anything in this closure
    },
    func(c *amqp.Channel) { // destructor closure
        c.Close() // <-- destructors are called for each resource that pool owns
    },
    true, // preallocate go map for holding resources.
)

// calls destructor for each obj currently in pool
defer p.Cleanup()

ch, _ := conn.Channel()
p.Put(ch) // we can preallocate some resources

_, _ = p.Get()  // get preallocated resource
c, _ := p.Get() // this time pool will create resource from scratch using constructor
p.Put(c)        // in order to avoid this we need to return resources to the pool
_, _ := p.Get() //
_, err := p.Get() // This time we will not be able to get resource, because 2 resources are in use & were not returned
if errors.Is(err, pool.ErrResourceUnavailable) {
    fmt.Printf("%v", err)
}
```
