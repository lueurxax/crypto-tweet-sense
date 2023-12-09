package fdb

import "errors"

var ErrTweetsNotFound = errors.New("no tweets found")
var ErrRequestLimitsNotFound = errors.New("no request limits found")
var ErrRequestLimitsUnmarshallingError = errors.New("request limits unmarshalling error")
var ErrAlreadyExists = errors.New("already exists")
var ErrTwitterAccountNotFound = errors.New("no twitter account found")
var ErrCookieNotFound = errors.New("no cookie found")
