package metrics

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/prometheus/client_golang/prometheus"
)

const countLabel = "count"

type repo interface {
	Count(ctx context.Context) (uint32, error)
}

type Metrics interface {
	Start(ctx context.Context)
}

type metrics struct {
	repo
	currentProcessingTweets *prometheus.GaugeVec

	log log.Logger
}

func (m *metrics) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 10) //nolint:gomnd
	for range ticker.C {
		select {
		case <-ctx.Done():
			return
		default:
			count, err := m.repo.Count(ctx)
			if err != nil {
				m.log.WithError(err).Error(countLabel)

				continue
			}

			m.log.WithField(countLabel, count).Trace("current processing tweets")

			m.currentProcessingTweets.WithLabelValues().Set(float64(count))
		}
	}
}

func NewMetrics(currentProcessingTweets *prometheus.GaugeVec, repo repo, logger log.Logger) Metrics {
	return &metrics{currentProcessingTweets: currentProcessingTweets, repo: repo, log: logger}
}
