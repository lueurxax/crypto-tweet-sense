package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/kelseyhightower/envconfig"
	repo "github.com/lueurxax/crypto-tweet-sense/internal/repo/redis"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

var version = "dev"

const (
	pkgKey = "pkg"
)

type config struct {
	LoggerLevel  logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs     bool         `envconfig:"LOG_TO_ECS" default:"false"`
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

	logger.WithField("cluster_location", cfg.RedisAddress).Info("starting redis client")

	db := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress})

	st := repo.NewDB(db, logger.WithField(pkgKey, "repo"))

	data, err := os.ReadFile("./.session.json")
	if err != nil {
		panic(err)
	}

	if err = st.StoreSession(ctx, data); err != nil {
		panic(err)
	}
	<-ctx.Done()
}
