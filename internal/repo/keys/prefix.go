package keys

type Prefix [2]byte

const (
	version                   = 'v'
	tweet                     = 't'
	sentTweet                 = 's'
	tweetRating               = 'a'
	editingTweet              = 'e'
	telegramSessionStorageOld = "telegramSessionStorage"
	twitterAccounts           = "_twitterAccounts"
	twitterAccountsCookie     = "_cookiesTwitterAccount"
	requestLimit              = "_requestLimit"
	requestLimitV2            = "_v2requestLimit"
	requests                  = "_requests"
	tweetRatingIndex          = "_tweetRatingIndex"
	tweetCreationIndex        = "_tweetCreationIndex"
	tweetCreationIndexV2      = "_tweetCreationNanoIndex"
)

var (
	// Prefixes for keys in the new format
	versionPrefix                Prefix = [2]byte{0x00, 0x00}
	tweetPrefix                  Prefix = [2]byte{0x00, 0x01}
	sentTweetPrefix              Prefix = [2]byte{0x00, 0x03}
	tweetRatingPrefix            Prefix = [2]byte{0x00, 0x04}
	editingTweetPrefix           Prefix = [2]byte{0x00, 0x05}
	telegramSessionStoragePrefix Prefix = [2]byte{0x00, 0x06}
	twitterAccountsPrefix        Prefix = [2]byte{0x00, 0x07}
	twitterAccountsCookiePrefix  Prefix = [2]byte{0x00, 0x08}
	requestLimitPrefix           Prefix = [2]byte{0x00, 0x09}
	requestLimitV2Prefix         Prefix = [2]byte{0x00, 0x10}
	requestsPrefix               Prefix = [2]byte{0x00, 0x11}
	tweetRatingIndexPrefix       Prefix = [2]byte{0x00, 0x12}
	tweetCreationIndexPrefix     Prefix = [2]byte{0x00, 0x13}
	tweetCreationIndexV2Prefix   Prefix = [2]byte{0x00, 0x14}

	OldToNewPrefixes = map[string]Prefix{
		string(version):           versionPrefix,
		string(tweet):             tweetPrefix,
		string(sentTweet):         sentTweetPrefix,
		string(tweetRating):       tweetRatingPrefix,
		string(editingTweet):      editingTweetPrefix,
		telegramSessionStorageOld: telegramSessionStoragePrefix,
		twitterAccounts:           twitterAccountsPrefix,
		twitterAccountsCookie:     twitterAccountsCookiePrefix,
		requestLimit:              requestLimitPrefix,
		requestLimitV2:            requestLimitV2Prefix,
		requests:                  requestsPrefix,
		tweetRatingIndex:          tweetRatingIndexPrefix,
		tweetCreationIndex:        tweetCreationIndexPrefix,
		tweetCreationIndexV2:      tweetCreationIndexV2Prefix,
	}
)
