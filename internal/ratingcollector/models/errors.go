package models

import "errors"

var (
	ErrUsernameNotFound        = errors.New("username not found")
	ErrCantParseUsername       = errors.New("can't parse username")
	ErrLinkNotFound            = errors.New("link not found")
	ErrNotImplemented          = errors.New("not implemented")
	ErrIncorrectTypeOfResponse = errors.New("incorrect type of response")
)
