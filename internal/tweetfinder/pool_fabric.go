package tweetfinder

import (
	"context"
	"errors"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder/windowlimiter"
)

const (
	startDelay  = 15
	finderLogin = "finder_login"
)

func NewPoolFabric(ctx context.Context, config ConfigPool, pkgKey string, repo fdb.DB, logger log.Logger) (Finder, error) {
	finders := make([]Finder, 0, len(config.XLogins))
	delayManagerLogger := logger.WithField(pkgKey, "delay_manager")
	limiterLogger := logger.WithField(pkgKey, "window_limiter")
	finderLogger := logger.WithField(pkgKey, "finder")
	poolLogger := logger.WithField(pkgKey, "finder_pool")

	var delayManager Manager

	for i, login := range config.XLogins {
		scraper := twitterscraper.New().WithDelay(startDelay).SetSearchMode(twitterscraper.SearchLatest)

		creds, err := repo.GetAccount(ctx, login)
		if err != nil {
			return nil, err
		}

		cookies, err := repo.GetCookie(ctx, login)
		if err != nil {
			if errors.Is(err, fdb.ErrCookieNotFound) {
				if err = scrapperLogin(scraper, creds); err != nil {
					return nil, err
				}
			}
			return nil, err
		}

		scraper.SetCookies(cookies)

		if !scraper.IsLoggedIn() {
			if err = scrapperLogin(scraper, creds); err != nil {
				return nil, err
			}
		}

		cookies = scraper.GetCookies()

		if err = repo.SaveCookie(ctx, login, cookies); err != nil {
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

func scrapperLogin(scraper *twitterscraper.Scraper, account common.TwitterAccount) error {
	if account.Confirmation == "" {
		return scraper.Login(account.Login, account.AccessToken)
	}

	return scraper.Login(account.Login, account.AccessToken, account.Confirmation)
}
