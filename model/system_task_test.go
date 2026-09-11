package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSystemTaskPayload struct {
	TargetTimestamp int64 `json:"target_timestamp"`
	BatchSize       int   `json:"batch_size"`
}

type testSystemTaskState struct {
	Total     int64 `json:"total"`
	Processed int64 `json:"processed"`
	Progress  int   `json:"progress"`
	Remaining int64 `json:"remaining"`
}

func createLegacyPendingSystemTask(t *testing.T, taskType string) *SystemTask {
	t.Helper()
	taskID, err := GenerateSystemTaskID()
	require.NoError(t, err)
	task := &SystemTask{
		TaskID: taskID,
		Type:   taskType,
		Status: SystemTaskStatusPending,
	}
	require.NoError(t, DB.Create(task).Error)
	return task
}

func TestSystemTaskCreateAndActiveLifecycle(t *testing.T) {
	truncateTables(t)

	payload := testSystemTaskPayload{TargetTimestamp: 1000, BatchSize: 100}
	state := testSystemTaskState{}
	task, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, payload, state)
	require.NoError(t, err)
	require.NotNil(t, task.ActiveKey)
	assert.Equal(t, SystemTaskTypeLogCleanup, *task.ActiveKey)

	var decodedPayload testSystemTaskPayload
	require.NoError(t, task.DecodePayload(&decodedPayload))
	assert.Equal(t, payload, decodedPayload)

	activeTask, err := GetActiveSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup)
	require.NoError(t, err)
	require.NotNil(t, activeTask)
	assert.Equal(t, task.TaskID, activeTask.TaskID)

	runnerID := "runner-a"
	claimedTask, claimed, err := ClaimSystemTask(testtenant.Context(), task.ID, SystemTaskTypeLogCleanup, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	err = FinishSystemTask(testtenant.Context(), claimedTask.TaskID, runnerID, SystemTaskStatusSucceeded, map[string]int64{"deleted_count": 0}, "")
	require.NoError(t, err)

	finishedTask, err := GetSystemTaskByTaskID(testtenant.Context(), task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finishedTask)
	assert.Nil(t, finishedTask.ActiveKey)

	activeTask, err = GetActiveSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup)
	require.NoError(t, err)
	require.Nil(t, activeTask)

	_, err = CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, payload, state)
	require.NoError(t, err)
}

func TestSystemTaskActiveKeyPreventsDuplicateActiveRun(t *testing.T) {
	truncateTables(t)

	payload := testSystemTaskPayload{TargetTimestamp: 1000, BatchSize: 100}
	task, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, payload, testSystemTaskState{})
	require.NoError(t, err)
	_, err = CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, payload, testSystemTaskState{})
	require.Error(t, err)

	activeTask, err := GetActiveSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup)
	require.NoError(t, err)
	require.NotNil(t, activeTask)
	assert.Equal(t, task.TaskID, activeTask.TaskID)
}

func TestSystemTaskLockPreventsConcurrentClaim(t *testing.T) {
	truncateTables(t)

	payload := testSystemTaskPayload{TargetTimestamp: 1000, BatchSize: 100}
	task, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, payload, testSystemTaskState{})
	require.NoError(t, err)
	secondTask := createLegacyPendingSystemTask(t, SystemTaskTypeLogCleanup)

	claimedTask, claimed, err := ClaimSystemTask(testtenant.Context(), task.ID, SystemTaskTypeLogCleanup, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	_, claimed, err = ClaimSystemTask(testtenant.Context(), secondTask.ID, SystemTaskTypeLogCleanup, "runner-b", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.False(t, claimed)

	assert.Equal(t, "runner-a", claimedTask.LockedBy)

	reloadedSecond, err := GetSystemTaskByTaskID(testtenant.Context(), secondTask.TaskID)
	require.NoError(t, err)
	require.NotNil(t, reloadedSecond)
	assert.Equal(t, SystemTaskStatusPending, reloadedSecond.Status)
}

func TestExpiredSystemTaskLockFailsOldRunAndClaimsLegacyPendingRun(t *testing.T) {
	truncateTables(t)

	first, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, nil, nil)
	require.NoError(t, err)
	_, claimed, err := ClaimSystemTask(testtenant.Context(), first.ID, SystemTaskTypeLogCleanup, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, DB.Model(&SystemTaskLock{}).
		Where("task_id = ?", first.TaskID).
		Update("locked_until", common.GetTimestamp()-1).Error)

	second := createLegacyPendingSystemTask(t, SystemTaskTypeLogCleanup)
	claimedTask, claimed, err := ClaimSystemTask(testtenant.Context(), second.ID, SystemTaskTypeLogCleanup, "runner-b", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	assert.Equal(t, second.TaskID, claimedTask.TaskID)
	assert.Equal(t, "runner-b", claimedTask.LockedBy)

	reloadedFirst, err := GetSystemTaskByTaskID(testtenant.Context(), first.TaskID)
	require.NoError(t, err)
	require.NotNil(t, reloadedFirst)
	assert.Equal(t, SystemTaskStatusFailed, reloadedFirst.Status)
	assert.Equal(t, "task lease expired", reloadedFirst.Error)
	assert.Nil(t, reloadedFirst.ActiveKey)
}

func TestExpireStaleSystemTaskLockFailsOldRunAndAllowsNewRun(t *testing.T) {
	truncateTables(t)

	first, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, nil, nil)
	require.NoError(t, err)
	_, claimed, err := ClaimSystemTask(testtenant.Context(), first.ID, SystemTaskTypeLogCleanup, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, DB.Model(&SystemTaskLock{}).
		Where("task_id = ?", first.TaskID).
		Update("locked_until", common.GetTimestamp()-1).Error)

	require.NoError(t, ExpireStaleSystemTaskLocks(testtenant.Context(), common.GetTimestamp()))

	reloadedFirst, err := GetSystemTaskByTaskID(testtenant.Context(), first.TaskID)
	require.NoError(t, err)
	require.NotNil(t, reloadedFirst)
	assert.Equal(t, SystemTaskStatusFailed, reloadedFirst.Status)
	assert.Equal(t, "task lease expired", reloadedFirst.Error)
	assert.Nil(t, reloadedFirst.ActiveKey)

	var lockCount int64
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("task_id = ?", first.TaskID).Count(&lockCount).Error)
	assert.Equal(t, int64(0), lockCount)

	second, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, nil, nil)
	require.NoError(t, err)
	require.NotEqual(t, first.TaskID, second.TaskID)
}

func TestFindEarliestPendingSystemTasks(t *testing.T) {
	truncateTables(t)

	empty, err := FindEarliestPendingSystemTasks(testtenant.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	firstA, err := CreateSystemTask(testtenant.Context(), "type_a", nil, nil)
	require.NoError(t, err)
	ignoredB, err := CreateSystemTask(testtenant.Context(), "type_b", nil, nil)
	require.NoError(t, err)
	_, claimed, err := ClaimSystemTask(testtenant.Context(), ignoredB.ID, "type_b", "runner-b", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, FinishSystemTask(testtenant.Context(), ignoredB.TaskID, "runner-b", SystemTaskStatusFailed, nil, "failed"))
	firstB, err := CreateSystemTask(testtenant.Context(), "type_b", nil, nil)
	require.NoError(t, err)
	ignoredC, err := CreateSystemTask(testtenant.Context(), "type_c", nil, nil)
	require.NoError(t, err)
	_, claimed, err = ClaimSystemTask(testtenant.Context(), ignoredC.ID, "type_c", "runner-c", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, FinishSystemTask(testtenant.Context(), ignoredC.TaskID, "runner-c", SystemTaskStatusFailed, nil, "failed"))

	tasks, err := FindEarliestPendingSystemTasks(testtenant.Context(), []string{"type_a", "type_b", "type_c", "missing"})
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	assert.Equal(t, firstA.TaskID, tasks["type_a"].TaskID)
	assert.Equal(t, firstB.TaskID, tasks["type_b"].TaskID)
	assert.Nil(t, tasks["type_c"])
	assert.Nil(t, tasks["missing"])
}

func TestGetLatestSystemTask(t *testing.T) {
	truncateTables(t)

	latest, err := GetLatestSystemTask(testtenant.Context(), SystemTaskTypeChannelTest)
	require.NoError(t, err)
	require.Nil(t, latest)

	first, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeChannelTest, nil, nil)
	require.NoError(t, err)

	runnerID := "runner-a"
	_, claimed, err := ClaimSystemTask(testtenant.Context(), first.ID, SystemTaskTypeChannelTest, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, FinishSystemTask(testtenant.Context(), first.TaskID, runnerID, SystemTaskStatusSucceeded, nil, ""))

	second, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeChannelTest, nil, nil)
	require.NoError(t, err)

	latest, err = GetLatestSystemTask(testtenant.Context(), SystemTaskTypeChannelTest)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, second.TaskID, latest.TaskID)
}

func TestGetLatestSystemTasks(t *testing.T) {
	truncateTables(t)

	empty, err := GetLatestSystemTasks(testtenant.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	firstA, err := CreateSystemTask(testtenant.Context(), "type_a", nil, nil)
	require.NoError(t, err)
	firstB, err := CreateSystemTask(testtenant.Context(), "type_b", nil, nil)
	require.NoError(t, err)
	_, claimed, err := ClaimSystemTask(testtenant.Context(), firstA.ID, "type_a", "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, FinishSystemTask(testtenant.Context(), firstA.TaskID, "runner-a", SystemTaskStatusSucceeded, nil, ""))
	secondA, err := CreateSystemTask(testtenant.Context(), "type_a", nil, nil)
	require.NoError(t, err)

	tasks, err := GetLatestSystemTasks(testtenant.Context(), []string{"type_a", "type_b", "missing"})
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	assert.NotEqual(t, firstA.TaskID, tasks["type_a"].TaskID)
	assert.Equal(t, secondA.TaskID, tasks["type_a"].TaskID)
	assert.Equal(t, firstB.TaskID, tasks["type_b"].TaskID)
	assert.Nil(t, tasks["missing"])
}

func TestRenewSystemTaskLock(t *testing.T) {
	truncateTables(t)

	task, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, nil, nil)
	require.NoError(t, err)

	runnerID := "runner-a"
	_, claimed, err := ClaimSystemTask(testtenant.Context(), task.ID, SystemTaskTypeLogCleanup, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	newLockUntil := common.GetTimestamp() + 600
	require.NoError(t, RenewSystemTaskLock(testtenant.Context(), task.TaskID, runnerID, newLockUntil))

	var lock SystemTaskLock
	require.NoError(t, DB.Where("task_id = ?", task.TaskID).First(&lock).Error)
	assert.Equal(t, newLockUntil, lock.LockedUntil)

	// A different runner cannot renew a lease it does not hold.
	assert.ErrorIs(t, RenewSystemTaskLock(testtenant.Context(), task.TaskID, "runner-b", common.GetTimestamp()+600), ErrSystemTaskLockLost)

	// After the task finishes it is no longer running, so renew fails.
	require.NoError(t, FinishSystemTask(testtenant.Context(), task.TaskID, runnerID, SystemTaskStatusSucceeded, nil, ""))
	assert.ErrorIs(t, RenewSystemTaskLock(testtenant.Context(), task.TaskID, runnerID, common.GetTimestamp()+600), ErrSystemTaskLockLost)
}

func TestFinishSystemTaskRetainsExecutor(t *testing.T) {
	truncateTables(t)

	task, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, nil, nil)
	require.NoError(t, err)

	runnerID := "node-1-abc123"
	_, claimed, err := ClaimSystemTask(testtenant.Context(), task.ID, SystemTaskTypeLogCleanup, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, FinishSystemTask(testtenant.Context(), task.TaskID, runnerID, SystemTaskStatusSucceeded, nil, ""))

	reloaded, err := GetSystemTaskByTaskID(testtenant.Context(), task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, SystemTaskStatusSucceeded, reloaded.Status)
	assert.Equal(t, runnerID, reloaded.LockedBy, "executor-of-record must be retained for history")

	var lockCount int64
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("task_id = ?", task.TaskID).Count(&lockCount).Error)
	assert.Equal(t, int64(0), lockCount)
}

func TestSystemTaskUpdatesRequireCurrentLock(t *testing.T) {
	truncateTables(t)

	task, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, nil, nil)
	require.NoError(t, err)

	runnerID := "runner-a"
	_, claimed, err := ClaimSystemTask(testtenant.Context(), task.ID, SystemTaskTypeLogCleanup, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, DB.Model(&SystemTaskLock{}).
		Where("task_id = ?", task.TaskID).
		Updates(map[string]any{"locked_by": "runner-b"}).Error)

	assert.ErrorIs(t, UpdateSystemTaskState(testtenant.Context(), task.TaskID, runnerID, testSystemTaskState{Progress: 10}), ErrSystemTaskLockLost)
	assert.ErrorIs(t, FinishSystemTask(testtenant.Context(), task.TaskID, runnerID, SystemTaskStatusSucceeded, nil, ""), ErrSystemTaskLockLost)
}

func TestSystemTaskUpdatesRequireUnexpiredLock(t *testing.T) {
	truncateTables(t)

	task, err := CreateSystemTask(testtenant.Context(), SystemTaskTypeLogCleanup, nil, nil)
	require.NoError(t, err)

	runnerID := "runner-a"
	_, claimed, err := ClaimSystemTask(testtenant.Context(), task.ID, SystemTaskTypeLogCleanup, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, DB.Model(&SystemTaskLock{}).
		Where("task_id = ?", task.TaskID).
		Update("locked_until", common.GetTimestamp()-1).Error)

	assert.ErrorIs(t, UpdateSystemTaskState(testtenant.Context(), task.TaskID, runnerID, testSystemTaskState{Progress: 10}), ErrSystemTaskLockLost)
	assert.ErrorIs(t, FinishSystemTask(testtenant.Context(), task.TaskID, runnerID, SystemTaskStatusSucceeded, nil, ""), ErrSystemTaskLockLost)

	reloaded, err := GetSystemTaskByTaskID(testtenant.Context(), task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, SystemTaskStatusRunning, reloaded.Status)
	assert.Empty(t, reloaded.State)
}

func TestUpdateSystemTaskStateIdenticalPayloadDoesNotLoseLock(t *testing.T) {
	// SQLite reports matched rows for unchanged UPDATEs, so this case passed
	// even before the fix. The MySQL regression is covered by
	// TestUpdateSystemTaskStateIdenticalPayloadDoesNotLoseLockConfiguredDatabases.
	truncateTables(t)
	runUpdateSystemTaskStateIdenticalPayloadKeepsLock(t, SystemTaskTypeLogCleanup)
}

func runUpdateSystemTaskStateIdenticalPayloadKeepsLock(t *testing.T, taskType string) {
	t.Helper()
	// Two persists in the same second so MySQL's unchanged-row UPDATE returns 0.

	task, err := CreateSystemTask(testtenant.Context(), taskType, nil, nil)
	require.NoError(t, err)

	runnerID := "runner-a"
	_, claimed, err := ClaimSystemTask(testtenant.Context(), task.ID, taskType, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	state := testSystemTaskState{Total: 10, Processed: 10, Progress: 100, Remaining: 0}
	require.NoError(t, UpdateSystemTaskState(testtenant.Context(), task.TaskID, runnerID, state))
	require.NoError(t, UpdateSystemTaskState(testtenant.Context(), task.TaskID, runnerID, state), "identical state persist must not be treated as lock loss")

	require.NoError(t, FinishSystemTask(testtenant.Context(), task.TaskID, runnerID, SystemTaskStatusSucceeded, map[string]int64{"deleted_count": 10}, ""))
	finished, err := GetSystemTaskByTaskID(testtenant.Context(), task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finished)
	assert.Equal(t, SystemTaskStatusSucceeded, finished.Status)
}
