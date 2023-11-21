package fdb

import (
	"context"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type requestLimiter interface {
	AddCounter(ctx context.Context, id string, window time.Duration, counterTime time.Time) error
	CleanCounters(ctx context.Context, id string, window time.Duration) error
	SetThreshold(ctx context.Context, id string, window time.Duration) error
	IncreaseThresholdTo(ctx context.Context, id string, window time.Duration, threshold uint64) error
	CheckIfExist(ctx context.Context, id string, window time.Duration) (bool, error)
	Create(ctx context.Context, id string, window time.Duration, threshold uint64) error
	GetRequestLimit(ctx context.Context, id string, window time.Duration) (common.RequestLimitData, error)
}

func (d *db) GetRequestLimit(ctx context.Context, id string, window time.Duration) (common.RequestLimitData, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return common.RequestLimitData{}, err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return common.RequestLimitData{}, err
	}

	return common.RequestLimitData{
		RequestsCount: uint64(len(el.Requests)),
		Threshold:     el.Threshold,
	}, nil
}

func (d *db) AddCounter(ctx context.Context, id string, window time.Duration, counterTime time.Time) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return err
	}

	el.Requests = append(el.Requests, counterTime)

	data, err := jsoniter.Marshal(el)
	if err != nil {
		return err
	}

	d.log.WithField("request_limits", el).Trace("add counter")

	if err = tx.Set(d.keyBuilder.RequestLimits(id, window), data); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *db) CleanCounters(ctx context.Context, id string, window time.Duration) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return err
	}

	requests := make([]time.Time, 0, len(el.Requests))

	for _, key := range el.Requests {
		if time.Since(key) < window {
			requests = append(requests, key)
		}
	}

	el.Requests = requests
	el.CurrentRequests = nil

	data, err := jsoniter.Marshal(el)
	if err != nil {
		return err
	}

	if err = tx.Set(d.keyBuilder.RequestLimits(id, window), data); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *db) SetThreshold(ctx context.Context, id string, window time.Duration) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return err
	}

	if uint64(len(el.Requests)) == 0 {
		return nil
	}

	el.Threshold = uint64(len(el.Requests))

	data, err := jsoniter.Marshal(el)
	if err != nil {
		return err
	}

	if err = tx.Set(d.keyBuilder.RequestLimits(id, window), data); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *db) IncreaseThresholdTo(ctx context.Context, id string, window time.Duration, threshold uint64) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return err
	}

	if el.Threshold > threshold {
		return nil
	}

	d.log.
		WithField("id", id).
		WithField("duration", int(window.Seconds())).
		WithField("old_threshold", el.Threshold).
		WithField("new_threshold", threshold).
		Debug("increase threshold")

	el.Threshold = threshold

	data, err := jsoniter.Marshal(el)
	if err != nil {
		return err
	}

	if err = tx.Set(d.keyBuilder.RequestLimits(id, window), data); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *db) CheckIfExist(ctx context.Context, id string, window time.Duration) (bool, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return false, err
	}

	key := d.keyBuilder.RequestLimits(id, window)

	data, err := tx.Get(key)
	if err != nil {
		return false, err
	}

	return data != nil, nil
}

func (d *db) Create(ctx context.Context, id string, window time.Duration, threshold uint64) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	key := d.keyBuilder.RequestLimits(id, window)

	data, err := tx.Get(key)
	if err != nil {
		return err
	}

	if data != nil {
		return ErrAlreadyExists
	}

	el := &common.RequestLimits{
		WindowSeconds: uint64(window.Seconds()),
		Requests:      make([]time.Time, 0),
		Threshold:     threshold,
	}

	data, err = jsoniter.Marshal(el)
	if err != nil {
		return err
	}

	if err = tx.Set(key, data); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *db) getRateLimit(tx fdbclient.Transaction, id string, window time.Duration) (*common.RequestLimits, error) {
	key := d.keyBuilder.RequestLimits(id, window)

	data, err := tx.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrRequestLimitsNotFound
	}

	el := new(common.RequestLimits)
	if err = jsoniter.Unmarshal(data, el); err != nil {
		return nil, err
	}

	return el, nil
}
