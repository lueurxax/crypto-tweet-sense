package rating_collector

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"golang.org/x/term"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/rating_collector/models"
)

const (
	likeSymbol    = "👍"
	dislikeSymbol = "👎"
)

type Fetcher interface {
	Auth(ctx context.Context) error
	Stop() error
	FetchRatingsAndUniqueMessages(ctx context.Context, id int64) (map[string]*models.Rating, map[string]struct{}, error)
	Subscribe(ctx context.Context, id int64) chan *models.UsernameRating
}

type messageParser interface {
	ParseUsername(message *tg.Message) (string, error)
	ParseLink(message *tg.Message) (string, error)
}

type fetcher struct {
	client           *telegram.Client
	gaps             *updates.Manager
	updateDispatcher tg.UpdateDispatcher

	messageParser

	phone string

	log  log.Logger
	stop bg.StopFunc
}

func (f *fetcher) Subscribe(_ context.Context, id int64) chan *models.UsernameRating {
	res := make(chan *models.UsernameRating, 1000)
	f.updateDispatcher.OnMessageReactions(func(ctx context.Context, e tg.Entities, update *tg.UpdateMessageReactions) error {
		select {
		case <-ctx.Done():
			close(res)
			return ctx.Err()
		default:
		}
		peer, ok := update.Peer.(*tg.PeerChannel)
		if !ok {
			f.log.WithField("reactions", update.Reactions).WithField("message", update.MsgID).Info("Reactions")
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
			f.log.WithField("reactions", update.Reactions).WithField("message", update.MsgID).Info("Reactions")
			return nil
		}
		f.log.WithField("reactions", update.Reactions).WithField("messages", msgs.GetMessages()).Info("Reactions")

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
			rating := &models.UsernameRating{
				Username: username,
				Rating: &models.Rating{
					Likes:    likes,
					Dislikes: dislikes,
				},
			}
			res <- rating
		}

		return nil
	})
	return res
}

func (f *fetcher) FetchRatingsAndUniqueMessages(ctx context.Context, id int64) (map[string]*models.Rating, map[string]struct{}, error) {
	chls, err := f.client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{Limit: 1000, OffsetPeer: &tg.InputPeerEmpty{}})
	if err != nil {
		return nil, nil, err
	}
	var (
		ch *tg.Channel
		ok bool
	)
	for _, chl := range chls.(*tg.MessagesDialogsSlice).Chats {
		ch, ok = chl.(*tg.Channel)
		if ok {
			if ch.ID == id {
				break
			}
		}
	}

	offset := 0
	const limit = 100
	ratings := map[string]*models.Rating{}
	uniqueLinks := map[string]struct{}{}
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
			return nil, nil, err
		}
		messages, ok := raw.(*tg.MessagesChannelMessages)
		if !ok {
			return nil, nil, fmt.Errorf("incorrect type of response")
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
				return nil, nil, err
			}
			uniqueLinks[link] = struct{}{}
			username, err := f.messageParser.ParseUsername(tgmes)
			if err != nil {
				if errors.Is(err, models.ErrUsernameNotFound) {
					continue
				}
				return nil, nil, err
			}
			likes, dislikes := f.parseReactions(tgmes.Reactions.Results)
			rating, ok := ratings[username]
			if !ok {
				rating = &models.Rating{}
			}
			rating.Likes += likes
			rating.Dislikes += dislikes

			ratings[username] = rating
		}
		if len(messages.Messages) < limit {
			break
		}
		offset += 100
		time.Sleep(time.Second)
	}

	return ratings, uniqueLinks, nil
}

func (f *fetcher) parseReactions(results []tg.ReactionCount) (likes, dislikes int) {
	if results == nil {
		return
	}
	for _, res := range results {
		emogicon := res.Reaction.(*tg.ReactionEmoji).Emoticon
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

func NewFetcher(appID int, appHash, phone, sessionFile string, logger log.Logger) Fetcher {
	d := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: d,
	})

	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		logger.Error(err)
	}

	client := telegram.NewClient(appID, appHash, telegram.Options{
		UpdateHandler: gaps,
		Middlewares: []telegram.Middleware{
			hook.UpdateHook(gaps.Handle),
		},
		SessionStorage: &session.FileStorage{
			Path: sessionFile,
		},
		Logger: zapLogger,
	})

	return &fetcher{
		client:           client,
		gaps:             gaps,
		updateDispatcher: d,
		messageParser:    newParser(),
		phone:            phone,
		log:              logger,
	}
}

// noSignUp can be embedded to prevent signing up.
type noSignUp struct{}

func (c noSignUp) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("not implemented")
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
