package windowcounter

import (
	"context"
	"time"
)

const queueLen = 10

type WindowCounter interface {
	Inc()
	GetCurrent() uint64
	Start(ctx context.Context)
}

type counter struct {
	window time.Duration

	count    chan time.Time
	counters map[time.Time]struct{}
}

func (c *counter) Start(ctx context.Context) {
	go c.loop(ctx)
}

func (c *counter) Inc() {
	c.count <- time.Now()
}

func (c *counter) GetCurrent() uint64 {
	return uint64(len(c.counters))
}

func (c *counter) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for key := range c.counters {
				if time.Since(key) > c.window {
					delete(c.counters, key)
				}
			}
		case t := <-c.count:
			c.counters[t] = struct{}{}
		}
	}
}

func NewCounter(window time.Duration) WindowCounter {
	return &counter{
		window:   window,
		count:    make(chan time.Time, queueLen),
		counters: make(map[time.Time]struct{}),
	}
}
