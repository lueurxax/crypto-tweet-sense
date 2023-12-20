package keys

type prefix [2]byte

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
	versionPrefix                prefix = [2]byte{0x00, 0x00}
	tweetPrefix                  prefix = [2]byte{0x00, 0x01}
	sentTweetPrefix              prefix = [2]byte{0x00, 0x03}
	tweetRatingPrefix            prefix = [2]byte{0x00, 0x04}
	editingTweetPrefix           prefix = [2]byte{0x00, 0x05}
	telegramSessionStoragePrefix prefix = [2]byte{0x00, 0x06}
	twitterAccountsPrefix        prefix = [2]byte{0x00, 0x07}
	twitterAccountsCookiePrefix  prefix = [2]byte{0x00, 0x08}
	requestLimitPrefix           prefix = [2]byte{0x00, 0x09}
	requestLimitV2Prefix         prefix = [2]byte{0x00, 0x10}
	requestsPrefix               prefix = [2]byte{0x00, 0x11}
	tweetRatingIndexPrefix       prefix = [2]byte{0x00, 0x12}
	tweetCreationIndexPrefix     prefix = [2]byte{0x00, 0x13}
	tweetCreationIndexV2Prefix   prefix = [2]byte{0x00, 0x14}

	oldToNewPrefixes = map[string]prefix{
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
