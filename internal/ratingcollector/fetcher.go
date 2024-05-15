package ratingcollector

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"golang.org/x/term"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/ratingcollector/models"
)

const (
	likeSymbol    = "👍"
	dislikeSymbol = "👎"

	limit        = 1000
	reactionsKey = "reactions"
	messageKey   = "message"
)

type Fetcher interface {
	Auth(ctx context.Context) error
	Stop() error
	FetchRatingsAndSave(ctx context.Context, id int64) error
	SubscribeAndSave(ctx context.Context, id int64)
}

type messageParser interface {
	ParseUsername(message *tg.Message) (string, error)
	ParseLink(message *tg.Message) (string, error)
}

type repo interface {
	SaveRatings(ctx context.Context, ratings []common.UsernameRating) error
	SaveSentTweet(ctx context.Context, link string) error
	GetRating(ctx context.Context, username string) (common.Rating, error)
}

type sessionRepo interface {
	LoadSession(ctx context.Context) ([]byte, error)
	StoreSession(ctx context.Context, data []byte) error
}

type fetcher struct {
	client           *telegram.Client
	gaps             *updates.Manager
	updateDispatcher tg.UpdateDispatcher

	repo

	messageParser

	phone string

	log  log.Logger
	stop bg.StopFunc
}

func (f *fetcher) FetchRatingsAndSave(ctx context.Context, id int64) error {
	chls, err := f.client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{Limit: limit, OffsetPeer: &tg.InputPeerEmpty{}})
	if err != nil {
		return err
	}

	var (
		ch *tg.Channel
		ok bool
	)

	dialogs, ok := chls.(*tg.MessagesDialogsSlice)
	if !ok {
		return models.ErrIncorrectTypeOfResponse
	}

	for _, chl := range dialogs.Chats {
		ch, ok = chl.(*tg.Channel)
		if ok && ch.ID == id {
			break
		}
	}

	offset := 0

	const limit = 100

	ratingsMap := map[string]int{}

	ratings := make([]common.UsernameRating, 0)

	for {
		raw, err := f.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer: &tg.InputPeerChannel{
				ChannelID:  ch.ID,
				AccessHash: ch.AccessHash,
			},
			Limit:     limit,
			AddOffset: offset,
		})
		if err != nil {
			return err
		}

		messages, ok := raw.(*tg.MessagesChannelMessages)
		if !ok {
			return models.ErrIncorrectTypeOfResponse
		}

		for _, m := range messages.Messages {
			tgmes, ok := m.(*tg.Message)
			if !ok {
				continue
			}

			link, err := f.messageParser.ParseLink(tgmes)
			if err != nil {
				if errors.Is(err, models.ErrLinkNotFound) {
					continue
				}

				return err
			}

			if err = f.repo.SaveSentTweet(ctx, link); err != nil {
				return err
			}

			username, err := f.messageParser.ParseUsername(tgmes)
			if err != nil {
				if errors.Is(err, models.ErrUsernameNotFound) {
					continue
				}

				return err
			}

			likes, dislikes := f.parseReactions(tgmes.Reactions.Results)

			index, ok := ratingsMap[username]
			if !ok {
				ratings = append(ratings, common.UsernameRating{Username: username, Rating: &common.Rating{}})
				index = len(ratings) - 1
				ratingsMap[username] = index
			}

			ratings[index].Likes += likes
			ratings[index].Dislikes += dislikes
		}

		if len(messages.Messages) < limit {
			break
		}

		offset += limit

		time.Sleep(time.Second)
	}

	return f.repo.SaveRatings(ctx, ratings)
}

func (f *fetcher) SubscribeAndSave(_ context.Context, id int64) {
	f.updateDispatcher.OnMessageReactions(func(ctx context.Context, e tg.Entities, update *tg.UpdateMessageReactions) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		peer, ok := update.Peer.(*tg.PeerChannel)
		if !ok {
			f.log.WithField(reactionsKey, update.Reactions).WithField(messageKey, update.MsgID).Info("Not a channel")
			return nil
		}

		if peer.ChannelID != id {
			return nil
		}

		channel := e.Channels[peer.ChannelID]
		raw, err := f.client.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash},
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: update.MsgID}},
		})
		if err != nil {
			return err
		}
		msgs, isCorrect := raw.AsModified()
		if !isCorrect {
			f.log.WithField(reactionsKey, update.Reactions).WithField(messageKey, update.MsgID).Info("Not a message")
			return nil
		}
		f.log.WithField(reactionsKey, update.Reactions).WithField("messages", msgs.GetMessages()).Info("Reactions")

		for _, m := range msgs.GetMessages() {
			tgmes, ok := m.(*tg.Message)
			if !ok {
				continue
			}
			username, err := f.messageParser.ParseUsername(tgmes)
			if err != nil {
				if errors.Is(err, models.ErrUsernameNotFound) {
					continue
				}
				return err
			}
			likes, dislikes := f.parseReactions(tgmes.Reactions.Results)
			rating := &common.UsernameRating{
				Username: username,
				Rating: &common.Rating{
					Likes:    likes,
					Dislikes: dislikes,
				},
			}
			if err = f.repo.SaveRatings(ctx, []common.UsernameRating{*rating}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (f *fetcher) parseReactions(results []tg.ReactionCount) (likes, dislikes int) {
	if results == nil {
		return
	}

	for _, res := range results {
		reactionEmoji, ok := res.Reaction.(*tg.ReactionEmoji)
		if !ok {
			continue
		}

		emogicon := reactionEmoji.Emoticon
		if emogicon == likeSymbol {
			likes = res.Count
		}

		if emogicon == dislikeSymbol {
			dislikes = res.Count
		}
	}

	return
}

func (f *fetcher) Auth(ctx context.Context) (err error) {
	// Setting up authentication flow helper based on terminal auth.
	flow := auth.NewFlow(
		termAuth{phone: f.phone},
		auth.SendCodeOptions{},
	)
	// bg.Connect will call Run in background.
	// Call stop() to disconnect and release resources.
	f.stop, err = bg.Connect(f.client)
	if err != nil {
		return err
	}

	if err = f.client.Auth().IfNecessary(ctx, flow); err != nil {
		return err
	}

	// Fetch user info.
	user, err := f.client.Self(ctx)
	if err != nil {
		return err
	}

	go f.run(ctx, user)

	return nil
}

func (f *fetcher) Stop() error {
	f.gaps.Reset()
	return f.stop()
}

func (f *fetcher) run(ctx context.Context, user *tg.User) {
	// Notify update manager about authentication.
	if err := f.gaps.Run(ctx, f.client.API(), user.ID, updates.AuthOptions{IsBot: user.Bot}); err != nil {
		f.log.WithError(err).Warn("Update manager error")
	}
}

func NewFetcher(appID int, appHash, phone string, repo repo, sessionRepo sessionRepo, logger log.Logger) Fetcher {
	d := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: d,
	})

	zapLogger, err := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()
	if err != nil {
		logger.Error(err)
	}

	client := telegram.NewClient(appID, appHash, telegram.Options{
		UpdateHandler: gaps,
		Middlewares: []telegram.Middleware{
			hook.UpdateHook(gaps.Handle),
		},
		SessionStorage: sessionRepo,
		Logger:         zapLogger,
	})

	return &fetcher{
		client:           client,
		gaps:             gaps,
		updateDispatcher: d,
		repo:             repo,
		messageParser:    newParser(),
		phone:            phone,
		log:              logger,
	}
}

// noSignUp can be embedded to prevent signing up.
type noSignUp struct{}

func (c noSignUp) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, models.ErrNotImplemented
}

func (c noSignUp) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

// termAuth implements authentication via terminal.
type termAuth struct {
	noSignUp

	phone string
}

func (a termAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a termAuth) Password(_ context.Context) (string, error) {
	fmt.Print("Enter 2FA password: ")

	bytePwd, err := term.ReadPassword(0)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(bytePwd)), nil
}

func (a termAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter code: ")

	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(code), nil
}
