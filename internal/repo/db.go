package fdb

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/gotd/td/telegram"
	"github.com/sirupsen/logrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
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
	keyBuilder keys.Builder
	db         fdbclient.Database

	log *logrus.Entry
}

func NewDB(fdb fdb.Database, log *logrus.Entry) DB {
	return &db{
		keyBuilder: keys.NewBuilder(),
		db:         fdbclient.NewDatabase(fdb),
		log:        log,
	}
}

func (d *db) Migrate(ctx context.Context) error {
	for oldPrefix, newPrefix := range keys.OldToNewPrefixes {
		if err := d.migratePrefix(ctx, oldPrefix, newPrefix); err != nil {
			return err
		}
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

	for _, kv := range kvs {
		tr.Clear(kv.Key)
	}

	return tr.Commit()
}
