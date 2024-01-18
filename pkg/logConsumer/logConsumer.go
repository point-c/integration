package logConsumer

import (
	"bytes"
	"github.com/testcontainers/testcontainers-go"
	"slices"
	"sync"
)

type LogConsumer struct {
	l              sync.RWMutex
	Stdout, Stderr [][]byte
}

func (lc *LogConsumer) Accept(l testcontainers.Log) {
	lc.l.Lock()
	defer lc.l.Unlock()
	switch l.LogType {
	case "STDERR":
		lc.Stderr = append(lc.Stderr, slices.Clone(l.Content))
	case "STDOUT":
		lc.Stdout = append(lc.Stdout, slices.Clone(l.Content))
	}
}

func (lc *LogConsumer) StdoutLen() int {
	lc.l.RLock()
	defer lc.l.RUnlock()
	return len(lc.Stdout)
}

func (lc *LogConsumer) StderrLen() int {
	lc.l.RLock()
	defer lc.l.RUnlock()
	return len(lc.Stderr)
}

func (lc *LogConsumer) StdoutFile() []byte {
	lc.l.RLock()
	defer lc.l.RUnlock()
	return lc.outfile(lc.Stdout)
}

func (lc *LogConsumer) StderrFile() []byte {
	lc.l.RLock()
	defer lc.l.RUnlock()
	return lc.outfile(lc.Stderr)
}

func (*LogConsumer) outfile(b [][]byte) []byte { return bytes.Join(b, []byte("\n")) }
