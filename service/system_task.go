package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	systemTaskLockTTL   = 60 * time.Second
	logCleanupBatchSize = 100
)

// SystemTaskHandler executes a claimed task of a specific type. Run owns the
// task lifecycle from claim to terminal state: it MUST call
// model.FinishSystemTask (succeeded/failed) before returning and MUST honor
// ctx cancellation, which the runner triggers if the per-type lock is lost.
type SystemTaskHandler interface {
	Type() string
	Run(ctx context.Context, task *model.SystemTask, runnerID string)
}

// ScheduledSystemTaskHandler is a SystemTaskHandler that the scheduler also
// creates periodically when enabled and the configured interval has elapsed
// since the last run.
type ScheduledSystemTaskHandler interface {
	SystemTaskHandler
	Enabled(ctx context.Context) bool
	Interval(ctx context.Context) time.Duration
	NewPayload() any
}

var (
	systemTaskHandlersMu sync.RWMutex
	systemTaskHandlers   = map[string]SystemTaskHandler{}
)

// RegisterSystemTaskHandler registers a handler keyed by its Type(). It must be
// called before RunTenantJobs (or any time, since the scheduler snapshots
// the registry every pass). Re-registering a type replaces the previous handler.
func RegisterSystemTaskHandler(h SystemTaskHandler) {
	if h == nil {
		return
	}
	systemTaskHandlersMu.Lock()
	defer systemTaskHandlersMu.Unlock()
	systemTaskHandlers[h.Type()] = h
}

func registeredSystemTaskHandlers() []SystemTaskHandler {
	systemTaskHandlersMu.RLock()
	defer systemTaskHandlersMu.RUnlock()
	handlers := make([]SystemTaskHandler, 0, len(systemTaskHandlers))
	for _, h := range systemTaskHandlers {
		handlers = append(handlers, h)
	}
	return handlers
}

// logCleanupHandler wraps the existing on-demand log cleanup task as a
// registered (non-scheduled) handler. It is created via StartLogCleanupTask.
type logCleanupHandler struct{}

func (logCleanupHandler) Type() string { return model.SystemTaskTypeLogCleanup }

func (logCleanupHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	runLogCleanupTask(ctx, task, runnerID)
}

func init() {
	RegisterSystemTaskHandler(logCleanupHandler{})
}

type LogCleanupPayload struct {
	TargetTimestamp int64 `json:"target_timestamp"`
	BatchSize       int   `json:"batch_size"`
}

type LogCleanupState struct {
	Total     int64 `json:"total"`
	Processed int64 `json:"processed"`
	Progress  int   `json:"progress"`
	Remaining int64 `json:"remaining"`
}

type LogCleanupResult struct {
	DeletedCount int64 `json:"deleted_count"`
}

func StartLogCleanupTask(tenantCtx context.Context, targetTimestamp int64) (*model.SystemTask, error) {
	if targetTimestamp <= 0 {
		return nil, errors.New("target timestamp is required")
	}

	activeTask, err := model.GetActiveSystemTask(tenantCtx, model.SystemTaskTypeLogCleanup)
	if err != nil {
		return nil, err
	}
	if activeTask != nil {
		return activeTask, nil
	}

	payload := LogCleanupPayload{
		TargetTimestamp: targetTimestamp,
		BatchSize:       logCleanupBatchSize,
	}
	state := LogCleanupState{}
	task, err := model.CreateSystemTask(tenantCtx, model.SystemTaskTypeLogCleanup, payload, state)
	if err != nil {
		activeTask, activeErr := model.GetActiveSystemTask(tenantCtx, model.SystemTaskTypeLogCleanup)
		if activeErr == nil && activeTask != nil {
			return activeTask, nil
		}
		return nil, err
	}
	return task, nil
}

// EnqueueSystemTask creates an on-demand task of the given type. The returned
// bool is true only when a new pending row was created; false means an active
// task of the same type already exists and was returned.
func EnqueueSystemTask(tenantCtx context.Context, taskType string, payload any) (*model.SystemTask, bool, error) {
	activeTask, err := model.GetActiveSystemTask(tenantCtx, taskType)
	if err != nil {
		return nil, false, err
	}
	if activeTask != nil {
		return activeTask, false, nil
	}

	task, err := model.CreateSystemTask(tenantCtx, taskType, payload, nil)
	if err != nil {
		activeTask, activeErr := model.GetActiveSystemTask(tenantCtx, taskType)
		if activeErr == nil && activeTask != nil {
			return activeTask, false, nil
		}
		return nil, false, err
	}
	return task, true, nil
}

// runSystemTaskClaimPass tries to claim one pending task per registered type
// and dispatches each claimed task in its own goroutine so a long-running
// handler (e.g. channel test) never blocks another type (e.g. log cleanup).
func runSystemTaskClaimPass(tenantCtx context.Context, runnerID string) {
	handlers := registeredSystemTaskHandlers()
	taskTypes := make([]string, 0, len(handlers))
	for _, handler := range handlers {
		taskTypes = append(taskTypes, handler.Type())
	}
	pendingTasks, err := model.FindEarliestPendingSystemTasks(tenantCtx, taskTypes)
	if err != nil {
		logger.LogWarn(tenantCtx, fmt.Sprintf("system task runner query failed: %v", err))
		return
	}
	for _, handler := range handlers {
		task := pendingTasks[handler.Type()]
		if task == nil {
			continue
		}
		claimedTask, claimed, err := model.ClaimSystemTask(tenantCtx, task.ID, handler.Type(), runnerID, systemTaskLockUntil())
		if err != nil {
			logger.LogWarn(tenantCtx, fmt.Sprintf("system task claim failed: %v", err))
			continue
		}
		if !claimed {
			continue
		}
		dispatchHandler := handler
		dispatchTask := claimedTask
		gopool.Go(func() {
			runWithLeaseHeartbeat(tenantCtx, dispatchTask, runnerID, func(ctx context.Context) {
				dispatchHandler.Run(ctx, dispatchTask, runnerID)
			})
		})
	}
}

// runSystemTaskScheduler creates a new task row for each enabled scheduled
// handler whose interval has elapsed since its last run and that has no active
// row. The task active_key unique index deduplicates concurrent creation while
// the per-type lock guarantees only one runner executes the task.
func runSystemTaskScheduler(tenantCtx context.Context) {
	now := common.GetTimestamp()
	handlers := registeredSystemTaskHandlers()
	scheduledHandlers := make([]ScheduledSystemTaskHandler, 0, len(handlers))
	taskTypes := make([]string, 0, len(handlers))
	for _, handler := range handlers {
		scheduled, ok := handler.(ScheduledSystemTaskHandler)
		if !ok || !scheduled.Enabled(tenantCtx) {
			continue
		}
		scheduledHandlers = append(scheduledHandlers, scheduled)
		taskTypes = append(taskTypes, scheduled.Type())
	}
	latestTasks, err := model.GetLatestSystemTasks(tenantCtx, taskTypes)
	if err != nil {
		logger.LogWarn(tenantCtx, fmt.Sprintf("system task scheduler query failed: %v", err))
		return
	}
	for _, scheduled := range scheduledHandlers {
		latest := latestTasks[scheduled.Type()]
		if latest != nil {
			if latest.Status == model.SystemTaskStatusPending || latest.Status == model.SystemTaskStatusRunning {
				continue // an active row already exists
			}
			if now-latest.UpdatedAt < int64(scheduled.Interval(tenantCtx).Seconds()) {
				continue // not due yet
			}
		}
		if _, err := model.CreateSystemTask(tenantCtx, scheduled.Type(), scheduled.NewPayload(), nil); err != nil {
			activeTask, activeErr := model.GetActiveSystemTask(tenantCtx, scheduled.Type())
			if activeErr == nil && activeTask != nil {
				continue
			}
			if activeErr != nil {
				logger.LogWarn(tenantCtx, fmt.Sprintf("system task scheduler active lookup failed: type=%s err=%v", scheduled.Type(), activeErr))
			}
			logger.LogWarn(tenantCtx, fmt.Sprintf("system task scheduler create failed: type=%s err=%v", scheduled.Type(), err))
			continue
		}
	}
}

// runWithLeaseHeartbeat renews the per-type lock on a background ticker while
// fn runs. The TTL is a crash-detection window, not a task time limit: an
// arbitrarily long handler stays alive as long as the heartbeat succeeds.
func runWithLeaseHeartbeat(tenantCtx context.Context, task *model.SystemTask, runnerID string, fn func(ctx context.Context)) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(tenantCtx))
	defer cancel()

	interval := systemTaskLockTTL / 3
	if interval <= 0 {
		interval = systemTaskLockTTL
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := model.RenewSystemTaskLock(tenantCtx, task.TaskID, runnerID, systemTaskLockUntil()); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	fn(ctx)
	close(done)
}

func runLogCleanupTask(ctx context.Context, task *model.SystemTask, runnerID string) {
	payload := LogCleanupPayload{}
	if err := task.DecodePayload(&payload); err != nil {
		failSystemTask(ctx, task, runnerID, err)
		return
	}
	if payload.TargetTimestamp <= 0 {
		failSystemTask(ctx, task, runnerID, errors.New("target timestamp is required"))
		return
	}
	if payload.BatchSize <= 0 {
		payload.BatchSize = logCleanupBatchSize
	}

	state := LogCleanupState{}
	if err := task.DecodeState(&state); err != nil {
		failSystemTask(ctx, task, runnerID, err)
		return
	}

	for {
		remaining, err := model.CountOldLog(ctx, payload.TargetTimestamp)
		if err != nil {
			failSystemTask(ctx, task, runnerID, err)
			return
		}
		syncLogCleanupStateFromRemaining(&state, remaining)
		if err := model.UpdateSystemTaskState(ctx, task.TaskID, runnerID, state); err != nil {
			logSystemTaskLockError(ctx, task, err)
			return
		}
		if state.Remaining == 0 {
			break
		}

		// Track whether this pass deleted anything so a fresh recount that still
		// reports remaining rows resumes immediately instead of waiting for the
		// lock to expire. If a whole pass deletes nothing while rows remain, the
		// rows cannot be removed and we fail instead of busy-looping.
		progressed := false
		for state.Remaining > 0 {
			rowsAffected, err := model.DeleteOldLogBatch(ctx, payload.TargetTimestamp, payload.BatchSize)
			if err != nil {
				failSystemTask(ctx, task, runnerID, err)
				return
			}
			if rowsAffected == 0 {
				break
			}
			progressed = true

			state.Processed += rowsAffected
			if state.Total < state.Processed {
				state.Total = state.Processed
			}
			if state.Remaining > rowsAffected {
				state.Remaining -= rowsAffected
			} else {
				state.Remaining = 0
			}
			state.Progress = logCleanupProgress(state.Processed, state.Total)

			if err := model.UpdateSystemTaskState(ctx, task.TaskID, runnerID, state); err != nil {
				logSystemTaskLockError(ctx, task, err)
				return
			}
		}

		if !progressed {
			failSystemTask(ctx, task, runnerID, errors.New("no log rows were deleted"))
			return
		}
	}

	state.Remaining = 0
	state.Progress = 100
	if state.Total < state.Processed {
		state.Total = state.Processed
	}
	if err := model.UpdateSystemTaskState(ctx, task.TaskID, runnerID, state); err != nil {
		logSystemTaskLockError(ctx, task, err)
		return
	}

	result := LogCleanupResult{DeletedCount: state.Processed}
	if err := model.FinishSystemTask(ctx, task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}

func syncLogCleanupStateFromRemaining(state *LogCleanupState, remaining int64) {
	if state.Total <= 0 {
		state.Total = remaining
		state.Processed = 0
	} else {
		processedFromRemaining := state.Total - remaining
		if processedFromRemaining > state.Processed {
			state.Processed = processedFromRemaining
		}
	}
	if state.Processed < 0 {
		state.Processed = 0
	}
	state.Remaining = remaining
	state.Progress = logCleanupProgress(state.Processed, state.Total)
}

func logCleanupProgress(processed int64, total int64) int {
	if total <= 0 {
		return 100
	}
	if processed <= 0 {
		return 0
	}
	if processed >= total {
		return 100
	}
	return int(processed * 100 / total)
}

func systemTaskLockUntil() int64 {
	return common.GetTimestamp() + int64(systemTaskLockTTL.Seconds())
}

// SystemTaskProgress is the state shape used by handlers that report percentage
// progress (channel test, model update). The frontend reads the progress field
// (0-100) to render a per-task progress indicator.
type SystemTaskProgress struct {
	Total     int `json:"total"`
	Processed int `json:"processed"`
	Progress  int `json:"progress"`
}

// NewSystemTaskProgressReporter returns a throttled progress callback bound to a
// running task. Handlers call it with (processed, total) as they iterate work;
// it persists a {processed,total,progress} state at most once every ~2s, always
// emitting the first update and the final 100%.
// Lock-loss errors are ignored: the lease heartbeat cancels the handler ctx on
// loss, so progress writes are best-effort and never abort the run themselves.
// The returned func is single-goroutine only (call it from the handler loop).
func NewSystemTaskProgressReporter(tenantCtx context.Context, task *model.SystemTask, runnerID string) func(processed, total int) {
	const minWriteInterval = 2 * time.Second
	var (
		lastWriteAt  time.Time
		lastProgress = -1
	)
	return func(processed, total int) {
		progress := 100
		if total > 0 {
			progress = processed * 100 / total
		}
		if progress < 0 {
			progress = 0
		} else if progress > 100 {
			progress = 100
		}

		if progress < 100 {
			if progress == lastProgress {
				return
			}
			if !lastWriteAt.IsZero() && time.Since(lastWriteAt) < minWriteInterval {
				return
			}
		}
		lastProgress = progress
		lastWriteAt = time.Now()

		state := SystemTaskProgress{Total: total, Processed: processed, Progress: progress}
		_ = model.UpdateSystemTaskState(tenantCtx, task.TaskID, runnerID, state)
	}
}

func failSystemTask(tenantCtx context.Context, task *model.SystemTask, runnerID string, err error) {
	logger.LogWarn(tenantCtx, fmt.Sprintf("system task %s failed: %v", task.TaskID, err))
	if finishErr := model.FinishSystemTask(tenantCtx, task.TaskID, runnerID, model.SystemTaskStatusFailed, nil, err.Error()); finishErr != nil {
		logger.LogWarn(tenantCtx, fmt.Sprintf("system task %s failed to save failure state: %v", task.TaskID, finishErr))
	}
}

func logSystemTaskLockError(ctx context.Context, task *model.SystemTask, err error) {
	if errors.Is(err, model.ErrSystemTaskLockLost) {
		logger.LogWarn(ctx, fmt.Sprintf("system task %s lock lost", task.TaskID))
		return
	}
	logger.LogWarn(ctx, fmt.Sprintf("system task %s update failed: %v", task.TaskID, err))
}
