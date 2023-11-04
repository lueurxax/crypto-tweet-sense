package rating_collector

import (
	"fmt"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/lueurxax/crypto-tweet-sense/internal/rating_collector/models"
)

type parser struct {
}

func (p *parser) ParseLink(message *tg.Message) (string, error) {
	for _, ent := range message.Entities {
		textURL, ok := ent.(*tg.MessageEntityTextURL)
		if !ok {
			continue
		}
		if !strings.Contains(textURL.URL, "https://twitter.com/") {
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
		cutted := strings.ReplaceAll(textURL.URL, "https://twitter.com/", "")
		splitted := strings.Split(cutted, "/")
		if len(splitted) == 0 {
			return "", fmt.Errorf("can't parse username")
		}
		return splitted[0], nil
	}
	return "", models.ErrUsernameNotFound
}

func newParser() messageParser {
	return &parser{}
}
