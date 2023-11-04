package log

import (
	"github.com/sirupsen/logrus"
)

// Logger abstruct interface for internal logging
type Logger interface {
	Error(msgs ...interface{})
	Warn(msgs ...interface{})
	Info(msgs ...interface{})
	Debug(msgs ...interface{})
	Trace(msgs ...interface{})
	Tracef(s string, msgs ...interface{})
	WithError(err error) Logger
	WithField(key string, value interface{}) Logger
}

type logger struct {
	log *logrus.Entry
}

func (l *logger) Warn(msgs ...interface{}) {
	l.log.Warn(msgs...)
}

func (l *logger) Tracef(s string, msgs ...interface{}) {
	l.log.Tracef(s, msgs...)
}

func (l *logger) WithField(key string, value interface{}) Logger {
	return NewLoggerFromEntry(l.log.WithField(key, value))
}

func (l *logger) WithError(err error) Logger {
	return NewLoggerFromEntry(l.log.WithError(err))
}

func (l *logger) Error(msgs ...interface{}) {
	l.log.Error(msgs...)
}

func (l *logger) Info(msgs ...interface{}) {
	l.log.Info(msgs...)
}

func (l *logger) Debug(msgs ...interface{}) {
	l.log.Debug(msgs...)
}

func (l *logger) Trace(msgs ...interface{}) {
	l.log.Trace(msgs...)
}

// NewLogger construct Logger from logrus.Logger
func NewLogger(log *logrus.Logger) Logger {
	return &logger{log: logrus.NewEntry(log)}
}

// NewLoggerFromEntry construct Logger from logrus.Entry
func NewLoggerFromEntry(log *logrus.Entry) Logger {
	return &logger{log: log}
}
