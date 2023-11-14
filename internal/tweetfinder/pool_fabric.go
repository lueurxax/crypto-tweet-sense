package tweetfinder

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const startDelay = 15

func NewPoolFabric(config ConfigPool, pkgKey string, logger log.Logger) (Finder, error) {
	finders := make([]Finder, 0, len(config.XCreds))
	delayManagerLogger := logger.WithField(pkgKey, "delay_manager")
	finderLogger := logger.WithField(pkgKey, "finder")

	for login, password := range config.XCreds {
		filename := strings.Join([]string{login, config.CookiesFilename}, "_")
		scraper := twitterscraper.New().WithDelay(startDelay).SetSearchMode(twitterscraper.SearchLatest)

		var cookies []*http.Cookie

		data, err := os.ReadFile(filename)
		if err != nil {
			logger.Error(err)

			if err = scraper.Login(login, password); err != nil {
				return nil, err
			}
		}

		if data != nil {
			if err = json.Unmarshal(data, &cookies); err != nil {
				logger.Error(err)

				if err = scraper.Login(login, password); err != nil {
					return nil, err
				}
			}
		}

		if cookies != nil {
			scraper.SetCookies(cookies)

			if !scraper.IsLoggedIn() {
				if err = scraper.Login(login, password); err != nil {
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

		delayManager := NewDelayManager(
			func(seconds int64) { scraper.WithDelay(seconds) },
			startDelay,
			delayManagerLogger,
		)

		finders = append(finders, NewFinder(scraper, delayManager, finderLogger))
	}

	return NewPool(finders), nil
}