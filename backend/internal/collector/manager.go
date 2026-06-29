package collector

import (
	"context"
	"log"
	"sync"
	"time"
)

type Collector interface {
	Name() string
	Poll(ctx context.Context) error
}

type entry struct {
	c        Collector
	interval time.Duration
	mu       sync.Mutex
}

type Manager struct {
	entries []*entry
}

func (m *Manager) Add(c Collector, interval time.Duration) {
	m.entries = append(m.entries, &entry{c: c, interval: interval})
}

func (m *Manager) Start(ctx context.Context) {
	for _, e := range m.entries {
		e := e
		log.Printf("%s collector: every %s", e.c.Name(), e.interval)
		go func() {
			m.pollOne(ctx, e)
			t := time.NewTicker(e.interval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					m.pollOne(ctx, e)
				}
			}
		}()
	}
}

func (m *Manager) pollOne(ctx context.Context, e *entry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.c.Poll(ctx); err != nil {
		log.Printf("%s: poll: %v", e.c.Name(), err)
	}
}

func (m *Manager) PollNow(ctx context.Context) []string {
	var errs []string
	for _, e := range m.entries {
		e.mu.Lock()
		if err := e.c.Poll(ctx); err != nil {
			errs = append(errs, e.c.Name()+": "+err.Error())
		}
		e.mu.Unlock()
	}
	return errs
}
