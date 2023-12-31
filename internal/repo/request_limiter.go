package fdb

import (
	"context"
	"errors"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/eko/gocache/lib/v4/store"

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
	GetRequestLimitDebug(ctx context.Context, id string, window time.Duration) (model.RequestLimitsV2Debug, error)
}

func (d *db) GetRequestLimit(ctx context.Context, id string, window time.Duration) (common.RequestLimitData, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return common.RequestLimitData{}, err
	}

	el, err := d.getRateLimit(ctx, tx, id, window)
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

	el, err := d.getRateLimit(ctx, tx, id, window)
	if err != nil {
		return err
	}

	el.AddCounter(counterTime)

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	key := d.keyBuilder.RequestLimits(id, window)

	tx.Set(key, data)

	data, err = el.Requests[len(el.Requests)-1].Marshal()
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.Requests(id, window, el.Requests[len(el.Requests)-1].Start), data)

	if err = d.requestsCache.Set(ctx, key, el); err != nil {
		d.log.WithError(err).Error("set cache error")
	}

	d.log.WithField("request_limits", el).Trace("add counter")

	return tx.Commit()
}

func (d *db) CleanCounters(ctx context.Context, id string, window time.Duration) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(ctx, tx, id, window)
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

	el.RequestsCount = 0

	for _, v := range el.Requests {
		el.RequestsCount += uint32(len(v.Data))
	}

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	key := d.keyBuilder.RequestLimits(id, window)

	tx.Set(key, data)

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

	el.Requests = requests

	if err = d.requestsCache.Set(ctx, key, el); err != nil {
		d.log.WithError(err).Error("set cache error")
	}

	return tx.Commit()
}

func (d *db) SetThreshold(ctx context.Context, id string, window time.Duration) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(ctx, tx, id, window)
	if err != nil {
		return err
	}

	if el.RequestsCount == 0 {
		d.log.WithField("id", id).WithField("duration", int(window.Seconds())).Debug("can't set threshold, no requests")
		return nil
	}

	el.Threshold = uint64(el.RequestsCount)

	data, err := el.Marshal()
	if err != nil {
		return err
	}

	key := d.keyBuilder.RequestLimits(id, window)

	tx.Set(key, data)

	if err = d.requestsCache.Set(ctx, key, el); err != nil {
		d.log.WithError(err).Error("set cache error")
	}

	return tx.Commit()
}

func (d *db) IncreaseThresholdTo(ctx context.Context, id string, window time.Duration, threshold uint64) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	el, err := d.getRateLimit(ctx, tx, id, window)
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

	key := d.keyBuilder.RequestLimits(id, window)

	tx.Set(key, data)

	if err = d.requestsCache.Set(ctx, key, el); err != nil {
		d.log.WithError(err).Error("set cache error")
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

func (d *db) getRateLimit(ctx context.Context, tx fdbclient.Transaction, id string, window time.Duration) (*model.RequestLimitsV2, error) {
	key := d.keyBuilder.RequestLimits(id, window)

	el, err := d.requestsCache.Get(ctx, key)
	if err == nil {
		return el, nil
	}

	if !errors.Is(err, store.NotFound{}) {
		return nil, err
	}

	data, err := tx.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrRequestLimitsNotFound
	}

	el = new(model.RequestLimitsV2)
	if err = el.Unmarshal(data); err != nil {
		d.log.WithField("key", key).WithField(dataKey, string(data)).Error(err)
		return nil, err
	}

	el.Requests = make([]model.RequestsV2, 0)

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

	if err = d.requestsCache.Set(ctx, key, el); err != nil {
		d.log.WithError(err).Error("set cache error")
	}

	return el, err
}

func (d *db) GetRequestLimitDebug(ctx context.Context, id string, window time.Duration) (model.RequestLimitsV2Debug, error) {
	result := model.RequestLimitsV2Debug{Requests: make([]model.RequestsV2, 0)}

	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return result, err
	}

	el, err := d.getRateLimit(ctx, tx, id, window)
	if err != nil {
		return result, err
	}

	result.RequestsCount = el.RequestsCount
	result.WindowSeconds = el.WindowSeconds
	result.Threshold = el.Threshold

	pr, err := fdb.PrefixRange(d.keyBuilder.RequestsByRequestLimits(id, window))
	if err != nil {
		return result, err
	}

	kvs, err := tx.GetRange(pr)
	if err != nil {
		return result, err
	}

	d.log.WithField("elements", len(kvs)).Info("requests batches")

	for _, kv := range kvs {
		r := new(model.RequestsV2)
		if err = r.Unmarshal(kv.Value); err != nil {
			return result, err
		}

		result.Requests = append(result.Requests, *r)
	}

	if err = tx.Commit(); err != nil {
		return result, err
	}

	return result, nil
}
