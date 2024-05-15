package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	nested "github.com/antonfisher/nested-logrus-formatter"
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/kelseyhightower/envconfig"
	repo "github.com/lueurxax/crypto-tweet-sense/internal/repo/redis"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	ratingCollector "github.com/lueurxax/crypto-tweet-sense/internal/ratingcollector"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
)

var version = "dev"

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
)

type config struct {
	LoggerLevel  logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs     bool         `envconfig:"LOG_TO_ECS" default:"false"`
	ChannelID    int64        `envconfig:"CHANNEL_ID" required:"true"`
	AppID        int          `envconfig:"APP_ID" required:"true"`
	AppHash      string       `envconfig:"APP_HASH" required:"true"`
	Phone        string       `envconfig:"PHONE" required:"true"`
	DatabasePath string       `default:"/usr/local/etc/foundationdb/fdb.cluster"`
	RedisAddress string       `default:"localhost:6379"`
}

func main() {
	printVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		return
	}

	// init main config
	cfg := new(config)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	// init logger
	logrusLogger := logrus.New()
	logrusLogger.SetLevel(cfg.LoggerLevel)
	logrusLogger.SetFormatter(&nested.Formatter{
		FieldsOrder:     []string{pkgKey},
		TimestampFormat: "01-02|15:04:05",
	})

	if cfg.LogToEcs {
		logrusLogger.SetFormatter(&ecslogrus.Formatter{})
	}

	logger := log.NewLogger(logrusLogger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	foundeationDB.MustAPIVersion(foundationDBVersion)

	logger.WithField("cluster_location", cfg.DatabasePath).Info("starting foundationdb")

	db, err := foundeationDB.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		panic(err)
	}

	st := fdb.NewDB(db, logrusLogger.WithField(pkgKey, "fdb"))

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress})
	rst := repo.NewDB(rdb, logger.WithField(pkgKey, "repo"))

	ratingFetcher := ratingCollector.NewFetcher(
		cfg.AppID,
		cfg.AppHash,
		cfg.Phone,
		st,
		rst,
		logger.WithField(pkgKey, "rating_fetcher"),
	)
	if err = ratingFetcher.Auth(ctx); err != nil {
		panic(err)
	}

	defer func() {
		if err = ratingFetcher.Stop(); err != nil {
			logger.Error(err)
		}
	}()

	if err = ratingFetcher.FetchRatingsAndSave(ctx, cfg.ChannelID); err != nil {
		panic(err)
	}

	ratingFetcher.SubscribeAndSave(ctx, cfg.ChannelID)

	logger.Info("service started")
	<-ctx.Done()
}
