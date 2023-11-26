package fdb

import "errors"

var ErrTweetsNotFound = errors.New("no tweets found")
var ErrRequestLimitsNotFound = errors.New("no request limits found")
var ErrAlreadyExists = errors.New("already exists")
var ErrTelegramSessionNotFound = errors.New("no telegram session found")
