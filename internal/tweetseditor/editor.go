package tweetseditor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/sashabaranov/go-openai"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	fdb "github.com/lueurxax/crypto-tweet-sense/internal/repo"
	"github.com/lueurxax/crypto-tweet-sense/pkg/utils"
)

const (
	prompt     = "I have several popular crypto tweets today. Can you extract information useful for cryptocurrency investing from these tweets and make summary? Skip information such as airdrops or giveaway, if they are not useful for investing. I will parse your answer by code like json `{\"tweets\":[{\"telegram_message\":\"summarized message by tweet\", \"link\":\"link to tweet\"}], \"new_useful_information\":true}`, then can you prepare messages in json with prepared telegram message? \nTweets: %s." //nolint:lll
	nextPrompt = "Additional tweets, create new message only for new information: %s."                                                                                                                                                                                                                                                                                                                                                                                                                                       //nolint:lll
	queueLen   = 10
)

type Tweet struct {
	Content string `json:"telegram_message"`
	Link    string `json:"link"`
}

type repo interface {
	GetTweetForEdit(ctx context.Context) ([]common.Tweet, error)
	DeleteEditedTweets(ctx context.Context, ids []string) error
}

type Editor interface {
	Edit(ctx context.Context) context.Context
	SubscribeEdited() <-chan string
}

type editor struct {
	editedCh      chan string
	client        *openai.Client
	sendInterval  time.Duration
	cleanInterval time.Duration
	existMessages []openai.ChatCompletionMessage
	repo

	log log.Logger
}

func (e *editor) Edit(ctx context.Context) context.Context {
	go e.editLoop(ctx)

	return ctx
}

func (e *editor) SubscribeEdited() <-chan string {
	return e.editedCh
}

func (e *editor) editLoop(ctx context.Context) {
	ticker := time.NewTicker(e.sendInterval)
	contextCleanerTicker := time.NewTicker(e.sendInterval)

	for {
		select {
		case <-ctx.Done():
			close(e.editedCh)
			e.log.Info("edit loop done")

			return
		case <-ticker.C:
			collectedTweets, err := e.repo.GetTweetForEdit(ctx)
			if err != nil {
				if errors.Is(err, fdb.ErrTweetsNotFound) {
					e.log.Info("skip edit, because no tweets")
					continue
				}

				e.log.WithError(err).Error("get tweets for edit error")

				continue
			}

			if err = e.edit(ctx, collectedTweets); err != nil {
				e.log.WithError(err).Error("edit error")
				continue
			}

			deletingTweets := make([]string, 0, len(collectedTweets))
			for _, tweet := range collectedTweets {
				deletingTweets = append(deletingTweets, tweet.ID)
			}

			if err = e.repo.DeleteEditedTweets(ctx, deletingTweets); err != nil {
				e.log.WithError(err).Error("delete edited tweets error")
			}
		case <-contextCleanerTicker.C:
			if len(e.existMessages) > 0 {
				e.existMessages = make([]openai.ChatCompletionMessage, 0)
			}
		}
	}
}

func (e *editor) edit(ctx context.Context, tweets []common.Tweet) error {
	tweetsStr := ""

	tweetsMap := map[string]common.Tweet{}

	for _, twee := range tweets {
		tweetsMap[twee.PermanentURL] = twee
		text := twee.Text + "Link - " + twee.PermanentURL
		tweetsStr = strings.Join([]string{tweetsStr, text}, "\n")
	}

	request := ""
	if len(e.existMessages) == 0 {
		request = fmt.Sprintf(prompt, tweetsStr)
	} else {
		request = fmt.Sprintf(nextPrompt, tweetsStr)
	}

	requestMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: request,
	}

	resp, err := e.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Model:    openai.GPT4TurboPreview,
			Messages: append(e.existMessages, requestMessage),
		},
	)

	if err != nil {
		e.log.WithError(err).Error("summary generation error")
		return err
	}

	e.log.WithField("response", resp).Debug("summary generation result")

	res := struct {
		Tweets               []Tweet `json:"tweets"`
		NewUsefulInformation bool    `json:"new_useful_information"`
	}{}

	if err = jsoniter.UnmarshalFromString(resp.Choices[0].Message.Content, &res); err != nil {
		// TODO: try to search correct json in string
		e.log.WithError(err).Error("summary unmarshal error")
		e.editedCh <- utils.Escape(resp.Choices[0].Message.Content)

		return nil
	}

	if !res.NewUsefulInformation {
		e.log.Info("skip edit, no new useful information")
		return nil
	}

	e.existMessages = append(e.existMessages, requestMessage, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp.Choices[0].Message.Content,
	})

	data := ""

	for _, el := range res.Tweets {
		tweet, ok := tweetsMap[el.Link]
		if ok {
			e.editedCh <- fmt.Sprintf("%s\n%s", data, e.formatTweet(tweet, el.Content))
		}
	}

	return nil
}

func (e *editor) formatTweet(tweet common.Tweet, text string) (str string) {
	str = fmt.Sprintf("*%s*\n", utils.Escape(tweet.TimeParsed.Format(time.RFC3339)))
	str += fmt.Sprintf("%s\n", utils.Escape(text))

	for _, photo := range tweet.Photos {
		str += fmt.Sprintf("[photo](%s)\n", utils.Escape(photo.URL))
	}

	for _, video := range tweet.Videos {
		str += fmt.Sprintf("[video](%s)\n", utils.Escape(video.URL))
	}

	str += fmt.Sprintf("[link](%s)\n", utils.Escape(tweet.PermanentURL))

	return
}

func NewEditor(client *openai.Client, db repo, sendInterval, cleanInterval time.Duration, log log.Logger) Editor {
	return &editor{
		editedCh:      make(chan string, queueLen),
		sendInterval:  sendInterval,
		cleanInterval: cleanInterval,
		client:        client,
		repo:          db,
		log:           log,
	}
}
