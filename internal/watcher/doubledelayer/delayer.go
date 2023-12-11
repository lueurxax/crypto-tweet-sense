package doubledelayer

import "time"

type DoubleDelayer interface {
	Duration(id string) time.Duration
}

type delayer struct {
	last string

	cold time.Duration
	hot  time.Duration
}

func (d *delayer) Duration(id string) time.Duration {
	if id == d.last {
		return d.hot
	}

	d.last = id

	return d.cold
}

func NewDelayer(hot, cold time.Duration) DoubleDelayer {
	return &delayer{hot: hot, cold: cold}
}
