package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/kelseyhightower/envconfig"
	twitterscraper "github.com/n0madic/twitter-scraper"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"
	"gopkg.in/telebot.v3"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	ratingCollector "github.com/lueurxax/crypto-tweet-sense/internal/rating_collector"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
	"github.com/lueurxax/crypto-tweet-sense/internal/sender"
	tweetFinder "github.com/lueurxax/crypto-tweet-sense/internal/tweet_finder"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweets_editor"
	"github.com/lueurxax/crypto-tweet-sense/internal/watcher"
)

var version = "dev"

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
)

type config struct {
	LoggerLevel        logrus.Level  `envconfig:"LOG_LEVEL" default:"info"`
	LogToEcs           bool          `envconfig:"LOG_TO_ECS" default:"false"`
	TopCount           int           `envconfig:"TOP_COUNT" default:"1000"`
	SessionFile        string        `envconfig:"SESSION_FILE" required:"true"`
	BotToken           string        `envconfig:"BOT_TOKEN" required:"true"`
	XLogin             string        `envconfig:"X_LOGIN" required:"true"`
	XPassword          string        `envconfig:"X_PASSWORD" required:"true"`
	ChannelID          int64         `envconfig:"CHANNEL_ID" required:"true"`
	ChatID             int64         `envconfig:"CHAT_ID" required:"true"`
	AppID              int           `envconfig:"APP_ID" required:"true"`
	AppHash            string        `envconfig:"APP_HASH" required:"true"`
	Phone              string        `envconfig:"PHONE" required:"true"`
	ChatGPTToken       string        `envconfig:"CHAT_GPT_TOKEN" required:"true"`    // OpenAI token
	EditorSendInterval time.Duration `envconfig:"EDITOR_SEND_INTERVAL" default:"2h"` // Interval to send edited tweets to telegram
	DatabasePath       string        `default:"/usr/local/etc/foundationdb/fdb.cluster"`
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

	ratingFetcher := ratingCollector.NewFetcher(
		cfg.AppID,
		cfg.AppHash,
		cfg.Phone,
		cfg.SessionFile,
		logger.WithField(pkgKey, "rating_fetcher"),
	)
	if err := ratingFetcher.Auth(ctx); err != nil {
		panic(err)
	}

	defer func() {
		if err := ratingFetcher.Stop(); err != nil {
			logger.Error(err)
		}
	}()

	res, links, err := ratingFetcher.FetchRatingsAndUniqueMessages(ctx, cfg.ChannelID)
	if err != nil {
		panic(err)
	}

	scraper := twitterscraper.New().WithDelay(10).SetSearchMode(twitterscraper.SearchLatest)

	var cookies []*http.Cookie

	data, err := os.ReadFile("cookies.json")
	if err != nil {
		logger.Error(err)
		if err = scrapperLogin(scraper, cfg.XLogin, cfg.XPassword); err != nil {
			panic(err)
		}
	}

	if data != nil {
		if err = json.Unmarshal(data, &cookies); err != nil {
			logger.Error(err)
			if err = scrapperLogin(scraper, cfg.XLogin, cfg.XPassword); err != nil {
				panic(err)
			}
		}
	}

	if cookies != nil {
		scraper.SetCookies(cookies)
		if !scraper.IsLoggedIn() {
			if err = scrapperLogin(scraper, cfg.XLogin, cfg.XPassword); err != nil {
				panic(err)
			}
		}
	}

	cookies = scraper.GetCookies()

	data, err = json.Marshal(cookies)
	if err != nil {
		panic(err)
	}

	if err = os.WriteFile("cookies.json", data, 0644); err != nil {
		panic(err)
	}
	checker := ratingCollector.NewChecker(res, cfg.TopCount)

	delayManager := tweetFinder.NewDelayManager(
		func(seconds int64) { scraper.WithDelay(seconds) },
		10,
		logger.WithField(pkgKey, "delay_manager"),
	)
	finder := tweetFinder.NewFinder(scraper, delayManager, logger.WithField(pkgKey, "finder"))
	watch := watcher.NewWatcher(finder, st, checker, links, logger.WithField(pkgKey, "watcher"))

	checker.CollectRatings(ratingFetcher.Subscribe(ctx, cfg.ChannelID))

	api, err := telebot.NewBot(
		telebot.Settings{Token: cfg.BotToken, Poller: &telebot.LongPoller{Timeout: 10 * time.Second}},
	)
	if err != nil {
		panic(err)
	}

	s := sender.NewSender(api, &telebot.Chat{ID: cfg.ChatID}, logger.WithField(pkgKey, "sender"))
	ctx = s.Send(ctx, watch.Subscribe())

	editor := tweets_editor.NewEditor(
		openai.NewClient(cfg.ChatGPTToken),
		cfg.EditorSendInterval,
		logger.WithField(pkgKey, "editor"),
	)
	editor.Edit(ctx, watch.RawSubscribe())
	ctx = s.Send(ctx, editor.SubscribeEdited())

	watch.Watch()

	logger.Info("service started")
	<-ctx.Done()
}

func scrapperLogin(scraper *twitterscraper.Scraper, login, password string) error {
	return scraper.Login(login, password)
}
