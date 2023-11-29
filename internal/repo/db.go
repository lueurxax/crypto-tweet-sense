package fdb

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/gotd/td/telegram"
	"github.com/sirupsen/logrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type DB interface {
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
