// Package scheduler runs the Schedule tick loop.
//
// One Scheduler instance ticks every TickInterval; on each tick it
// reads due Schedules (NextFireAt <= now) from the store and emits
// execution.started events for each. The handler that turns events
// into actual Executions lives in pkg/executor (M2 follow-on; for
// now the Scheduler publishes a *replay.Record shape that the
// Execution path picks up).
//
// The Scheduler lives in cmd/promptsheond; production wires it into
// the daemon at boot via WithScheduler().
package scheduler

import (
	"context"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/schedule"
)

// Scheduler holds the tick loop.
type Scheduler struct {
	schedules schedule.Repository
	publisher eventbus.Publisher
	tick      time.Duration
}

// New constructs a Scheduler. tick must be > 0; 5 seconds is the
// sensible default for a control plane that schedules executions
// down to one-minute cron resolution.
func New(s schedule.Repository, p eventbus.Publisher, tick time.Duration) *Scheduler {
	if tick <= 0 {
		tick = 5 * time.Second
	}
	return &Scheduler{schedules: s, publisher: p, tick: tick}
}

// Start launches the tick goroutine and returns. Cancelling ctx
// stops the loop cleanly. Start blocks until ctx is cancelled; it is
// meant to run as a daemon-managed goroutine.
func (s *Scheduler) Start(ctx context.Context) error {
	t := time.NewTicker(s.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-t.C:
			s.tickOnce(ctx, now)
		}
	}
}

// tickOnce reads due Schedules and publishes an event per fired
// schedule. Errors are logged by the caller; we never abort the
// loop on a single bad schedule.
func (s *Scheduler) tickOnce(ctx context.Context, now time.Time) {
	due, err := s.schedules.ListDueSchedules(ctx, now, 64)
	if err != nil {
		return
	}
	for i := range due {
		sc := due[i]
		if !sc.Enabled {
			continue
		}
		fired := sc.MarkFired(now)
		if err := s.schedules.UpdateSchedule(ctx, &fired); err != nil {
			continue
		}
		_ = s.publisher.Publish(capability.Event{
			Type:          capability.EventType("schedule.fired"),
			AggregateID:   sc.ID,
			AggregateType: "schedule",
			Data: map[string]any{
				"workspace_id": sc.WorkspaceID,
				"release_id":   sc.ReleaseID,
				"fired_at":     now,
			},
		})
	}
}
