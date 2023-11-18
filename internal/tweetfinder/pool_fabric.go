package tweetfinder

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const (
	startDelay     = 15
	finderIndexKey = "finder_index"
)

func NewPoolFabric(config ConfigPool, pkgKey string, logger log.Logger) (Finder, error) {
	finders := make([]Finder, 0, len(config.XCreds))
	delayManagerLogger := logger.WithField(pkgKey, "delay_manager")
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

		var f func(setter func(seconds int64), minimalDelay int64, log log.Logger) Manager

		if i == 4 {
			f = NewDelayManagerV2

		} else {
			f = NewDelayManager
		}

		delayManager = f(
			func(seconds int64) { scraper.WithDelay(seconds) },
			startDelay,
			delayManagerLogger.WithField(finderIndexKey, i),
		)

		finders = append(finders, NewFinder(scraper, delayManager, finderLogger.WithField(finderIndexKey, i)))
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
