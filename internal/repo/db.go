package fdb

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/dgraph-io/ristretto"
	"github.com/eko/gocache/lib/v4/cache"
	ristrettoStore "github.com/eko/gocache/store/ristretto/v4"
	"github.com/gotd/td/telegram"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/model"
	"github.com/sirupsen/logrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/migrations"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type DB interface {
	Migrate(ctx context.Context) error
	version
	tweetRepo
	requestLimiter
	ratingRepo
	telegram.SessionStorage
	editingTweetsRepo
	twitterAccountsRepo
}

type db struct {
	keyBuilder    keys.Builder
	db            fdbclient.Database
	requestsCache *cache.Cache[*model.RequestLimitsV2]

	log *logrus.Entry
}

func NewDB(fdb fdb.Database, log *logrus.Entry) DB {
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,
		MaxCost:     100000000,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}

	return &db{
		keyBuilder:    keys.NewBuilder(),
		db:            fdbclient.NewDatabase(fdb),
		requestsCache: cache.New[*model.RequestLimitsV2](ristrettoStore.NewRistretto(ristrettoCache)),
		log:           log,
	}
}

func (d *db) Migrate(ctx context.Context) error {
	v, err := d.GetVersion(ctx)
	if err != nil {
		return err
	}

	d.log.WithField("version", v).Info("current version")

	m := migrations.Migrations(v)
	for _, el := range m {
		d.log.WithField("version", el.Version()).Info("migrating to version")

		tr, err := d.db.NewTransaction(ctx)
		if err != nil {
			return err
		}

		if err = el.Up(ctx, tr); err != nil {
			return err
		}

		if err = d.WriteVersion(ctx, el.Version()); err != nil {
			return err
		}

		if err = tr.Commit(); err != nil {
			return err
		}

		d.log.WithField("version", el.Version()).Info("migrated to version")
	}

	return nil
}

func (d *db) migratePrefix(ctx context.Context, prefix string, prefix2 keys.Prefix) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	pr, err := fdb.PrefixRange([]byte(prefix))
	if err != nil {
		return err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return err
	}

	if err = tr.Commit(); err != nil {
		return err
	}

	for _, kv := range kvs {
		tr, err = d.db.NewTransaction(ctx)
		if err != nil {
			return err
		}

		key := append(prefix2[:], kv.Key[len(prefix):]...)

		tr.Set(key, kv.Value)
		tr.Clear(kv.Key)

		if err = tr.Commit(); err != nil {
			return err
		}
	}

	return nil
}
