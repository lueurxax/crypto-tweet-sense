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
	prompt     = "I have several popular crypto tweets today. Can you extract information useful for cryptocurrency investing from these tweets and make summary? Skip information such as airdrops or giveaway, if they are not useful for investing. I will parse your answer by code like json `{\"tweets\":[{\"telegram_message\":\"summarized message by tweet\", \"link\":\"link to tweet\", \"useful_information\":true, \"duplicate_information\": false}]}`, then can you prepare messages in json with prepared telegram message? \nTweets: %s." //nolint:lll
	nextPrompt = "Additional tweets, create new message only for new information: %s."                                                                                                                                                                                                                                                                                                                                                                                                                                                                     //nolint:lll

	longStorySystem = `User has some popular crypto tweets.
Can you extract information useful for cryptocurrency investing from these tweets and make summary?
Skip information such as airdrops or giveaway, if they are not useful for investing.
Deduplicate information, summarize only new information. Format message for telegram channel.`
	russianPrompt = "Translate to russian this text: "

	queueLen = 100
)

type Tweet struct {
	Content   string `json:"telegram_message"`
	Link      string `json:"link"`
	Useful    bool   `json:"useful_information"`
	Duplicate bool   `json:"duplicate_information"`
}

type Editor interface {
	Edit(ctx context.Context, tweets []common.Tweet, out chan string) error
	EditLongStory(ctx context.Context, tweets []common.Tweet, out chan string) (string, error)
	TranslateLongStory(ctx context.Context, content string) (string, error)
	Clean()
}

type editor struct {
	client            *openai.Client
	existMessages     []openai.ChatCompletionMessage
	longStoryMessages []openai.ChatCompletionMessage

	log log.Logger
}

func (e *editor) EditLongStory(ctx context.Context, tweets []common.Tweet, out chan string) (string, error) {
	tweetsStr := tweets[0].Text

	for _, twee := range tweets[1:] {
		tweetsStr = strings.Join([]string{tweetsStr, twee.Text}, "\n")
	}

	e.log.WithField("tweets", tweetsStr).Debug("long story summary generation request")

	requestMessages := make([]openai.ChatCompletionMessage, 0)
	if len(e.longStoryMessages) == 0 {
		requestMessages = append(requestMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: longStorySystem,
		})
	}

	requestMessages = append(requestMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: tweetsStr,
	})

	resp, err := e.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    openai.GPT4TurboPreview,
			Messages: append(e.longStoryMessages, requestMessages...),
		},
	)

	if err != nil {
		e.log.WithError(err).Error("long story summary generation error")
		return "", err
	}

	e.log.WithField("response", resp).Debug("long story summary generation result")

	out <- utils.Escape(resp.Choices[0].Message.Content)

	e.longStoryMessages = append(append(e.longStoryMessages, requestMessages...), openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp.Choices[0].Message.Content,
	})

	return resp.Choices[0].Message.Content, nil
}

func (e *editor) Edit(ctx context.Context, tweets []common.Tweet, out chan string) error {
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
		Tweets []Tweet `json:"tweets"`
	}{}

	if err = jsoniter.UnmarshalFromString(resp.Choices[0].Message.Content, &res); err != nil {
		// TODO: try to search correct json in string
		e.log.WithError(err).Error("summary unmarshal error")
		out <- utils.Escape(resp.Choices[0].Message.Content)

		return nil
	}

	usefulInformation := false

	for _, el := range res.Tweets {
		tweet, ok := tweetsMap[el.Link]
		if ok {
			if !el.Useful {
				e.log.WithField("tweet", el.Link).Debug("skip tweet, no new useful information")
				continue
			}

			if el.Duplicate {
				e.log.WithField("tweet", el.Link).Debug("skip tweet, duplicate information")
				continue
			}

			usefulInformation = true

			out <- e.formatTweet(tweet, el.Content)
		}
	}

	if usefulInformation {
		e.existMessages = append(e.existMessages, requestMessage, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: resp.Choices[0].Message.Content,
		})
	}

	return nil
}

func (e *editor) TranslateLongStory(ctx context.Context, content string) (string, error) {
	requestMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: russianPrompt + content,
	}

	resp, err := e.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    openai.GPT4TurboPreview,
			Messages: []openai.ChatCompletionMessage{requestMessage},
		},
	)

	if err != nil {
		e.log.WithError(err).Error("rus long story summary generation error")
		return "", err
	}

	e.log.WithField("response", resp).Debug("rus long story summary generation result")

	return utils.Escape(resp.Choices[0].Message.Content), nil
}

func (e *editor) Clean() {
	if len(e.existMessages) > 0 {
		e.existMessages = make([]openai.ChatCompletionMessage, 0)
	}

	if len(e.longStoryMessages) > 0 {
		e.longStoryMessages = make([]openai.ChatCompletionMessage, 0)
	}
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

func NewEditor(client *openai.Client, log log.Logger) Editor {
	return &editor{
		client:            client,
		existMessages:     make([]openai.ChatCompletionMessage, 0),
		longStoryMessages: make([]openai.ChatCompletionMessage, 0),
		log:               log,
	}
}
