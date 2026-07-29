package download

import (
	"context"
	"sync"
	"testing"
)

// resetSchedulerState clears the package-level scheduler state so each test is
// isolated. It does not touch the DB-backed helpers (those are exercised
// indirectly via the pure scheduling logic only).
func resetSchedulerState() {
	// Clear statusMap
	statusMap.Range(func(k, _ any) bool {
		statusMap.Delete(k)
		return true
	})
	// Clear taskParams
	taskParams.Range(func(k, _ any) bool {
		taskParams.Delete(k)
		return true
	})
	tasks = sync.Map{}
	maxConcurrent = DefaultMaxConcurrentDownload
	// Override the real "resume" with a fake that simply registers the task as
	// running, so the scheduler's running-slot accounting is exercised without
	// performing any real network downloads.
	resumeFunc = func(taskId int) error {
		tasks.Store(taskId, noopCancel())
		return nil
	}
}

func noopCancel() context.CancelFunc { return func() {} }

// TestActiveRunningCount verifies the running-slot counter tracks live tasks.
func TestActiveRunningCount(t *testing.T) {
	resetSchedulerState()
	if got := activeRunningCount(); got != 0 {
		t.Fatalf("expected 0 running, got %d", got)
	}
	tasks.Store(1, noopCancel())
	tasks.Store(2, noopCancel())
	if got := activeRunningCount(); got != 2 {
		t.Fatalf("expected 2 running, got %d", got)
	}
}

// TestHighestPriorityQueued picks the queued task with the greatest priority.
func TestHighestPriorityQueued(t *testing.T) {
	resetSchedulerState()
	statusMap.Store(1, &Progress{TaskID: 1, Status: Queued, Priority: 5})
	statusMap.Store(2, &Progress{TaskID: 2, Status: Queued, Priority: 9})
	statusMap.Store(3, &Progress{TaskID: 3, Status: Queued, Priority: 9}) // tie -> lower id wins
	statusMap.Store(4, &Progress{TaskID: 4, Status: Paused})              // not queued

	if id := highestPriorityQueued(); id != 2 {
		t.Fatalf("expected task 2 (priority 9, lower id on tie), got %d", id)
	}

	// No queued tasks -> 0.
	statusMap.Range(func(k, v any) bool {
		v.(*Progress).Status = Paused
		return true
	})
	if id := highestPriorityQueued(); id != 0 {
		t.Fatalf("expected 0 when nothing queued, got %d", id)
	}
}

// TestConcurrencyCapQueuesWhenFull confirms a new task is Queued (not started)
// when the running count already equals the cap.
func TestConcurrencyCapQueuesWhenFull(t *testing.T) {
	resetSchedulerState()
	maxConcurrent = 2

	// Two tasks already running.
	statusMap.Store(10, &Progress{TaskID: 10, Status: Downloading, MediaType: Mp4})
	statusMap.Store(11, &Progress{TaskID: 11, Status: Downloading, MediaType: Mp4})
	tasks.Store(10, noopCancel())
	tasks.Store(11, noopCancel())

	// A third task tries to start.
	statusMap.Store(12, &Progress{TaskID: 12, Status: Downloading, MediaType: Mp4})
	enqueueOrStart(12)

	v, _ := statusMap.Load(12)
	if v.(*Progress).Status != Queued {
		t.Fatalf("expected task 12 to be Queued at capacity, got %s", v.(*Progress).Status)
	}
}

// TestSchedulerPromotesByPriority verifies a freed slot promotes the highest
// priority queued task (and only up to the cap).
func TestSchedulerPromotesByPriority(t *testing.T) {
	resetSchedulerState()
	maxConcurrent = 1

	// One running task.
	statusMap.Store(1, &Progress{TaskID: 1, Status: Downloading, MediaType: Mp4})
	tasks.Store(1, noopCancel())

	// Two queued tasks with different priorities.
	statusMap.Store(2, &Progress{TaskID: 2, Status: Queued, Priority: 1, MediaType: Mp4})
	statusMap.Store(3, &Progress{TaskID: 3, Status: Queued, Priority: 7, MediaType: Mp4})

	// Free the slot.
	tasks.Delete(1)
	scheduleDownloads()

	v3, _ := statusMap.Load(3)
	v2, _ := statusMap.Load(2)
	if v3.(*Progress).Status != Downloading {
		t.Fatalf("expected highest-priority task 3 to start, got %s", v3.(*Progress).Status)
	}
	if v2.(*Progress).Status != Queued {
		t.Fatalf("expected lower-priority task 2 to remain Queued, got %s", v2.(*Progress).Status)
	}
	if activeRunningCount() != 1 {
		t.Fatalf("expected exactly 1 running after promotion, got %d", activeRunningCount())
	}
}

// TestReorderTasksAssignsPriorities checks that the requested order maps to
// descending priorities (front = highest).
func TestReorderTasksAssignsPriorities(t *testing.T) {
	resetSchedulerState()
	statusMap.Store(1, &Progress{TaskID: 1, Status: Paused, MediaType: Mp4})
	statusMap.Store(2, &Progress{TaskID: 2, Status: Paused, MediaType: Mp4})
	statusMap.Store(3, &Progress{TaskID: 3, Status: Paused, MediaType: Mp4})

	if err := ReorderTasks([]int{3, 1, 2}); err != nil {
		t.Fatalf("ReorderTasks returned error: %v", err)
	}
	v3, _ := statusMap.Load(3)
	v1, _ := statusMap.Load(1)
	v2, _ := statusMap.Load(2)
	if v3.(*Progress).Priority != 3 {
		t.Fatalf("expected task 3 (front) priority 3, got %d", v3.(*Progress).Priority)
	}
	if v1.(*Progress).Priority != 2 {
		t.Fatalf("expected task 1 (middle) priority 2, got %d", v1.(*Progress).Priority)
	}
	if v2.(*Progress).Priority != 1 {
		t.Fatalf("expected task 2 (back) priority 1, got %d", v2.(*Progress).Priority)
	}
}

// TestSetPriorityUpdatesAndReschedules verifies SetPriority changes the value
// and that a higher priority can be promoted when a slot is free.
func TestSetPriorityUpdatesAndReschedules(t *testing.T) {
	resetSchedulerState()
	maxConcurrent = 1

	statusMap.Store(1, &Progress{TaskID: 1, Status: Downloading, MediaType: Mp4})
	tasks.Store(1, noopCancel())
	statusMap.Store(2, &Progress{TaskID: 2, Status: Queued, Priority: 0, MediaType: Mp4})

	// Free the slot, then bump task 2's priority and reschedule.
	tasks.Delete(1)
	if err := SetPriority(2, 100); err != nil {
		t.Fatalf("SetPriority returned error: %v", err)
	}
	v2, _ := statusMap.Load(2)
	if v2.(*Progress).Status != Downloading {
		t.Fatalf("expected task 2 promoted after priority bump, got %s", v2.(*Progress).Status)
	}
	if v2.(*Progress).Priority != 100 {
		t.Fatalf("expected priority 100, got %d", v2.(*Progress).Priority)
	}
}

// TestGetMaxConcurrentBounds ensures the cap is never below 1.
func TestGetMaxConcurrentBounds(t *testing.T) {
	resetSchedulerState()
	maxConcurrent = 0
	if got := GetMaxConcurrent(); got != 1 {
		t.Fatalf("expected min cap 1, got %d", got)
	}
	maxConcurrent = 5
	if got := GetMaxConcurrent(); got != 5 {
		t.Fatalf("expected cap 5, got %d", got)
	}
}
