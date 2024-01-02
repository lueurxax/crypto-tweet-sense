package checkcleanpool

import (
	"context"
	"sync"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
)

const workersCount = 100

type Worker interface {
	CheckTweetOrClear(ctx context.Context, key fdb.Key, id string)
}

type Pool interface {
	Worker
	Start()
	Stop()
}

type pool struct {
	worker Worker
	ch     chan object
	wg     sync.WaitGroup
}

type object struct {
	ctx context.Context
	key fdb.Key
	id  string
}

func (p *pool) Start() {
	for i := 0; i < workersCount; i++ {
		p.wg.Add(1)
		go p.loop()
	}
}

func (p *pool) Stop() {
	close(p.ch)
	p.wg.Wait()
}

func (p *pool) CheckTweetOrClear(ctx context.Context, key fdb.Key, id string) {
	select {
	case <-ctx.Done():
		return
	case p.ch <- object{ctx: ctx, key: key, id: id}:
	}
}

func (p *pool) loop() {
	for obj := range p.ch {
		p.worker.CheckTweetOrClear(obj.ctx, obj.key, obj.id)
	}
	p.wg.Done()
}

func NewPool(worker Worker) Pool {
	return &pool{worker: worker, ch: make(chan object, 1000), wg: sync.WaitGroup{}}
}
