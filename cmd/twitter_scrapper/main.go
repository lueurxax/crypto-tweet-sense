package main

import (
	"context"
	"flag"
	"fmt"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	nested "github.com/antonfisher/nested-logrus-formatter"
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/buaazp/fasthttprouter"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
)

var version = "dev"

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
	GetMethod           = "GET"
)

type config struct {
	LoggerLevel      logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs         bool         `envconfig:"LOG_TO_ECS" default:"false"`
	TopCount         int          `envconfig:"TOP_COUNT" default:"1000"`
	DatabasePath     string       `default:"/usr/local/etc/foundationdb/fdb.cluster"`
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

	checker := ratingCollector.NewChecker(st, cfg.TopCount)

	xConfig := tweetFinder.GetConfigPool()

	accountManager := account_manager.NewManager(st, logger.WithField(pkgKey, "account_manager"))

	all := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "crypto_tweet_sense",
		Subsystem: "finder",
		Name:      "find_all_requests_seconds",
		Help:      "Find all requests histogram in seconds",
		Buckets:   []float64{1, 10, 100, 200, 300, 400, 500, 1000, 2000, 3000, 4000},
	}, []string{"login", "search", "error"})

	one := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "crypto_tweet_sense",
		Subsystem: "finder",
		Name:      "find_requests_seconds",
		Help:      "Find requests histogram in seconds",
		Buckets:   []float64{.005, .05, .1, .5, 1, 2.5, 5, 10, 25, 50, 100},
	}, []string{"login", "error"})

	prometheus.MustRegister(all, one)

	finder := tweetFinder.NewPool(all, one, xConfig, accountManager, st, logger.WithField(pkgKey, "tweet_finder_pool"))
	if err = finder.Init(ctx); err != nil {
		panic(err)
	}

	watch := watcher.NewWatcher(finder, st, checker, logger.WithField(pkgKey, "watcher"))

	watch.Watch()

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
