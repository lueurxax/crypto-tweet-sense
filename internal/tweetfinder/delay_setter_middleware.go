package tweetfinder

import "github.com/prometheus/client_golang/prometheus"

type delaySetter struct {
	setter func(seconds int64)

	login string

	metric *prometheus.GaugeVec
}

func (d *delaySetter) Set(seconds int64) {
	d.setter(seconds)
	d.metric.WithLabelValues(d.login).Set(float64(seconds))
}

func newDelaySetter(setter func(seconds int64), metric *prometheus.GaugeVec, login string) *delaySetter {
	return &delaySetter{setter: setter, metric: metric, login: login}
}
