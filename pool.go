package pool

import (
	"errors"
	"sync"
	"time"
)

var ErrResourceUnavailable = errors.New("timeout while trying to fulfil request, resource unavailable")

// Represents generic pool of any resources.
//
// ResourceID generally should be int, but might be string (e.g. IP
// for network connections).
// User is responsible for cleaning up any resources.
// It is unsafe to copy pool (pass as a value in other functions).
type Pool[Resource any] struct {
	m sync.Mutex

	// If there are no resources available, client waits for this long
	// before getting error. High number may put CPU pressure due to pool
	// worker actively checking for new resource.
	// Making memory tradeoff is recommended.
	waitsForResourceFor time.Duration

	// Pool of available (idle) resources.
	idle map[int64]Resource

	requests chan Request[Resource]

	max       int64
	objsInUse int64

	factoryFn    func() (Resource, error)
	destructorFn func(Resource)

	// Notifies pool maintainer about new available resource.
	returnNotifs chan struct{}
}

type Request[T any] struct {
	e chan error
	c chan T
}

// Calls provided destructor for every entity that currently is stored
// in the pool. Objects which are taken and not returned are not subject
// to cleanup, because pool no longer owns them.
func (pool *Pool[T]) Cleanup() {
	close(pool.requests)
	for _, r := range pool.idle {
		pool.destructorFn(r)
	}
}

// New creates new pool and launches one background pool maintainer GR.
// If maxSize == -1, pool in unlimited. This means, that pool will try to reuse
// existing resources, but if there no available, creates them from scratch.
// User may choose to preallocate map inside pool. With high 'maxSize'
// this may create significant heap pressure.
func New[T any](
	maxSize int64,
	waitFor time.Duration,
	factoryFn func() (T, error),
	destructorFn func(T),
	preallocatePool bool,
) *Pool[T] {
	p := &Pool[T]{
		m:                   sync.Mutex{},
		waitsForResourceFor: waitFor,
		requests:            make(chan Request[T]),
		max:                 maxSize,
		objsInUse:           0,
		factoryFn:           factoryFn,
		destructorFn:        destructorFn,
		returnNotifs:        make(chan struct{}, 1),
	}

	if preallocatePool && maxSize != -1 {
		p.idle = make(map[int64]T, maxSize)
	} else {
		p.idle = make(map[int64]T)
	}

	go p.launchPoolMaintainer()
	return p
}

// Launches pool maintainer GR. This GR is killed when `pool.Cleanup()` is called.
// Maintains pool resources, fulfils new requests in case of full pool.
// Rejects requests for new resources if it's impossible to fulfil them in
// timely manner.
func (pool *Pool[T]) launchPoolMaintainer() {
	for req := range pool.requests {
		timeout, fulfilled := false, false
		timeoutChan := time.After(pool.waitsForResourceFor)

		for !timeout && !fulfilled {
			select {
			case <-timeoutChan:
				timeout = true
				req.e <- ErrResourceUnavailable
			case <-pool.returnNotifs:
				pool.m.Lock()
				for key, r := range pool.idle {
					delete(pool.idle, key)
					pool.m.Unlock()
					req.c <- r
					fulfilled = true
				}
			}
		}
	}
}

// Returns resource from the pool.
func (pool *Pool[T]) Get() (T, error) {
	pool.m.Lock()
	if len(pool.idle) > 0 { // (1) If pool is not empty
		for key, c := range pool.idle {
			delete(pool.idle, key)
			pool.m.Unlock()
			return c, nil
		}
	}

	// (2) If there are too many existing resources, we have request one from pool
	if pool.max != -1 && pool.objsInUse >= pool.max {
		req := Request[T]{
			c: make(chan T),
			e: make(chan error),
		}

		pool.m.Unlock()
		pool.requests <- req

		select {
		case c := <-req.c:
			return c, nil
		case e := <-req.e:
			var defaultValue T
			return defaultValue, e
		}
	}

	// (3) Otherwise, we are free to make resource
	// Increment objs in use even before creation, because we trust happy path.
	pool.objsInUse++
	pool.m.Unlock()

	resource, creationErr := pool.factoryFn()
	if creationErr != nil {
		pool.m.Lock()
		pool.objsInUse--
		pool.m.Unlock()
		var defaultValue T
		return defaultValue, creationErr
	}

	return resource, nil
}

// Puts resource back into the pool. Returns whether the object was accepted
// by the pool, which depends on provided pool capacity.
func (pool *Pool[T]) Put(resource T) bool {
	pool.m.Lock()

	if pool.max == -1 || (pool.objsInUse <= pool.max) { // If there is space in the pool
		pool.idle[pool.objsInUse] = resource
		pool.objsInUse++
		pool.m.Unlock()

		// We should notify worker only if the pool is starving
		if len(pool.requests) > 0 {
			pool.returnNotifs <- struct{}{}
		}
		return true
	}

	pool.m.Unlock()
	return false
}
