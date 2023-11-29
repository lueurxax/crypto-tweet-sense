package account_manager

import (
	"context"
	"net/http"
	"os"

	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type Manager interface {
	AddAccount(ctx context.Context, config Config) error
}

type repo interface {
	SaveAccount(ctx context.Context, account common.TwitterAccount) error
	SaveCookie(ctx context.Context, login string, cookie []*http.Cookie) error
}

type manager struct {
	repo
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

func NewManager(repo repo) Manager {
	return &manager{repo: repo}
}
