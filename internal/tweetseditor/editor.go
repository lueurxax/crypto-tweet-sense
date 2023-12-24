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
	prompt     = "I have several popular crypto tweets today. Can you extract information useful for cryptocurrency investing from these tweets and make summary? Skip information such as airdrops or giveaway, if they are not useful for investing. I will parse your answer by code like json `{\"tweets\":[{\"telegram_message\":\"summarized message by tweet\", \"link\":\"link to tweet\", \"useful_information\":true, \"duplicate_information\": false}]}`, then can you prepare messages in json with prepared telegram message? \nTweets: %s." //nolint:lll
	nextPrompt = "Additional tweets, create new message only for new information: %s."                                                                                                                                                                                                                                                                                                                                                                                                                                                                     //nolint:lll

	longStoryPrompt     = "Analyze several recent, popular tweets related to cryptocurrency. Extract key insights relevant for cryptocurrency investment, excluding non-investment related content like airdrops or giveaways. Summarize the useful information from each tweet. Format the summaries in json with fields: 'telegram_message' for the summary, 'useful_information' to indicate if the information is investment-relevant (true/false), and 'duplicate_information' to indicate if the information is repetitive (true/false), for example {\"telegram_message\":\"summarized message by tweet\", \"useful_information\":true, \"duplicate_information\": false}. \nTweets: %s." //nolint:lll
	longStoryNextPrompt = "Additional tweets, create new message only for new information: %s."

	queueLen = 10
)

type Tweet struct {
	Content   string `json:"telegram_message"`
	Link      string `json:"link"`
	Useful    bool   `json:"useful_information"`
	Duplicate bool   `json:"duplicate_information"`
}

type LongStoryMessage struct {
	Content   string `json:"telegram_message"`
	Useful    bool   `json:"useful_information"`
	Duplicate bool   `json:"duplicate_information"`
}

type repo interface {
	GetTweetForEdit(ctx context.Context) ([]common.Tweet, error)
	DeleteEditedTweets(ctx context.Context, ids []string) error
}

type Editor interface {
	Edit(ctx context.Context) context.Context
	SubscribeEdited() <-chan string
	SubscribeLongStoryMessages() <-chan string
}

type editor struct {
	editedCh      chan string
	client        *openai.Client
	sendInterval  time.Duration
	cleanInterval time.Duration
	existMessages []openai.ChatCompletionMessage

	longStoryEditedCh chan string
	longStoryBuffer   [20]common.Tweet
	longStoryIndex    uint8
	longStoryMessages []openai.ChatCompletionMessage

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

func (e *editor) SubscribeLongStoryMessages() <-chan string {
	return e.longStoryEditedCh
}

func (e *editor) editLoop(ctx context.Context) {
	ticker := time.NewTicker(e.sendInterval)
	contextCleanerTicker := time.NewTicker(e.sendInterval * 10)

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

			if err = e.longStoryProcess(ctx, collectedTweets); err != nil {
				e.log.WithError(err).Error("long story process error")
			}
		case <-contextCleanerTicker.C:
			if len(e.existMessages) > 0 {
				e.existMessages = make([]openai.ChatCompletionMessage, 0)
			}
			if len(e.longStoryMessages) > 0 {
				e.longStoryMessages = make([]openai.ChatCompletionMessage, 0)
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
		Tweets []Tweet `json:"tweets"`
	}{}

	if err = jsoniter.UnmarshalFromString(resp.Choices[0].Message.Content, &res); err != nil {
		// TODO: try to search correct json in string
		e.log.WithError(err).Error("summary unmarshal error")
		e.editedCh <- utils.Escape(resp.Choices[0].Message.Content)

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

			e.editedCh <- e.formatTweet(tweet, el.Content)
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

func (e *editor) longStoryProcess(ctx context.Context, tweets []common.Tweet) error {
	for _, tweet := range tweets {
		e.longStoryBuffer[e.longStoryIndex] = tweet
		e.longStoryIndex++

		if e.longStoryIndex == 20 {
			err := e.longStorySend(ctx)

			e.longStoryIndex = 0

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *editor) longStorySend(ctx context.Context) error {
	tweetsStr := ""

	for _, twee := range e.longStoryBuffer {
		text := twee.Text + "Link - " + twee.PermanentURL
		tweetsStr = strings.Join([]string{tweetsStr, text}, "\n")
	}

	request := ""
	if len(e.longStoryMessages) == 0 {
		request = fmt.Sprintf(longStoryPrompt, tweetsStr)
	} else {
		request = fmt.Sprintf(longStoryNextPrompt, tweetsStr)
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
			Messages: append(e.longStoryMessages, requestMessage),
		},
	)

	if err != nil {
		e.log.WithError(err).Error("long story summary generation error")
		return err
	}

	e.log.WithField("response", resp).Debug("long story summary generation result")

	res := LongStoryMessage{}

	if err = jsoniter.UnmarshalFromString(resp.Choices[0].Message.Content, &res); err != nil {
		// TODO: try to search correct json in string
		e.log.WithError(err).Error("long story summary unmarshal error")
		e.longStoryEditedCh <- utils.Escape(resp.Choices[0].Message.Content)

		return err
	}

	if !res.Useful || res.Duplicate {
		return nil
	}

	e.longStoryEditedCh <- utils.Escape(res.Content)

	e.existMessages = append(e.existMessages, requestMessage, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp.Choices[0].Message.Content,
	})

	return nil
}

func NewEditor(client *openai.Client, db repo, sendInterval, cleanInterval time.Duration, log log.Logger) Editor {
	return &editor{
		editedCh:          make(chan string, queueLen),
		client:            client,
		sendInterval:      sendInterval,
		cleanInterval:     cleanInterval,
		existMessages:     make([]openai.ChatCompletionMessage, 0),
		longStoryEditedCh: make(chan string, queueLen),
		longStoryBuffer:   [20]common.Tweet{},
		longStoryMessages: make([]openai.ChatCompletionMessage, 0),
		repo:              db,
		log:               log,
	}
}
