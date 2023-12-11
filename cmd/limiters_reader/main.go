package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"

	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
)

var version = "dev"

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
)

type config struct {
	LoggerLevel  logrus.Level  `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs     bool          `envconfig:"LOG_TO_ECS" default:"false"`
	DatabasePath string        `default:"/usr/local/etc/foundationdb/fdb.cluster"`
	Login        string        `envconfig:"LOGIN" required:"true"`
	Window       time.Duration `envconfig:"WINDOW" required:"true"`
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

	foundeationDB.MustAPIVersion(foundationDBVersion)

	db, err := foundeationDB.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		panic(err)
	}

	st := fdb.NewDB(db, logrusLogger.WithField(pkgKey, "fdb"))

	limits, err := st.GetRequestLimitDebug(context.Background(), cfg.Login, cfg.Window)
	if err != nil {
		panic(err)
	}

	data, err := jsoniter.MarshalToString(limits)
	if err != nil {
		panic(err)
	}

	fmt.Println(data)
}
