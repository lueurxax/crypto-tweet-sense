package fdb

import (
	"context"
	"errors"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/model"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

const dataKey = "data"

type requestLimiter interface {
	AddCounter(ctx context.Context, id string, window time.Duration, counterTime time.Time) error
	CleanCounters(ctx context.Context, id string, window time.Duration) error
	SetThreshold(ctx context.Context, id string, window time.Duration) error
	IncreaseThresholdTo(ctx context.Context, id string, window time.Duration, threshold uint64) error
	CheckIfExist(ctx context.Context, id string, window time.Duration) (bool, error)
	Create(ctx context.Context, id string, window time.Duration, threshold uint64) error
	GetRequestLimit(ctx context.Context, id string, window time.Duration) (common.RequestLimitData, error)
	GetRequestLimitDebug(ctx context.Context, id string, window time.Duration) (model.RequestLimits, error)
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

	counter := int32(0)
	for i, v := range el.Requests.Data {
		counter += v
		el.Requests.Data[i] = counter
	}

	if err = tx.Commit(); err != nil {
		return common.RequestLimitData{}, err
	}

	return common.RequestLimitData{
		RequestsCount: uint64(len(el.Requests.Data)),
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

	el.AddCounter(counterTime)

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	d.log.WithField("request_limits", el).Trace("add counter")

	tx.Set(d.keyBuilder.RequestLimits(id, window), data)

	return tx.Commit()
}

func (d *db) CleanCounters(ctx context.Context, id string, window time.Duration) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return errors.Join(err, ErrRequestLimitsUnmarshallingError)
	}

	if el.Requests == nil {
		el.Requests = &model.Requests{Data: make([]int32, 0)}
	}

	el.Requests = el.CleanCounters()

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.RequestLimits(id, window), data)

	// V2 write
	v2 := el.ToV2()

	dataV2, err := v2.Marshal()
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.RequestLimitsV2(id, window), dataV2)

	tx.ClearRange(d.keyBuilder.RequestsByRequestLimits(id, window))

	for _, v := range v2.Requests {
		data, err = v.Marshal()
		if err != nil {
			return err
		}

		tx.Set(d.keyBuilder.Requests(id, window, v.Start), data)
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

	if uint64(len(el.Requests.Data)) == 0 {
		return nil
	}

	el.Threshold = uint64(len(el.Requests.Data))

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.RequestLimits(id, window), data)

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

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.RequestLimits(id, window), data)

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

	if err = tx.Commit(); err != nil {
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

	el := &model.RequestLimits{
		WindowSeconds: uint64(window.Seconds()),
		Requests:      &model.Requests{Data: make([]int32, 0), Start: time.Now().Add(-window)},
		Threshold:     threshold,
	}

	data, err = el.Marshal()
	if err != nil {
		return err
	}

	tx.Set(key, data)

	return tx.Commit()
}

func (d *db) getRateLimit(tx fdbclient.Transaction, id string, window time.Duration) (*model.RequestLimits, error) {
	key := d.keyBuilder.RequestLimits(id, window)

	data, err := tx.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrRequestLimitsNotFound
	}

	el := new(model.RequestLimits)
	if err = el.Unmarshal(data); err != nil {
		d.log.WithField("key", key).WithField(dataKey, string(data)).Error(err)
		return nil, err
	}

	return el, err
}

func (d *db) GetRequestLimitDebug(ctx context.Context, id string, window time.Duration) (model.RequestLimits, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return model.RequestLimits{}, err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return model.RequestLimits{}, err
	}

	if err = tx.Commit(); err != nil {
		return model.RequestLimits{}, err
	}

	return *el, nil

}
