package common

import "errors"

var ErrAllTweetsAreFresh = errors.New("all tweets are fresh")
var ErrRatingNotFound = errors.New("rating for this user not found")
