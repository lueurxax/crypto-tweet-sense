package keys

type Prefix [2]byte

var (
	// Prefixes for keys in the new format
	versionPrefix                Prefix = [2]byte{0x00, 0x00}
	tweetPrefix                  Prefix = [2]byte{0x00, 0x01}
	sentTweetPrefix              Prefix = [2]byte{0x00, 0x03}
	tweetRatingPrefix            Prefix = [2]byte{0x00, 0x04}
	editingTweetShortPrefix      Prefix = [2]byte{0x00, 0x05}
	editingTweetLongPrefix       Prefix = [2]byte{0x00, 0x09}
	telegramSessionStoragePrefix Prefix = [2]byte{0x00, 0x06}
	twitterAccountsPrefix        Prefix = [2]byte{0x00, 0x07}
	twitterAccountsCookiePrefix  Prefix = [2]byte{0x00, 0x08}
	requestLimitPrefix           Prefix = [2]byte{0x00, 0x10}
	requestLimitUnzipPrefix      Prefix = [2]byte{0x00, 0x1a}
	requestsUnzipPrefix          Prefix = [2]byte{0x00, 0x1b}
	requestsPrefix               Prefix = [2]byte{0x00, 0x11}
	tweetRatingIndexPrefix       Prefix = [2]byte{0x00, 0x12}
	tweetCreationIndexPrefix     Prefix = [2]byte{0x00, 0x14}
)
