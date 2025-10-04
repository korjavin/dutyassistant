package handlers

import (
	"github.com/korjavin/dutyassistant/internal/scheduler"
	"github.com/korjavin/dutyassistant/internal/store"
)

// Handlers holds dependencies for command handlers, such as the database store
// and the business logic scheduler. This approach centralizes dependencies.
type Handlers struct {
	Store     store.Store
	Scheduler scheduler.SchedulerInterface
}

// New creates a new Handlers instance with the provided dependencies.
func New(s store.Store, sch scheduler.SchedulerInterface) *Handlers {
	return &Handlers{
		Store:     s,
		Scheduler: sch,
	}
}