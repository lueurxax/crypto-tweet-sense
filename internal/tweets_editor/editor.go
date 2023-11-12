package tweets_editor

import (
	"context"
	"fmt"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/sashabaranov/go-openai"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/pkg/utils"
)

const prompt = "I have several popular crypto tweets today. Can you extract information useful for cryptocurrency investing from these tweets and make summary? Skip info if it not useful for investing. I will parse your answer by code like json `{\"message\":\"telegram message\"}`, then can you prepare message and replace \"telegram message\" in json with prepared telegram message? \nTweets: %s."

type Editor interface {
	Edit(ctx context.Context, tweetCh <-chan string) context.Context
	SubscribeEdited() <-chan string
}

type editor struct {
	editedCh     chan string
	client       *openai.Client
	sendInterval time.Duration

	log log.Logger
}

func (e *editor) Edit(ctx context.Context, tweetCh <-chan string) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go e.editLoop(ctx, cancel, tweetCh)

	return ctx
}

func (e *editor) SubscribeEdited() <-chan string {
	return e.editedCh
}

func (e *editor) editLoop(ctx context.Context, cancel context.CancelFunc, ch <-chan string) {
	collectedTweets := make([]string, 0)
	ticker := time.NewTicker(e.sendInterval)

	for {
		select {
		case <-ctx.Done():
			close(e.editedCh)
			e.log.Info("edit loop done")

			return
		case tweet := <-ch:
			e.log.WithField("tweet", tweet).Debug("tweet received")
			collectedTweets = append(collectedTweets, tweet)
		case <-ticker.C:
			if len(collectedTweets) == 0 {
				e.log.Info("skip edit, because no tweets")
				continue
			}

			if err := e.edit(context.Background(), collectedTweets); err != nil {
				e.log.WithError(err).Error("edit error")
				cancel()
			}

			collectedTweets = make([]string, 0)
		}
	}
}

func (e *editor) edit(ctx context.Context, tweets []string) error {
	request := fmt.Sprintf(prompt, strings.Join(tweets, "\n"))

	resp, err := e.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Model: openai.GPT4TurboPreview,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: request,
				},
			},
		},
	)

	if err != nil {
		e.log.WithError(err).Error("summary generation error")
		return err
	}

	e.log.WithField("response", resp).Debug("summary generation result")

	res := struct {
		Message string `json:"message"`
	}{}

	if err = jsoniter.UnmarshalFromString(resp.Choices[0].Message.Content, &res); err != nil {
		// TODO: try to search correct json in string
		e.log.WithError(err).Error("summary unmarshal error")
		e.editedCh <- utils.Escape(resp.Choices[0].Message.Content)

		return nil
	}

	e.editedCh <- utils.Escape(res.Message)

	return nil
}

func NewEditor(client *openai.Client, sendInterval time.Duration, log log.Logger) Editor {
	return &editor{
		editedCh:     make(chan string, 10),
		sendInterval: sendInterval,
		client:       client,
		log:          log,
	}
}
