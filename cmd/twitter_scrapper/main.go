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
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/account_manager"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	ratingCollector "github.com/lueurxax/crypto-tweet-sense/internal/ratingcollector"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
	tweetFinder "github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder"
	"github.com/lueurxax/crypto-tweet-sense/internal/watcher"
)

var version = "dev"

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
)

type config struct {
	LoggerLevel  logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs     bool         `envconfig:"LOG_TO_ECS" default:"false"`
	TopCount     int          `envconfig:"TOP_COUNT" default:"1000"`
	DatabasePath string       `default:"/usr/local/etc/foundationdb/fdb.cluster"`
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

	db, err := foundeationDB.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		panic(err)
	}

	st := fdb.NewDB(db, logrusLogger.WithField(pkgKey, "fdb"))

	checker := ratingCollector.NewChecker(st, cfg.TopCount)

	xConfig := tweetFinder.GetConfigPool()

	accountManager := account_manager.NewManager(st)

	finder := tweetFinder.NewPool(xConfig, accountManager, st, logger.WithField(pkgKey, "tweet_finder_pool"))
	if err = finder.Init(ctx); err != nil {
		panic(err)
	}

	watch := watcher.NewWatcher(finder, st, checker, logger.WithField(pkgKey, "watcher"))

	watch.Watch()

	logger.Info("service started")
	<-ctx.Done()
}
