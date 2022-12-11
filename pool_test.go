package pool_test

import (
	"log"
	"sync/atomic"
	"testing"
	"time"

	pool "github.com/posidoni/resource-pool"
	"github.com/stretchr/testify/require"
)

func TestPool(t *testing.T) {
	t.Parallel()
	type R struct{ a, b, c, d int }

	t.Run(
		"When there are no objects in the pool, pool creates object from scratch with the given CTR callback",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			pool := pool.New(
				1,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {},
				true,
			)
			r, err := pool.Get()
			if err != nil {
				t.Errorf("Unexpected error while creating obj:%v", err)
			}
			require.Equal(t, int64(1), ctrCalls)
			require.Equal(t, R{1, 2, 3, 4}, r)
		})

	t.Run(
		"When there are available objects in pool, pool returns them without building from scratch with CTR",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			pool := pool.New(
				1,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {},
				true,
			)
			pool.Put(R{5, 5, 5, 5})
			r, err := pool.Get()
			if err != nil {
				t.Errorf("Unexpected error while creating obj:%v", err)
			}
			require.Equal(t, int64(0), ctrCalls)
			require.Equal(t, R{5, 5, 5, 5}, r)
		})

	t.Run(
		"When asked to create more objects than allowed, pool does not create them and waits for old objects to be put back",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			pool := pool.New(
				1,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {},
				true,
			)
			r, err := pool.Get()
			_, _ = pool.Get()
			_, _ = pool.Get()
			if err != nil {
				t.Errorf("Unexpected error while creating obj:%v", err)
			}
			require.Equal(t, int64(1), ctrCalls)
			require.Equal(t, R{1, 2, 3, 4}, r)
		})

	t.Run(
		"When there are available old objects in the pool, pool gives them and doesn't call the constructor",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			dstrCall := int64(0)
			pool := pool.New(
				1,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {
					atomic.AddInt64(&dstrCall, 1)
				},
				true,
			)
			pool.Put(R{5, 5, 5, 5}) // l1
			r, _ := pool.Get()      // 0
			pool.Put(R{5, 5, 5, 5})
			_, _ = pool.Get()
			pool.Put(R{5, 5, 5, 5})
			_, _ = pool.Get()
			require.Equal(t, int64(0), ctrCalls)
			require.Equal(t, int64(0), dstrCall)
			require.Equal(t, R{5, 5, 5, 5}, r)
		})

	t.Run(
		"When pool is unable to create new object before timeout, pool returns error",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			dstrCall := int64(0)
			pool := pool.New(
				1,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {
					atomic.AddInt64(&dstrCall, 1)
				},
				true,
			)
			pool.Put(R{5, 5, 5, 5})
			_, _ = pool.Get()
			_, err := pool.Get()

			require.Equal(t, int64(0), ctrCalls, "Constructor was called, but should not")
			require.Equal(t, int64(0), dstrCall, "Destructor was called, but should not")
			require.Error(t, err)
		})

	t.Run(
		"When pool is full, it refuses to consume new elements (so the users must clean these things up themself)",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			dstrCall := int64(0)
			pool := pool.New(
				1,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {
					atomic.AddInt64(&dstrCall, 1)
					log.Println("DSTR")
				},
				true,
			)
			pool.Put(R{5, 5, 5, 5})
			_, _ = pool.Get()
			_, err := pool.Get()

			pool.Put(R{5, 5, 5, 5})

			require.False(t, pool.Put(R{5, 5, 5, 5}))
			require.False(t, pool.Put(R{5, 5, 5, 5}))
			require.False(t, pool.Put(R{5, 5, 5, 5}))

			require.Equal(t, int64(0), ctrCalls, "Constructor was called, but should not")
			require.Equal(t, int64(0), dstrCall, "Destructor was called, but should not")
			require.Error(t, err)
		})

	t.Run(
		"When `Cleanup` is called, pools calls destructors for all elements present in pool",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			dstrCall := int64(0)
			pool := pool.New(
				5,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {
					atomic.AddInt64(&dstrCall, 1)
				},
				true,
			)
			pool.Put(R{5, 5, 5, 5})
			pool.Put(R{5, 5, 5, 5})
			pool.Put(R{5, 5, 5, 5})
			pool.Put(R{5, 5, 5, 5})
			pool.Put(R{5, 5, 5, 5})

			pool.Cleanup()

			require.Equal(t, int64(0), ctrCalls, "Constructor was called, but should not")
			require.Equal(t, int64(5), dstrCall, "Destructor was called, but should not")
		})

	t.Run(
		"When pool is empty (all objects are held by users), pool doesn't call provided destructor, even if the pool itself created all objects",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			dstrCall := int64(0)
			pool := pool.New(
				5,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {
					atomic.AddInt64(&dstrCall, 1)
				},
				true,
			)
			_, _ = pool.Get()
			_, _ = pool.Get()
			_, _ = pool.Get()

			pool.Cleanup()

			require.Equal(t, int64(3), ctrCalls, "Constructor was called, but should not")
			require.Equal(t, int64(0), dstrCall, "Destructor was called, but should not")
		})

	t.Run(
		"When the pool is unlimited, it still may return available idle objects",
		func(t *testing.T) {
			t.Parallel()
			ctrCalls := int64(0)
			dstrCall := int64(0)
			pool := pool.New(
				-1,
				100*time.Millisecond,
				func() (R, error) {
					atomic.AddInt64(&ctrCalls, 1)
					return R{1, 2, 3, 4}, nil
				},
				func(r R) {
					atomic.AddInt64(&dstrCall, 1)
				},
				true,
			)

			pool.Put(R{5, 5, 5, 5})
			r, _ := pool.Get()
			_, _ = pool.Get()
			_, _ = pool.Get()

			pool.Cleanup()

			require.Equal(t, int64(2), ctrCalls, "Constructor was called, but should not")
			require.Equal(t, int64(0), dstrCall, "Destructor was called, but should not")
			require.Equal(t, R{5, 5, 5, 5}, r)
		})
}
