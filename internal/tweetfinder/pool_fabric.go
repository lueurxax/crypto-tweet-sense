package tweetfinder

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder/windowlimiter"
)

const (
	startDelay  = 15
	finderLogin = "finder_login"
)

func NewPoolFabric(ctx context.Context, config ConfigPool, pkgKey string, repo repo, logger log.Logger) (Finder, error) {
	finders := make([]Finder, 0, len(config.XCreds))
	delayManagerLogger := logger.WithField(pkgKey, "delay_manager")
	limiterLogger := logger.WithField(pkgKey, "window_limiter")
	finderLogger := logger.WithField(pkgKey, "finder")
	poolLogger := logger.WithField(pkgKey, "finder_pool")

	i := 0

	var delayManager Manager

	for login, password := range config.XCreds {
		filename := strings.Join([]string{login, config.CookiesFilename}, "_")
		scraper := twitterscraper.New().WithDelay(startDelay).SetSearchMode(twitterscraper.SearchLatest)

		var cookies []*http.Cookie

		data, err := os.ReadFile(filename)
		if err != nil {
			logger.Error(err)

			if err = scrapperLogin(scraper, config.XConfirmation[len(finders)], login, password); err != nil {
				return nil, err
			}
		}

		if data != nil {
			if err = json.Unmarshal(data, &cookies); err != nil {
				logger.Error(err)

				if err = scrapperLogin(scraper, config.XConfirmation[len(finders)], login, password); err != nil {
					return nil, err
				}
			}
		}

		if cookies != nil {
			scraper.SetCookies(cookies)

			if !scraper.IsLoggedIn() {
				if err = scrapperLogin(scraper, config.XConfirmation[len(finders)], login, password); err != nil {
					return nil, err
				}
			}
		}

		cookies = scraper.GetCookies()

		data, err = json.Marshal(cookies)
		if err != nil {
			return nil, err
		}

		if err = os.WriteFile(filename, data, 0600); err != nil {
			return nil, err
		}

		if len(config.Proxies) > i {
			if err = scraper.SetProxy(config.Proxies[i]); err != nil {
				return nil, err
			}
		}

		limiterIntervals := []time.Duration{
			time.Minute,
			time.Hour,
			time.Hour * 24,
			time.Hour * 24 * 30,
		}

		windowLimiters := make([]WindowLimiter, len(limiterIntervals))

		for j := len(limiterIntervals) - 1; j >= 0; j-- {
			resetInterval := limiterIntervals[j]
			if j != len(limiterIntervals)-1 {
				resetInterval = limiterIntervals[j+1]
			}

			windowLimiters[j] = windowlimiter.NewLimiter(
				limiterIntervals[j],
				resetInterval,
				login,
				repo,
				limiterLogger.WithField(finderLogin, login),
			)

			if j != len(limiterIntervals)-1 {
				windowLimiters[j].SetResetLimiter(windowLimiters[j+1])
			}
		}

		delayManager = NewDelayManagerV2(
			func(seconds int64) { scraper.WithDelay(seconds) },
			windowLimiters,
			startDelay,
			delayManagerLogger.WithField(finderLogin, login),
		)

		if err = delayManager.Start(ctx); err != nil {
			return nil, err
		}

		finders = append(finders, NewFinder(scraper, delayManager, finderLogger.WithField(finderLogin, login)))
		i++
	}

	return NewPool(finders, poolLogger), nil
}

func scrapperLogin(scraper *twitterscraper.Scraper, confirmation string, login string, password string) error {
	if confirmation == "X" {
		return scraper.Login(login, password)
	}

	return scraper.Login(login, password, confirmation)
}
