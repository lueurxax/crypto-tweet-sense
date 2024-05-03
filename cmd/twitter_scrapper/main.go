package main

import (
	"context"
	"flag"
	"fmt"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/buaazp/fasthttprouter"
	"github.com/kelseyhightower/envconfig"
	repo "github.com/lueurxax/crypto-tweet-sense/internal/repo/redis"
	watcherMetrics "github.com/lueurxax/crypto-tweet-sense/internal/watcher/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go.elastic.co/ecslogrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/account_manager"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	ratingCollector "github.com/lueurxax/crypto-tweet-sense/internal/ratingcollector"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
	tweetFinder "github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder"
	"github.com/lueurxax/crypto-tweet-sense/internal/watcher"
	"github.com/lueurxax/crypto-tweet-sense/internal/watcher/doubledelayer"
)

var version = "dev"

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
	GetMethod           = "GET"
	namespace           = "crypto_tweet_sense"
	subsystem           = "finder"
)

type config struct {
	LoggerLevel      logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs         bool         `envconfig:"LOG_TO_ECS" default:"false"`
	TopCount         int          `envconfig:"TOP_COUNT" default:"1000"`
	DatabasePath     string       `default:"/usr/local/etc/foundationdb/fdb.cluster"`
	RedisAddress     string       `default:"localhost:6379"`
	MetricsSubsystem string       `envconfig:"METRICS_SUBSYSTEM" default:"crypto_tweet_sense"`
	DiagHTTPPort     int          `envconfig:"DIAG_HTTP_PORT" default:"8080"`
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

	if err = st.Migrate(ctx); err != nil {
		panic(err)
	}

	if err = st.CleanWrongIndexes(ctx); err != nil {
		panic(err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress})

	rst := repo.NewDB(rdb, logger.WithField(pkgKey, "repo"))

	if err = rst.Migrate(ctx); err != nil {
		panic(err)
	}

	checker := ratingCollector.NewChecker(st, cfg.TopCount)

	xConfig := tweetFinder.GetConfigPool()

	accountManager := account_manager.NewManager(rst, logger.WithField(pkgKey, "account_manager"))

	next := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "find_next_requests_seconds",
		Help:      "Find next requests histogram in seconds",
		Buckets:   []float64{.005, .05, .1, .5, 1, 2.5, 5, 10, 25, 50, 100, 1000, 3600},
	}, []string{"login", "search", "error"})

	one := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "find_requests_seconds",
		Help:      "Find requests histogram in seconds",
		Buckets:   []float64{.005, .05, .1, .5, 1, 2.5, 5, 10, 25, 50, 100, 1000, 3600},
	}, []string{"login", "error"})

	delay := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "delay_seconds",
		Help:      "Requests delay in seconds",
	}, []string{"login"})

	tweetCounter := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "tweets_count",
		Help:      "Tweets count",
	}, []string{})

	prometheus.MustRegister(one, next, delay, tweetCounter)

	finder := tweetFinder.NewPool(one, next, delay, xConfig, accountManager, st, logger.WithField(pkgKey, "tweet_finder_pool"))
	if err = finder.Init(ctx); err != nil {
		panic(err)
	}

	finderWithMetrics := tweetFinder.NewMetricMiddleware(one, next, "pool", finder)

	go watcherMetrics.NewMetrics(tweetCounter, st, logger.WithField(pkgKey, "watcher_metrics")).Start(ctx)

	watch := watcher.NewWatcher(
		watcher.GetConfig(),
		finderWithMetrics,
		st,
		checker,
		doubledelayer.NewDelayer(time.Minute, time.Second),
		logger.WithField(pkgKey, "watcher"),
	)

	watch.Watch(ctx)

	diagAPIRouter := fasthttprouter.New()
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Index))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/profile", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Profile))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/trace", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Trace))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/symbol", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Symbol))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/cmdline", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Cmdline))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/goroutine", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("goroutine").ServeHTTP))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/heap", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("heap").ServeHTTP))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/allocs", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("allocs").ServeHTTP))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/block", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("block").ServeHTTP))
	diagAPIRouter.Handle(GetMethod, "/debug/pprof/mutex", fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("mutex").ServeHTTP))
	diagAPIRouter.Handle(GetMethod, "/metrics", fasthttpadaptor.NewFastHTTPHandlerFunc(promhttp.Handler().ServeHTTP))
	diagAPIServer := &fasthttp.Server{
		Handler: diagAPIRouter.Handler,
	}

	go func() {
		logger.WithField("port", cfg.DiagHTTPPort).Info("starting diag API server")

		if err = diagAPIServer.ListenAndServe(fmt.Sprintf(":%d", cfg.DiagHTTPPort)); err != nil {
			logger.WithError(err).Error("diag API server run failure")
			os.Exit(1)
		}
	}()

	logger.Info("service started")
	<-ctx.Done()
}
