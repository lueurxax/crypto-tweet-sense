package tweetfinder

import (
	"context"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type metricMiddleware struct {
	login string
	next  Finder

	findAllRequestsHistogramSeconds *prometheus.HistogramVec
	findRequestsHistogramSeconds    *prometheus.HistogramVec
}

func (m *metricMiddleware) Init(context.Context) error {
	return nil
}

func (m *metricMiddleware) FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error) {
	st := time.Now()

	data, err := m.next.FindAll(ctx, start, end, search)

	m.findAllRequestsHistogramSeconds.WithLabelValues(m.login, search, strconv.FormatBool(err != nil)).Observe(time.Since(st).Seconds())

	return data, err
}

func (m *metricMiddleware) Find(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	st := time.Now()

	data, err := m.next.Find(ctx, id)

	m.findRequestsHistogramSeconds.WithLabelValues(m.login, strconv.FormatBool(err != nil)).Observe(time.Since(st).Seconds())

	return data, err
}

func (m *metricMiddleware) CurrentDelay() int64 {
	return m.next.CurrentDelay()
}

func NewMetricMiddleware(all, one *prometheus.HistogramVec, login string, next Finder) Finder {
	return &metricMiddleware{
		login:                           login,
		next:                            next,
		findAllRequestsHistogramSeconds: all,
		findRequestsHistogramSeconds:    one,
	}
}
