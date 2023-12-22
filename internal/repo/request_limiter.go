package fdb

import (
	"context"
	"errors"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"

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
	GetRequestLimitDebug(ctx context.Context, id string, window time.Duration) (model.RequestLimitsV2, error)
}

func (d *db) GetRequestLimit(ctx context.Context, id string, window time.Duration) (common.RequestLimitData, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return common.RequestLimitData{}, err
	}

	el, err := d.getRateLimitLite(tx, id, window)
	if err != nil {
		return common.RequestLimitData{}, err
	}

	if err = tx.Commit(); err != nil {
		return common.RequestLimitData{}, err
	}

	return common.RequestLimitData{
		RequestsCount: uint64(el.RequestsCount),
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

	tx.Set(d.keyBuilder.RequestLimits(id, window), data)

	data, err = el.Requests[len(el.Requests)-1].Marshal()
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.Requests(id, window, el.Requests[len(el.Requests)-1].Start), data)

	d.log.WithField("request_limits", el).Trace("add counter")

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
		el.Requests = make([]model.RequestsV2, 0)
	}

	deleteMap := make(map[time.Time]struct{})

	for _, v := range el.Requests {
		deleteMap[v.Start] = struct{}{}
	}

	requests := el.CleanCounters()

	insert := make([]model.RequestsV2, 0, len(requests))

	for _, v := range requests {
		if _, ok := deleteMap[v.Start]; ok {
			delete(deleteMap, v.Start)
		} else {
			insert = append(insert, v)
		}
	}

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.RequestLimits(id, window), data)

	for v := range deleteMap {
		tx.Clear(d.keyBuilder.Requests(id, window, v))
	}

	for _, v := range insert {
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

	el, err := d.getRateLimitLite(tx, id, window)
	if err != nil {
		return err
	}

	if el.RequestsCount == 0 {
		return nil
	}

	el.Threshold = uint64(el.RequestsCount)

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

	el, err := d.getRateLimitLite(tx, id, window)
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

	el := &model.RequestLimitsV2{
		WindowSeconds: uint64(window.Seconds()),
		Threshold:     threshold,
	}

	data, err = el.Marshal()
	if err != nil {
		return err
	}

	tx.Set(key, data)

	return tx.Commit()
}

func (d *db) getRateLimit(tx fdbclient.Transaction, id string, window time.Duration) (*model.RequestLimitsV2, error) {
	key := d.keyBuilder.RequestLimits(id, window)

	data, err := tx.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrRequestLimitsNotFound
	}

	el := new(model.RequestLimitsV2)
	if err = el.Unmarshal(data); err != nil {
		d.log.WithField("key", key).WithField(dataKey, string(data)).Error(err)
		return nil, err
	}

	pr, err := fdb.PrefixRange(d.keyBuilder.RequestsByRequestLimits(id, window))
	if err != nil {
		return nil, err
	}

	kvs, err := tx.GetRange(pr)
	if err != nil {
		return nil, err
	}

	for _, kv := range kvs {
		r := new(model.RequestsV2)
		if err = r.Unmarshal(kv.Value); err != nil {
			return nil, err
		}

		el.Requests = append(el.Requests, *r)
	}

	return el, err
}

func (d *db) getRateLimitLite(tx fdbclient.Transaction, id string, window time.Duration) (*model.RequestLimitsV2, error) {
	key := d.keyBuilder.RequestLimits(id, window)

	data, err := tx.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrRequestLimitsNotFound
	}

	el := new(model.RequestLimitsV2)
	if err = el.Unmarshal(data); err != nil {
		d.log.WithField("key", key).WithField(dataKey, string(data)).Error(err)
		return nil, err
	}

	return el, err
}

func (d *db) GetRequestLimitDebug(ctx context.Context, id string, window time.Duration) (model.RequestLimitsV2, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return model.RequestLimitsV2{}, err
	}

	el, err := d.getRateLimit(tx, id, window)
	if err != nil {
		return model.RequestLimitsV2{}, err
	}

	if err = tx.Commit(); err != nil {
		return model.RequestLimitsV2{}, err
	}

	return *el, nil

}
