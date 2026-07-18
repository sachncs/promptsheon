package scheduler_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/schedule"
	"github.com/sachncs/promptsheon/internal/scheduler"
)

// fakeScheduleRepo is a hand-rolled in-memory implementation of
// schedule.Repository used only by scheduler tests.
type fakeScheduleRepo struct {
	mu     sync.Mutex
	all    []*schedule.Schedule
	dueErr error
	updErr error
}

func (f *fakeScheduleRepo) CreateSchedule(_ context.Context, s *schedule.Schedule) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.all = append(f.all, s)
	return nil
}

func (f *fakeScheduleRepo) GetSchedule(_ context.Context, id string) (*schedule.Schedule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.all {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (f *fakeScheduleRepo) ListSchedulesForRelease(_ context.Context, releaseID string) ([]*schedule.Schedule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []*schedule.Schedule{}
	for _, s := range f.all {
		if s.ReleaseID == releaseID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeScheduleRepo) ListDueSchedules(_ context.Context, now time.Time, _ int) ([]*schedule.Schedule, error) {
	if f.dueErr != nil {
		return nil, f.dueErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []*schedule.Schedule{}
	for _, s := range f.all {
		if !s.NextFireAt.After(now) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeScheduleRepo) UpdateSchedule(_ context.Context, s *schedule.Schedule) error {
	if f.updErr != nil {
		return f.updErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, cur := range f.all {
		if cur.ID == s.ID {
			f.all[i] = s
			return nil
		}
	}
	return nil
}

func (f *fakeScheduleRepo) DeleteSchedule(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.all {
		if s.ID == id {
			f.all = append(f.all[:i], f.all[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestNewDefaultsTick(t *testing.T) {
	t.Parallel()
	repo := &fakeScheduleRepo{}
	pub := eventbus.NewMemory()
	defer pub.Close()

	if s := scheduler.New(repo, pub, 0); s == nil {
		t.Fatal("New(0) returned nil")
	}
	if s := scheduler.New(repo, pub, -1*time.Second); s == nil {
		t.Fatal("New(negative tick) returned nil")
	}
}

func TestStartStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	repo := &fakeScheduleRepo{}
	pub := eventbus.NewMemory()
	defer pub.Close()

	s := scheduler.New(repo, pub, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()
	cancel()

	select {
	case err := <-done:
		if err == nil || err != context.Canceled {
			t.Errorf("Start returned %v on cancel, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return within 2s after cancel")
	}
}

func TestTickOncePublishesForDueSchedules(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	due := &schedule.Schedule{
		ID:          "s1",
		WorkspaceID: "w1",
		ReleaseID:   "r1",
		Kind:        schedule.KindCron,
		Cron:        "*/5 * * * *",
		NextFireAt:  now.Add(-time.Minute),
		Enabled:     true,
		CreatedAt:   now,
		CreatedBy:   "u1",
	}
	notDue := &schedule.Schedule{
		ID:          "s2",
		WorkspaceID: "w1",
		ReleaseID:   "r1",
		Kind:        schedule.KindCron,
		Cron:        "0 0 * * *",
		NextFireAt:  now.Add(time.Hour),
		Enabled:     true,
		CreatedAt:   now,
		CreatedBy:   "u1",
	}
	disabled := &schedule.Schedule{
		ID:          "s3",
		WorkspaceID: "w1",
		ReleaseID:   "r1",
		Kind:        schedule.KindCron,
		Cron:        "*/5 * * * *",
		NextFireAt:  now.Add(-time.Minute),
		Enabled:     false,
		CreatedAt:   now,
		CreatedBy:   "u1",
	}

	repo := &fakeScheduleRepo{all: []*schedule.Schedule{due, notDue, disabled}}

	got := make(chan capability.Event, 4)
	pub := eventbus.NewMemory()
	defer pub.Close()
	sub, err := pub.Subscribe(func(e capability.Event) {
		got <- e
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Cancel()

	s := scheduler.New(repo, pub, time.Hour)
	s.TickOnce(context.Background(), now)

	select {
	case ev := <-got:
		if ev.Type != "schedule.fired" {
			t.Errorf("event type = %q, want schedule.fired", ev.Type)
		}
		if ev.AggregateID != "s1" {
			t.Errorf("aggregate id = %q, want s1", ev.AggregateID)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive event for due schedule")
	}

	select {
	case extra := <-got:
		t.Errorf("unexpected event for non-firing schedule: %+v", extra)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTickOnceListErrorIsSwallowed(t *testing.T) {
	t.Parallel()
	repo := &fakeScheduleRepo{dueErr: context.DeadlineExceeded}
	pub := eventbus.NewMemory()
	defer pub.Close()
	s := scheduler.New(repo, pub, time.Hour)
	s.TickOnce(context.Background(), time.Now())
}

func TestTickOnceUpdateErrorDoesNotPublish(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	due := &schedule.Schedule{
		ID:          "s1",
		WorkspaceID: "w1",
		ReleaseID:   "r1",
		Kind:        schedule.KindCron,
		Cron:        "*/5 * * * *",
		NextFireAt:  now.Add(-time.Minute),
		Enabled:     true,
		CreatedAt:   now,
		CreatedBy:   "u1",
	}
	repo := &fakeScheduleRepo{all: []*schedule.Schedule{due}, updErr: context.DeadlineExceeded}

	got := make(chan capability.Event, 4)
	pub := eventbus.NewMemory()
	defer pub.Close()
	sub, err := pub.Subscribe(func(e capability.Event) { got <- e })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Cancel()

	s := scheduler.New(repo, pub, time.Hour)
	s.TickOnce(context.Background(), now)

	select {
	case extra := <-got:
		t.Errorf("unexpected event when UpdateSchedule fails: %+v", extra)
	case <-time.After(50 * time.Millisecond):
	}
}
