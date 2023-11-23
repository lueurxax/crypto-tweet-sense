package common

type ChannelMessage struct {
	ID       int
	Tweet    *Tweet
	Likes    map[string]int
	Dislikes map[string]int
}
