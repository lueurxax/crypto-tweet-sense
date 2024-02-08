package tweetseditor

import (
	"context"
	"errors"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
)

type Manager interface {
	Edit(ctx context.Context) context.Context
	SubscribeEdited() <-chan string
	SubscribeLongStoryMessages() <-chan string
	SubscribeRusStoryMessages() <-chan string
}

type repo interface {
	GetTweetForShortEdit(ctx context.Context) ([]common.Tweet, error)
	DeleteShortEditedTweets(ctx context.Context, ids []string) error
	GetTweetForLongEdit(ctx context.Context, count int) ([]common.Tweet, error)
	DeleteLongEditedTweets(ctx context.Context, ids []string) error
}

type manager struct {
	sendInterval  time.Duration
	cleanInterval time.Duration

	editedCh                 chan string
	longStoryEditedCh        chan string
	russianLongStoryEditedCh chan string

	editor Editor

	repo

	log log.Logger
}

func (m *manager) Edit(ctx context.Context) context.Context {
	go m.editLoop(ctx)

	return ctx
}

func (m *manager) SubscribeEdited() <-chan string { return m.editedCh }

func (m *manager) SubscribeLongStoryMessages() <-chan string { return m.longStoryEditedCh }

func (m *manager) SubscribeRusStoryMessages() <-chan string { return m.russianLongStoryEditedCh }

func (m *manager) editLoop(ctx context.Context) {
	ticker := time.NewTicker(m.sendInterval)
	longTicker := time.NewTicker(2 * m.sendInterval)
	contextCleanerTicker := time.NewTicker(10 * m.sendInterval)

	for {
		select {
		case <-ctx.Done():
			close(m.editedCh)
			m.log.Info("edit loop done")

			return
		case <-ticker.C:
			collectedTweets, err := m.repo.GetTweetForShortEdit(ctx)
			if err != nil {
				if errors.Is(err, fdb.ErrTweetsNotFound) {
					m.log.Info("skip edit, because no tweets")
					continue
				}

				m.log.WithError(err).Error("get tweets for edit error")

				continue
			}

			if err = m.editor.Edit(ctx, collectedTweets, m.editedCh); err != nil {
				m.log.WithError(err).Error("edit error")
				continue
			}

			deletingTweets := make([]string, 0, len(collectedTweets))
			for _, tweet := range collectedTweets {
				deletingTweets = append(deletingTweets, tweet.ID)
			}

			if err = m.repo.DeleteShortEditedTweets(ctx, deletingTweets); err != nil {
				m.log.WithError(err).Error("delete edited tweets error")
			}
		case <-longTicker.C:
			collectedTweets, err := m.repo.GetTweetForLongEdit(ctx, 20)
			if err != nil {
				if errors.Is(err, fdb.ErrTweetsNotFound) {
					m.log.Info("skip edit, because no tweets")
					continue
				}

				if errors.Is(err, fdb.ErrNotEnoughTweets) {
					m.log.Info("skip edit, because not enough tweets")
					continue
				}

				m.log.WithError(err).Error("get tweets for edit error")

				continue
			}

			if err = m.longStoryProcess(ctx, collectedTweets); err != nil {
				m.log.WithError(err).Error("edit error")
				continue
			}

			deletingTweets := make([]string, 0, len(collectedTweets))
			for _, tweet := range collectedTweets {
				deletingTweets = append(deletingTweets, tweet.ID)
			}

			if err = m.repo.DeleteLongEditedTweets(ctx, deletingTweets); err != nil {
				m.log.WithError(err).Error("delete edited tweets error")
			}
		case <-contextCleanerTicker.C:
			m.editor.Clean()
		}
	}
}

func (m *manager) longStoryProcess(ctx context.Context, tweets []common.Tweet) error {
	// FIXME temporary solution
	retry := 0

	err := errors.New("initial error")

	var content string

	for err != nil && retry < 10 {
		content, err = m.editor.EditLongStory(ctx, tweets, m.longStoryEditedCh)
		if err != nil {
			m.log.WithField("retry", retry).WithError(err).Error("long story summary generation error")

			retry++
		}
	}

	if err == nil && content != "" {
		go func() {
			translatedContent, err := m.editor.TranslateLongStory(ctx, content)
			if err != nil {
				m.log.WithError(err).Error("rus long story summary generation error")

				return
			}

			m.russianLongStoryEditedCh <- translatedContent
		}()
	}

	return err
}

func NewManager(sendInterval time.Duration, cleanInterval time.Duration, editor Editor, repo repo, log log.Logger) Manager {
	return &manager{
		sendInterval:             sendInterval,
		cleanInterval:            cleanInterval,
		editedCh:                 make(chan string, queueLen),
		longStoryEditedCh:        make(chan string, queueLen),
		russianLongStoryEditedCh: make(chan string, queueLen),
		editor:                   editor,
		repo:                     repo,
		log:                      log,
	}
}
