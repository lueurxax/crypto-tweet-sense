package tweetseditor

import (
	"context"
	"fmt"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/sashabaranov/go-openai"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/pkg/utils"
)

const (
	prompt     = "I have several popular crypto tweets today. Can you extract information useful for cryptocurrency investing from these tweets and make summary? Skip information such as airdrops or giveaway, if they are not useful for investing. I will parse your answer by code like json `{\"tweets\":[{\"telegram_message\":\"summarized message by tweet\", \"link\":\"link to tweet\"}], \"new_information\":true}`, then can you prepare messages in json with prepared telegram message? \nTweets: %s." //nolint:lll
	nextPrompt = "Additional tweets, create new message only for new information: %s."                                                                                                                                                                                                                                                                                                                                                                                                                                //nolint:lll
	queueLen   = 10
)

type Tweet struct {
	Content string `json:"telegram_message"`
	Link    string `json:"link"`
}

type Editor interface {
	Edit(ctx context.Context, tweetCh <-chan *common.Tweet) context.Context
	SubscribeEdited() <-chan string
}

type editor struct {
	editedCh      chan string
	client        *openai.Client
	sendInterval  time.Duration
	cleanInterval time.Duration
	existMessages []openai.ChatCompletionMessage

	log log.Logger
}

func (e *editor) Edit(ctx context.Context, tweetCh <-chan *common.Tweet) context.Context {
	go e.editLoop(ctx, tweetCh)

	return ctx
}

func (e *editor) SubscribeEdited() <-chan string {
	return e.editedCh
}

func (e *editor) editLoop(ctx context.Context, ch <-chan *common.Tweet) {
	collectedTweets := make([]*common.Tweet, 0)
	ticker := time.NewTicker(e.sendInterval)
	contextCleanerTicker := time.NewTicker(e.sendInterval)

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

			if err := e.edit(context.Background(), collectedTweets); err != nil { //nolint:contextcheck
				e.log.WithError(err).Error("edit error")
			}

			collectedTweets = make([]*common.Tweet, 0)
		case <-contextCleanerTicker.C:
			if len(e.existMessages) > 0 {
				e.existMessages = e.existMessages[:1]
			}
		}
	}
}

func (e *editor) edit(ctx context.Context, tweets []*common.Tweet) error {
	tweetsStr := ""

	for _, twee := range tweets {
		text := twee.Text + "Link - " + twee.PermanentURL
		tweetsStr = strings.Join([]string{tweetsStr, text}, "\n")
	}

	request := ""
	if len(e.existMessages) == 0 {
		request = fmt.Sprintf(prompt, tweetsStr)
	} else {
		request = fmt.Sprintf(nextPrompt, tweetsStr)
	}

	e.existMessages = append(e.existMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: request,
	})

	resp, err := e.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Model:    openai.GPT4TurboPreview,
			Messages: e.existMessages,
		},
	)

	if err != nil {
		e.log.WithError(err).Error("summary generation error")
		return err
	}

	e.existMessages = append(e.existMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp.Choices[0].Message.Content,
	})

	e.log.WithField("response", resp).Debug("summary generation result")

	res := struct {
		Tweets         []Tweet `json:"tweets"`
		NewInformation bool    `json:"new_information"`
	}{}

	if err = jsoniter.UnmarshalFromString(resp.Choices[0].Message.Content, &res); err != nil {
		// TODO: try to search correct json in string
		e.log.WithError(err).Error("summary unmarshal error")
		e.editedCh <- utils.Escape(resp.Choices[0].Message.Content)

		return nil
	}

	if !res.NewInformation {
		return nil
	}

	data := ""
	for _, el := range res.Tweets {
		data = fmt.Sprintf("%s\n%s", data, e.formatTweet(el))
	}

	e.editedCh <- strings.Trim(data, "\n")

	return nil
}

func (e *editor) formatTweet(tweet Tweet) (str string) {
	str += fmt.Sprintf("%s\n", utils.Escape(tweet.Content))

	str += fmt.Sprintf("[link](%s)\n", utils.Escape(tweet.Link))

	return
}

func NewEditor(client *openai.Client, sendInterval, cleanInterval time.Duration, log log.Logger) Editor {
	return &editor{
		editedCh:      make(chan string, queueLen),
		sendInterval:  sendInterval,
		cleanInterval: cleanInterval,
		client:        client,
		log:           log,
	}
}
