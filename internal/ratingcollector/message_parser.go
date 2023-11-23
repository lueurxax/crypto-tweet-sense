package ratingcollector

import (
	"strings"

	"github.com/gotd/td/tg"

	"github.com/lueurxax/crypto-tweet-sense/internal/ratingcollector/models"
)

const twitterURL = "https://twitter.com/"

type parser struct {
}

func (p *parser) ParseLink(message *tg.Message) (string, error) {
	for _, ent := range message.Entities {
		textURL, ok := ent.(*tg.MessageEntityTextURL)
		if !ok {
			continue
		}

		if !strings.Contains(textURL.URL, twitterURL) {
			continue
		}

		return textURL.URL, nil
	}

	return "", models.ErrLinkNotFound
}

func (p *parser) ParseUsername(message *tg.Message) (string, error) {
	for _, ent := range message.Entities {
		textURL, ok := ent.(*tg.MessageEntityTextURL)
		if !ok {
			continue
		}

		cutted := strings.ReplaceAll(textURL.URL, twitterURL, "")

		splitted := strings.Split(cutted, "/")
		if len(splitted) == 0 {
			return "", models.ErrCantParseUsername
		}

		return splitted[0], nil
	}

	return "", models.ErrUsernameNotFound
}

func newParser() messageParser {
	return &parser{}
}
