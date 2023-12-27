package tweetfinder

import "errors"

var (
	ErrNoTops              = errors.New("no top tweets")
	ErrNotFound            = errors.New("not found")
	ErrTimeoutSelectFinder = errors.New("timeout select finder")
)
