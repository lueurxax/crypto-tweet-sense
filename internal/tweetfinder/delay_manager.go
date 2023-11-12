package tweetfinder

import (
	"math/rand"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const delayKey = "delay"

type Manager interface {
	TooManyRequests()
	ProcessedBatchOfTweets()
	ProcessedQuery()
	SetSetterFn(func(seconds int64))
}

type manager struct {
	setter              func(seconds int64)
	delay, minimalDelay int64

	log log.Logger
}

func (m *manager) TooManyRequests() {
	m.log.WithField(delayKey, m.delay).Error("too many requests")
	m.minimalDelay = m.delay - 1
	m.incRandomDelay()
}

func (m *manager) ProcessedBatchOfTweets() {
	m.decDelay()
}

func (m *manager) ProcessedQuery() {
	m.decDelay()

	if m.minimalDelay > 4 {
		m.minimalDelay--
	}
}

func (m *manager) SetSetterFn(fn func(seconds int64)) {
	m.setter = fn
}

func (m *manager) decDelay() {
	if m.delay <= m.minimalDelay {
		return
	}
	m.delay--
	m.setter(m.delay)
	m.log.WithField(delayKey, m.delay).Debug("delay decreased")
}

func (m *manager) incRandomDelay() {
	if m.delay == 0 {
		m.delay = 1
	}

	m.delay += rand.Int63n(m.delay+1-m.minimalDelay/2) + 1 //nolint:gosec
	m.setter(m.delay)
	m.log.WithField(delayKey, m.delay).Debug("delay increased")
}

func NewDelayManager(setter func(seconds int64), minimalDelay int64, logger log.Logger) Manager {
	return &manager{setter: setter, minimalDelay: minimalDelay, delay: minimalDelay, log: logger}
}
