package frame

import "sync"

type Latest struct {
	mu      sync.Mutex
	frame   *Frame
	version uint64
}

func NewLatest() *Latest {
	return &Latest{}
}

func (l *Latest) Publish(f *Frame) {
	l.mu.Lock()
	l.frame = f
	l.version++
	l.mu.Unlock()
}

func (l *Latest) GetFrame() *Frame {
	l.mu.Lock()
	f := l.frame
	// l.frame = nil
	l.mu.Unlock()

	return f
}

func (l *Latest) Version() uint64 {
	l.mu.Lock()
	version := l.version
	l.mu.Unlock()

	return version
}
