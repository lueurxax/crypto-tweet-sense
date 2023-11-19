package windowlimiter

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const queueLen = 10

type WindowLimiter interface {
	Start(ctx context.Context, delay int64) error
	Inc()
	TrySetThreshold(ctx context.Context, startTime time.Time) error
	Duration() time.Duration
	TooFast(ctx context.Context) (uint64, error)
	Threshold(ctx context.Context) uint64
	SetResetLimiter(resetLimiter ResetLimiter)
}

type ResetLimiter interface {
	Threshold(ctx context.Context) uint64
}

type repo interface {
	AddCounter(ctx context.Context, id string, window time.Duration, counterTime time.Time) error
	CleanCounters(ctx context.Context, id string, window time.Duration) error
	GetCounters(ctx context.Context, id string, window time.Duration) (uint64, error)
	SetThreshold(ctx context.Context, id string, window time.Duration) error
	GetThreshold(ctx context.Context, id string, window time.Duration) (uint64, error)
	CheckIfExist(ctx context.Context, id string, window time.Duration) (bool, error)
	Create(ctx context.Context, id string, window time.Duration, threshold uint64) error
	IncreaseThresholdTo(ctx context.Context, id string, duration time.Duration, threshold uint64) error
}

type limiter struct {
	id       string
	duration time.Duration

	resetLimiter  ResetLimiter
	resetDuration time.Duration

	count chan time.Time
	repo

	log log.Logger
}

func (l *limiter) Threshold(ctx context.Context) uint64 {
	threshold, err := l.repo.GetThreshold(ctx, l.id, l.duration)
	if err != nil {
		l.log.WithError(err).Error("error while getting threshold")
		return 0
	}

	return threshold
}

func (l *limiter) TrySetThreshold(ctx context.Context, startTime time.Time) error {
	if time.Since(startTime) > l.duration {
		if err := l.SetThreshold(ctx, l.id, l.duration); err != nil {
			return err
		}
	}

	return nil
}

func (l *limiter) Duration() time.Duration {
	return l.duration
}

func (l *limiter) TooFast(ctx context.Context) (uint64, error) {
	t, err := l.repo.GetThreshold(ctx, l.id, l.duration)
	if err != nil {
		return 0, err
	}

	if t == 0 {
		return 0, nil
	}

	current, err := l.GetCurrent(ctx)
	if err != nil {
		return 0, err
	}

	isFast := t-1 <= current

	if !isFast {
		return 0, nil
	}

	l.log.WithField("threshold", t).
		WithField("counter", current).
		WithField("duration", l.duration).
		Debug("checking if too fast")

	return uint64(l.duration.Seconds()) / t, nil
}

func (l *limiter) Start(ctx context.Context, delay int64) error {
	go l.loop(ctx)

	isExist, err := l.repo.CheckIfExist(ctx, l.id, l.duration)
	if err != nil {
		return err
	}

	if isExist {
		return nil
	}

	threshold := uint64(l.duration.Seconds() / float64(delay))

	return l.repo.Create(ctx, l.id, l.duration, threshold)
}

func (l *limiter) Inc() {
	l.count <- time.Now()
}

func (l *limiter) GetCurrent(ctx context.Context) (uint64, error) {
	return l.repo.GetCounters(ctx, l.id, l.duration)
}

func (l *limiter) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	resetTicker := time.NewTicker(l.resetDuration)

	l.log.WithField("duration", l.duration).Info("start loop")

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-l.count:
			l.log.WithField("time", t).Trace("inc counter")

			if err := l.repo.AddCounter(context.Background(), l.id, l.duration, t); err != nil {
				panic(err)
			}
		case <-ticker.C:
			l.log.WithField("duration", l.duration).Trace("clean counters")

			if err := l.repo.CleanCounters(context.Background(), l.id, l.duration); err != nil {
				panic(err)
			}
		case <-resetTicker.C:
			if l.resetLimiter == nil {
				continue
			}
			ctx := context.Background()
			threshold := uint64(l.duration.Seconds()) * l.resetLimiter.Threshold(ctx) / uint64(l.resetDuration.Seconds())

			if threshold == 0 {
				continue
			}

			if err := l.repo.IncreaseThresholdTo(ctx, l.id, l.duration, threshold); err != nil {
				panic(err)
			}

			l.log.WithField("duration", l.duration).Trace("reset threshold")
		}
	}
}

func (l *limiter) SetResetLimiter(resetLimiter ResetLimiter) {
	l.resetLimiter = resetLimiter
}

func NewLimiter(duration, resetDuration time.Duration, id string, repo repo, logger log.Logger) WindowLimiter {
	return &limiter{
		duration:      duration,
		count:         make(chan time.Time, queueLen),
		id:            id,
		resetDuration: resetDuration,
		repo:          repo,
		log:           logger,
	}
}
