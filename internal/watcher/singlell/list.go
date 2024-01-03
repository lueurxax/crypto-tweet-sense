package singlell

import "sync"

type SingleLL[T any] interface {
	Push(n T)
	Pop() (T, bool)
}

type node[T any] struct {
	next  *node[T]
	value T
}

type singleLL[T any] struct {
	mu    sync.Mutex
	first *node[T]
	last  *node[T]
}

func (l *singleLL[T]) Push(v T) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := &node[T]{value: v}
	if l.first == nil {
		l.first = n
		l.last = n
		return
	}

	l.last.next = n
	l.last = n
}

func (l *singleLL[T]) Pop() (T, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.first == nil {
		var result T
		return result, false
	}

	n := l.first
	l.first = n.next
	return n.value, true
}

func New[T any]() SingleLL[T] {
	return &singleLL[T]{}
}
