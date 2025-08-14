package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"mysql_tablespace_recycling/cluster"
	"mysql_tablespace_recycling/recycler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitor(t *testing.T) {
	t.Run("Create monitor with default config", func(t *testing.T) {
		monitor := NewMonitor(nil)
		assert.NotNil(t, monitor)

		defaultMonitor := monitor.(*DefaultMonitor)
		assert.NotNil(t, defaultMonitor.tasks)
		assert.NotNil(t, defaultMonitor.taskHistory)
		assert.NotNil(t, defaultMonitor.hooks)
		assert.NotNil(t, defaultMonitor.stats)
		assert.Equal(t, "/tmp/mysql-recycler-status.json", defaultMonitor.statusFile)
		assert.Equal(t, 1000, defaultMonitor.maxHistoryRecords)
	})

	t.Run("Create monitor with custom config", func(t *testing.T) {
		config := &MonitorConfig{
			StatusFile:        "/tmp/custom-status.json",
			MaxHistoryRecords: 500,
			PersistInterval:   10 * time.Minute,
			EnableAutoSave:    false,
		}

		monitor := NewMonitor(config)
		assert.NotNil(t, monitor)

		defaultMonitor := monitor.(*DefaultMonitor)
		assert.Equal(t, "/tmp/custom-status.json", defaultMonitor.statusFile)
		assert.Equal(t, 500, defaultMonitor.maxHistoryRecords)
	})
}

func TestMonitorTaskLifecycle(t *testing.T) {
	tempFile := "/tmp/test-monitor-status.json"
	defer os.Remove(tempFile)

	config := &MonitorConfig{
		StatusFile:        tempFile,
		MaxHistoryRecords: 100,
		EnableAutoSave:    false,
	}

	monitor := NewMonitor(config)
	defer monitor.Shutdown()

	t.Run("Start task", func(t *testing.T) {
		task := createTestTask("test-task-1")
		err := monitor.StartTask(task)
		assert.NoError(t, err)

		status, err := monitor.GetTaskStatus("test-task-1")
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "test-task-1", status.TaskID)
		assert.NotNil(t, status.StartedAt)

		stats := monitor.GetStats()
		assert.Equal(t, 1, stats.ActiveTasks)
	})

	t.Run("Update task progress", func(t *testing.T) {
		progress := &recycler.TaskProgress{
			TaskID:          "test-task-1",
			TotalTables:     10,
			CompletedTables: 5,
			CurrentTable:    "test_db.test_table",
			ProcessedSize:   100 * 1024 * 1024, // 100MB
			ReclaimedSpace:  50 * 1024 * 1024,  // 50MB
			StartTime:       time.Now(),
			LastUpdate:      time.Now(),
		}

		err := monitor.UpdateProgress("test-task-1", progress)
		assert.NoError(t, err)

		status, err := monitor.GetTaskStatus("test-task-1")
		assert.NoError(t, err)
		assert.Equal(t, 5, status.Progress.CompletedTables)
		assert.Equal(t, "test_db.test_table", status.Progress.CurrentTable)
	})

	t.Run("Update task status", func(t *testing.T) {
		err := monitor.UpdateTaskStatus("test-task-1", recycler.StatusRunning)
		assert.NoError(t, err)

		status, err := monitor.GetTaskStatus("test-task-1")
		assert.NoError(t, err)
		assert.Equal(t, recycler.StatusRunning, status.Status)
	})

	t.Run("Complete task successfully", func(t *testing.T) {
		result := &recycler.RecycleResult{
			TaskID:           "test-task-1",
			ProcessedTables:  10,
			SuccessfulTables: 9,
			FailedTables:     1,
			ReclaimedSpace:   200 * 1024 * 1024, // 200MB
			TotalTime:        5 * time.Minute,
		}

		err := monitor.CompleteTask("test-task-1", result, nil)
		assert.NoError(t, err)

		status, err := monitor.GetTaskStatus("test-task-1")
		assert.NoError(t, err)
		assert.Equal(t, recycler.StatusCompleted, status.Status)
		assert.NotNil(t, status.CompletedAt)
		assert.Equal(t, int64(200*1024*1024), status.Result.ReclaimedSpace)

		stats := monitor.GetStats()
		assert.Equal(t, 0, stats.ActiveTasks)
		assert.Equal(t, 1, stats.CompletedTasks)
		assert.Equal(t, int64(200*1024*1024), stats.TotalReclaimedSpace)
		assert.Equal(t, 10, stats.TotalProcessedTables)
	})

	t.Run("Complete task with error", func(t *testing.T) {
		task2 := createTestTask("test-task-2")
		monitor.StartTask(task2)

		err := monitor.CompleteTask("test-task-2", nil, assert.AnError)
		assert.NoError(t, err)

		status, err := monitor.GetTaskStatus("test-task-2")
		assert.NoError(t, err)
		assert.Equal(t, recycler.StatusFailed, status.Status)
		assert.Contains(t, status.ErrorMessage, assert.AnError.Error())

		stats := monitor.GetStats()
		assert.Equal(t, 1, stats.FailedTasks)
	})

	t.Run("Task not found error", func(t *testing.T) {
		err := monitor.UpdateProgress("non-existent", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		err = monitor.UpdateTaskStatus("non-existent", recycler.StatusRunning)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		_, err = monitor.GetTaskStatus("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestMonitorTaskHistory(t *testing.T) {
	tempFile := "/tmp/test-monitor-history.json"
	defer os.Remove(tempFile)

	config := &MonitorConfig{
		StatusFile:        tempFile,
		MaxHistoryRecords: 3, // 小的历史记录限制用于测试
		EnableAutoSave:    false,
	}

	monitor := NewMonitor(config)
	defer monitor.Shutdown()

	// 创建并完成多个任务
	for i := 1; i <= 5; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		task := createTestTask(taskID)
		monitor.StartTask(task)
		
		result := &recycler.RecycleResult{
			TaskID:          taskID,
			ProcessedTables: i,
		}
		monitor.CompleteTask(taskID, result, nil)
	}

	t.Run("Get task history with limit", func(t *testing.T) {
		history, err := monitor.GetTaskHistory(2)
		assert.NoError(t, err)
		assert.Len(t, history, 2)
		
		// 应该返回最新的两个任务
		assert.Equal(t, "task-5", history[0].TaskID)
		assert.Equal(t, "task-4", history[1].TaskID)
	})

	t.Run("Get all task history", func(t *testing.T) {
		history, err := monitor.GetTaskHistory(0)
		assert.NoError(t, err)
		assert.Len(t, history, 3) // 最多保留3个历史记录
		
		// 应该包含最后3个任务
		assert.Equal(t, "task-5", history[0].TaskID)
		assert.Equal(t, "task-4", history[1].TaskID)
		assert.Equal(t, "task-3", history[2].TaskID)
	})
}

func TestMonitorActiveTasks(t *testing.T) {
	tempFile := "/tmp/test-monitor-active.json"
	defer os.Remove(tempFile)

	config := &MonitorConfig{
		StatusFile:     tempFile,
		EnableAutoSave: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Shutdown()

	t.Run("List active tasks", func(t *testing.T) {
		// 启动几个任务
		task1 := createTestTask("active-task-1")
		task2 := createTestTask("active-task-2")
		
		monitor.StartTask(task1)
		monitor.StartTask(task2)

		activeTasks, err := monitor.ListActiveTasks()
		assert.NoError(t, err)
		assert.Len(t, activeTasks, 2)

		taskIDs := make([]string, 0, 2)
		for _, task := range activeTasks {
			taskIDs = append(taskIDs, task.TaskID)
		}
		assert.Contains(t, taskIDs, "active-task-1")
		assert.Contains(t, taskIDs, "active-task-2")

		// 完成一个任务
		monitor.CompleteTask("active-task-1", &recycler.RecycleResult{}, nil)

		activeTasks, err = monitor.ListActiveTasks()
		assert.NoError(t, err)
		assert.Len(t, activeTasks, 1)
		assert.Equal(t, "active-task-2", activeTasks[0].TaskID)
	})

	t.Run("No active tasks", func(t *testing.T) {
		// 完成所有任务
		monitor.CompleteTask("active-task-2", &recycler.RecycleResult{}, nil)

		activeTasks, err := monitor.ListActiveTasks()
		assert.NoError(t, err)
		assert.Len(t, activeTasks, 0)
	})
}

func TestMonitorPersistence(t *testing.T) {
	tempFile := "/tmp/test-monitor-persist.json"
	defer os.Remove(tempFile)

	config := &MonitorConfig{
		StatusFile:     tempFile,
		EnableAutoSave: false,
	}

	t.Run("Save and load status", func(t *testing.T) {
		// 创建监控器并添加任务
		monitor1 := NewMonitor(config)
		
		task := createTestTask("persist-task")
		monitor1.StartTask(task)
		monitor1.UpdateTaskStatus("persist-task", recycler.StatusRunning)

		// 持久化状态
		err := monitor1.PersistStatus()
		assert.NoError(t, err)
		
		// 验证文件存在
		assert.FileExists(t, tempFile)
		
		// 完成任务并关闭监控器
		result := &recycler.RecycleResult{
			TaskID:         "persist-task",
			ReclaimedSpace: 100 * 1024 * 1024,
		}
		monitor1.CompleteTask("persist-task", result, nil)
		monitor1.Shutdown()

		// 创建新监控器并加载状态
		monitor2 := NewMonitor(config)
		defer monitor2.Shutdown()

		// 验证历史任务被加载
		history, err := monitor2.GetTaskHistory(0)
		assert.NoError(t, err)
		assert.Len(t, history, 1)
		assert.Equal(t, "persist-task", history[0].TaskID)

		// 验证统计信息被加载
		stats := monitor2.GetStats()
		assert.Equal(t, 1, stats.CompletedTasks)
		assert.Equal(t, int64(100*1024*1024), stats.TotalReclaimedSpace)
	})

	t.Run("Load status with missing file", func(t *testing.T) {
		missingFile := "/tmp/non-existent-status.json"
		configMissing := &MonitorConfig{
			StatusFile:     missingFile,
			EnableAutoSave: false,
		}

		monitor := NewMonitor(configMissing)
		defer monitor.Shutdown()

		// 应该成功创建，只是没有加载到历史数据
		stats := monitor.GetStats()
		assert.Equal(t, 0, stats.CompletedTasks)
	})
}

func TestMonitorHooks(t *testing.T) {
	tempFile := "/tmp/test-monitor-hooks.json"
	defer os.Remove(tempFile)

	config := &MonitorConfig{
		StatusFile:     tempFile,
		EnableAutoSave: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Shutdown()

	// 创建测试钩子
	hook := &TestHook{}
	err := monitor.RegisterHook(hook)
	assert.NoError(t, err)

	t.Run("Hook called on task start", func(t *testing.T) {
		task := createTestTask("hook-task")
		err := monitor.StartTask(task)
		assert.NoError(t, err)
		
		assert.True(t, hook.onStartCalled)
		assert.Equal(t, "hook-task", hook.lastTaskID)
	})

	t.Run("Hook called on progress update", func(t *testing.T) {
		progress := &recycler.TaskProgress{
			TaskID:          "hook-task",
			CompletedTables: 5,
		}
		
		err := monitor.UpdateProgress("hook-task", progress)
		assert.NoError(t, err)
		
		assert.True(t, hook.onProgressCalled)
	})

	t.Run("Hook called on task completion", func(t *testing.T) {
		result := &recycler.RecycleResult{
			TaskID: "hook-task",
		}
		
		err := monitor.CompleteTask("hook-task", result, nil)
		assert.NoError(t, err)
		
		assert.True(t, hook.onCompleteCalled)
	})

	t.Run("Hook called on task failure", func(t *testing.T) {
		failTask := createTestTask("fail-task")
		monitor.StartTask(failTask)
		
		hook.Reset() // 重置钩子状态
		
		err := monitor.CompleteTask("fail-task", nil, assert.AnError)
		assert.NoError(t, err)
		
		assert.True(t, hook.onFailedCalled)
	})
}

func TestMonitorStats(t *testing.T) {
	tempFile := "/tmp/test-monitor-stats.json"
	defer os.Remove(tempFile)

	config := &MonitorConfig{
		StatusFile:     tempFile,
		EnableAutoSave: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Shutdown()

	t.Run("Initial stats", func(t *testing.T) {
		stats := monitor.GetStats()
		assert.Equal(t, 0, stats.ActiveTasks)
		assert.Equal(t, 0, stats.CompletedTasks)
		assert.Equal(t, 0, stats.FailedTasks)
		assert.Equal(t, int64(0), stats.TotalReclaimedSpace)
		assert.Equal(t, 0, stats.TotalProcessedTables)
		assert.NotZero(t, stats.UptimeStart)
	})

	t.Run("Stats after tasks", func(t *testing.T) {
		// 启动2个任务
		task1 := createTestTask("stats-task-1")
		task2 := createTestTask("stats-task-2")
		
		monitor.StartTask(task1)
		monitor.StartTask(task2)

		stats := monitor.GetStats()
		assert.Equal(t, 2, stats.ActiveTasks)

		// 成功完成1个任务
		result1 := &recycler.RecycleResult{
			TaskID:          "stats-task-1",
			ProcessedTables: 5,
			ReclaimedSpace:  100 * 1024 * 1024,
		}
		monitor.CompleteTask("stats-task-1", result1, nil)

		// 失败完成1个任务
		monitor.CompleteTask("stats-task-2", nil, assert.AnError)

		stats = monitor.GetStats()
		assert.Equal(t, 0, stats.ActiveTasks)
		assert.Equal(t, 1, stats.CompletedTasks)
		assert.Equal(t, 1, stats.FailedTasks)
		assert.Equal(t, int64(100*1024*1024), stats.TotalReclaimedSpace)
		assert.Equal(t, 5, stats.TotalProcessedTables)
	})

	t.Run("Average task duration calculation", func(t *testing.T) {
		// 创建有明确时间的任务
		startTime := time.Now().Add(-10 * time.Minute)
		endTime := startTime.Add(5 * time.Minute)

		// 直接操作monitor内部结构来设置准确的时间
		defaultMonitor := monitor.(*DefaultMonitor)
		defaultMonitor.mu.Lock()
		
		taskStatus := &TaskStatus{
			TaskID:      "duration-task",
			Status:      recycler.StatusCompleted,
			StartedAt:   &startTime,
			CompletedAt: &endTime,
		}
		defaultMonitor.addToHistory(taskStatus)
		defaultMonitor.stats.CompletedTasks++
		
		defaultMonitor.mu.Unlock()

		stats := monitor.GetStats()
		assert.Greater(t, stats.AverageTaskDuration, time.Duration(0))
	})
}

func TestNotificationHook(t *testing.T) {
	t.Run("Create notification hook", func(t *testing.T) {
		hook := NewNotificationHook("http://example.com/webhook")
		assert.NotNil(t, hook)
		assert.True(t, hook.enabled)

		emptyHook := NewNotificationHook("")
		assert.False(t, emptyHook.enabled)
	})

	t.Run("Notification hook methods", func(t *testing.T) {
		hook := NewNotificationHook("") // 禁用实际发送

		taskStatus := &TaskStatus{
			TaskID: "test-task",
			Target: &recycler.RecycleTarget{
				Node: &cluster.NodeConfig{Host: "localhost", Port: 3306},
			},
			Progress: &recycler.TaskProgress{
				CompletedTables: 10,
				TotalTables:     20,
			},
			Result: &recycler.RecycleResult{
				ReclaimedSpace: 100 * 1024 * 1024,
			},
		}

		// 测试各种钩子方法不会崩溃
		err := hook.OnTaskStart(taskStatus)
		assert.NoError(t, err)

		err = hook.OnTaskProgress(taskStatus)
		assert.NoError(t, err)

		err = hook.OnTaskComplete(taskStatus)
		assert.NoError(t, err)

		err = hook.OnTaskFailed(taskStatus, assert.AnError)
		assert.NoError(t, err)
	})
}

func TestMonitorBackgroundPersistence(t *testing.T) {
	tempFile := "/tmp/test-monitor-background.json"
	defer os.Remove(tempFile)

	config := &MonitorConfig{
		StatusFile:      tempFile,
		PersistInterval: 100 * time.Millisecond,
		EnableAutoSave:  true,
	}

	monitor := NewMonitor(config)
	defer monitor.Shutdown()

	// 添加任务
	task := createTestTask("background-task")
	monitor.StartTask(task)

	// 等待后台持久化
	time.Sleep(200 * time.Millisecond)

	// 验证文件被创建
	assert.FileExists(t, tempFile)

	// 验证内容
	data, err := os.ReadFile(tempFile)
	require.NoError(t, err)

	var persistedData struct {
		Tasks map[string]*TaskStatus `json:"tasks"`
	}
	err = json.Unmarshal(data, &persistedData)
	require.NoError(t, err)

	assert.Contains(t, persistedData.Tasks, "background-task")
}

func TestFormatBytes(t *testing.T) {
	testCases := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1536, "1.5 KB"},                // 1.5 * 1024
		{2048 * 1024, "2.0 MB"},         // 2MB
		{1536 * 1024 * 1024, "1.50 GB"}, // 1.5GB
	}

	for _, tc := range testCases {
		result := formatBytes(tc.bytes)
		assert.Equal(t, tc.expected, result)
	}
}

// 测试辅助函数和结构

func createTestTask(taskID string) *recycler.RecycleTask {
	return &recycler.RecycleTask{
		ID: taskID,
		Target: &recycler.RecycleTarget{
			Node: &cluster.NodeConfig{
				Host:     "localhost",
				Port:     3306,
				Username: "test",
				Password: "test",
			},
			Mode:        1,
			Concurrency: 1,
		},
		Tables: []*recycler.TableFragmentation{
			{
				Schema:    "test_db",
				TableName: "test_table",
				Engine:    "InnoDB",
			},
		},
		Priority:  1,
		CreatedAt: time.Now(),
		Status:    recycler.StatusPending,
	}
}

// TestHook 测试用的钩子实现
type TestHook struct {
	onStartCalled    bool
	onProgressCalled bool
	onCompleteCalled bool
	onFailedCalled   bool
	lastTaskID       string
}

func (h *TestHook) OnTaskStart(taskStatus *TaskStatus) error {
	h.onStartCalled = true
	h.lastTaskID = taskStatus.TaskID
	return nil
}

func (h *TestHook) OnTaskProgress(taskStatus *TaskStatus) error {
	h.onProgressCalled = true
	h.lastTaskID = taskStatus.TaskID
	return nil
}

func (h *TestHook) OnTaskComplete(taskStatus *TaskStatus) error {
	h.onCompleteCalled = true
	h.lastTaskID = taskStatus.TaskID
	return nil
}

func (h *TestHook) OnTaskFailed(taskStatus *TaskStatus, err error) error {
	h.onFailedCalled = true
	h.lastTaskID = taskStatus.TaskID
	return nil
}

func (h *TestHook) Reset() {
	h.onStartCalled = false
	h.onProgressCalled = false
	h.onCompleteCalled = false
	h.onFailedCalled = false
	h.lastTaskID = ""
}