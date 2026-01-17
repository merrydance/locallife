package scheduler

import (
	"context"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// RunnableScheduler defines a minimal scheduler interface.
type RunnableScheduler interface {
	Start() error
	Stop()
}

type schedulerEntry struct {
	name      string
	scheduler RunnableScheduler
}

// Manager manages multiple schedulers with unified lifecycle handling.
type Manager struct {
	entries []schedulerEntry
}

// NewManager creates a new scheduler manager.
func NewManager() *Manager {
	return &Manager{}
}

// Register adds a scheduler to the manager.
func (m *Manager) Register(name string, scheduler RunnableScheduler) {
	if scheduler == nil {
		log.Warn().Str("scheduler", name).Msg("skip registering nil scheduler")
		return
	}
	m.entries = append(m.entries, schedulerEntry{name: name, scheduler: scheduler})
}

// StartAll starts all registered schedulers and ties their shutdown to ctx.
func (m *Manager) StartAll(ctx context.Context, waitGroup *errgroup.Group) {
	for _, entry := range m.entries {
		if err := entry.scheduler.Start(); err != nil {
			log.Error().Err(err).Str("scheduler", entry.name).Msg("failed to start scheduler")
			continue
		}

		log.Info().Str("scheduler", entry.name).Msg("scheduler started")

		entry := entry
		waitGroup.Go(func() error {
			<-ctx.Done()
			log.Info().Str("scheduler", entry.name).Msg("graceful shutdown scheduler")
			entry.scheduler.Stop()
			return nil
		})
	}
}

























































