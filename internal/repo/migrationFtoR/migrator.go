package migrations

import (
	"context"
	"encoding/binary"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
	"github.com/redis/go-redis/v9"
)

const versionKey = "version"

type Migrator interface {
	Migrate(ctx context.Context) error
}

type migrator struct {
	keyBuilder keys.Builder

	fdb fdbclient.Database
	rdb *redis.Client

	log log.Logger
}

func (m *migrator) Migrate(ctx context.Context) error {
	v, err := m.GetVersion(ctx)
	if err != nil {
		return err
	}

	m.log.WithField(versionKey, v).Info("current version")

	mig := Migrations(v)
	for _, el := range mig {
		m.log.WithField(versionKey, el.Version()).Info("migrating to version")

		tr, err := m.fdb.NewTransaction(ctx)
		if err != nil {
			return err
		}

		tx := m.rdb.TxPipeline()

		if err = el.Up(ctx, tr, tx); err != nil {
			return err
		}

		if err = m.WriteVersion(ctx, el.Version()); err != nil {
			return err
		}

		if err = tr.Commit(); err != nil {
			return err
		}

		if _, err = tx.Exec(ctx); err != nil {
			return err
		}

		m.log.WithField(versionKey, el.Version()).Info("migrated to version")
	}

	return nil
}

func (m *migrator) GetVersion(ctx context.Context) (uint32, error) {
	tr, err := m.fdb.NewTransaction(ctx)
	if err != nil {
		return 0, err
	}

	data, err := tr.Get(m.keyBuilder.Version())
	if err != nil {
		return 0, err
	}

	if data == nil {
		return 0, nil
	}

	return binary.BigEndian.Uint32(data), nil
}

func (m *migrator) WriteVersion(ctx context.Context, version uint32) error {
	tr, err := m.fdb.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data := make([]byte, binary.Size(version))
	binary.BigEndian.PutUint32(data, version)

	tr.Set(m.keyBuilder.Version(), data)

	return tr.Commit()
}

func NewMigrator(fdb fdb.Database, rdb *redis.Client, log log.Logger) Migrator {
	return &migrator{
		keyBuilder: keys.NewBuilder(),
		fdb:        fdbclient.NewDatabase(fdb),
		rdb:        rdb,
		log:        log,
	}
}
