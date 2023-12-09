package account_manager

import (
	"context"
	"errors"
	"net/http"
	"os"

	jsoniter "github.com/json-iterator/go"
	twitterscraper "github.com/lueurxax/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
)

type Manager interface {
	AddAccount(ctx context.Context, config Config) error
	AuthScrapper(ctx context.Context, account common.TwitterAccount, scraper *twitterscraper.Scraper) error
	SearchUnAuthAccounts(ctx context.Context) ([]common.TwitterAccount, error)
}

type repo interface {
	GetAccount(ctx context.Context, login string) (common.TwitterAccount, error)
	SaveAccount(ctx context.Context, account common.TwitterAccount) error
	SaveCookie(ctx context.Context, login string, cookie []*http.Cookie) error
	GetCookie(ctx context.Context, login string) ([]*http.Cookie, error)
	GetAccounts(ctx context.Context) ([]common.TwitterAccount, error)
}

type manager struct {
	repo
	authAccounts map[string]struct{}

	log log.Logger
}

func (m *manager) SearchUnAuthAccounts(ctx context.Context) ([]common.TwitterAccount, error) {
	accounts, err := m.repo.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]common.TwitterAccount, 0, len(accounts))

	for _, account := range accounts {
		if _, ok := m.authAccounts[account.Login]; !ok {
			res = append(res, account)
		}
	}

	return res, nil
}

func (m *manager) AuthScrapper(ctx context.Context, account common.TwitterAccount, scraper *twitterscraper.Scraper) error {
	cookies, err := m.repo.GetCookie(ctx, account.Login)
	if err != nil {
		if !errors.Is(err, fdb.ErrCookieNotFound) {
			m.log.WithError(err).WithField("login", account.Login).Error("error while login")
			return err
		}

		if err = scrapperLogin(ctx, scraper, account); err != nil {
			m.log.WithError(err).WithField("login", account.Login).Error("error while login")
			return err
		}
	}

	scraper.SetCookies(cookies)

	if !scraper.IsLoggedIn(ctx) {
		if err = scrapperLogin(ctx, scraper, account); err != nil {
			m.log.WithError(err).WithField("login", account.Login).Error("error while login")
			return err
		}
	}

	cookies = scraper.GetCookies()

	if err = m.repo.SaveCookie(ctx, account.Login, cookies); err != nil {
		m.log.WithError(err).WithField("login", account.Login).Error("error while login")
		return err
	}

	m.authAccounts[account.Login] = struct{}{}

	return nil
}

func (m *manager) AddAccount(ctx context.Context, config Config) error {
	account := common.TwitterAccount{
		Login:       config.Login,
		AccessToken: config.Password,
	}

	if config.Confirmation != "" {
		account.Confirmation = config.Confirmation
	}

	if err := m.repo.SaveAccount(ctx, account); err != nil {
		return err
	}

	if config.CookiesFilename == "" {
		return nil
	}

	data, err := os.ReadFile(config.CookiesFilename)
	if err != nil {
		return err
	}

	cookies := make([]*http.Cookie, 0)
	if err = jsoniter.Unmarshal(data, &cookies); err != nil {
		return err
	}

	return m.repo.SaveCookie(ctx, config.Login, cookies)
}

func scrapperLogin(ctx context.Context, scraper *twitterscraper.Scraper, account common.TwitterAccount) error {
	if account.Confirmation == "" {
		return scraper.Login(ctx, account.Login, account.AccessToken)
	}

	return scraper.Login(ctx, account.Login, account.AccessToken, account.Confirmation)
}

func NewManager(repo repo, logger log.Logger) Manager {
	return &manager{repo: repo, authAccounts: make(map[string]struct{}), log: logger}
}
