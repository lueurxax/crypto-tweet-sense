package common

import (
	"testing"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

func TestTweetSnapshot_String(t1 *testing.T) {
	t1.Run("logrus nested formatter", func(t1 *testing.T) {
		// init logger
		logrusLogger := logrus.New()
		logrusLogger.SetFormatter(&nested.Formatter{
			TimestampFormat: "01-02|15:04:05",
		})

		logger := log.NewLogger(logrusLogger)
		t := TweetSnapshot{
			Tweet: &Tweet{
				ID:         "123",
				TimeParsed: time.Now(),
			},
			RatingGrowSpeed: 1.345345,
			CheckedAt:       time.Now(),
		}
		logger.WithField("tweet", t).Info("test")
	})
}
