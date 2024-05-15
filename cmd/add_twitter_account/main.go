package main

import (
	"context"
	"os/signal"
	"syscall"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/kelseyhightower/envconfig"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/account_manager"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	repo "github.com/lueurxax/crypto-tweet-sense/internal/repo/redis"
)

const (
	pkgKey = "pkg"
)

type config struct {
	LoggerLevel  logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs     bool         `envconfig:"LOG_TO_ECS" default:"false"`
	RedisAddress string       `default:"localhost:6379"`
}

func main() {
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

	logger.WithField("cluster_location", cfg.RedisAddress).Info("starting redis connection")

	db := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress})

	st := repo.NewDB(db, logger.WithField(pkgKey, "repo"))

	if err := account_manager.NewManager(st, logger.WithField(pkgKey, "account_manager")).AddAccount(ctx, account_manager.GetConfig()); err != nil {
		panic(err)
	}

	logger.Info("account added")
}
