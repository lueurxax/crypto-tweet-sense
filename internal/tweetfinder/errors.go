package tweetfinder

import "errors"

var (
	ErrNoTops   = errors.New("no top tweets")
	ErrNotFound = errors.New("not found")
)
