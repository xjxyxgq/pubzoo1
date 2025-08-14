package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"mysql_tablespace_recycling/recycler"
)

// Monitor 监控器接口
type Monitor interface {
	// StartTask 开始监控任务
	StartTask(task *recycler.RecycleTask) error

	// UpdateProgress 更新任务进度
	UpdateProgress(taskID string, progress *recycler.TaskProgress) error

	// UpdateTaskStatus 更新任务状态
	UpdateTaskStatus(taskID string, status recycler.TaskStatus) error

	// GetTaskStatus 获取任务状态
	GetTaskStatus(taskID string) (*TaskStatus, error)

	// ListActiveTasks 列出活跃任务
	ListActiveTasks() ([]*TaskStatus, error)

	// GetTaskHistory 获取任务历史
	GetTaskHistory(limit int) ([]*TaskStatus, error)

	// PersistStatus 持久化状态到文件
	PersistStatus() error

	// LoadStatus 从文件加载状态
	LoadStatus() error

	// RegisterHook 注册监控钩子
	RegisterHook(hook MonitorHook) error

	// StartBackgroundPersistence 启动后台持久化
	StartBackgroundPersistence(ctx context.Context, interval time.Duration) error

	// GetStats 获取监控统计信息
	GetStats() *MonitorStats

	// CompleteTask 完成任务
	CompleteTask(taskID string, result *recycler.RecycleResult, err error) error

	// Shutdown 优雅关闭监控器
	Shutdown() error
}

// TaskStatus 任务状态（扩展版本）
type TaskStatus struct {
	TaskID       string                  `json:"task_id"`
	Target       *recycler.RecycleTarget `json:"target"`
	Status       recycler.TaskStatus     `json:"status"`
	Progress     *recycler.TaskProgress  `json:"progress,omitempty"`
	Result       *recycler.RecycleResult `json:"result,omitempty"`
	CreatedAt    time.Time               `json:"created_at"`
	StartedAt    *time.Time              `json:"started_at,omitempty"`
	CompletedAt  *time.Time              `json:"completed_at,omitempty"`
	LastUpdate   time.Time               `json:"last_update"`
	ErrorMessage string                  `json:"error_message,omitempty"`
	Warnings     []string                `json:"warnings,omitempty"`
	Metadata     map[string]interface{}  `json:"metadata,omitempty"`
}

// MonitorStats 监控统计信息
type MonitorStats struct {
	ActiveTasks          int           `json:"active_tasks"`
	CompletedTasks       int           `json:"completed_tasks"`
	FailedTasks          int           `json:"failed_tasks"`
	TotalReclaimedSpace  int64         `json:"total_reclaimed_space"`
	TotalProcessedTables int           `json:"total_processed_tables"`
	AverageTaskDuration  time.Duration `json:"average_task_duration"`
	LastPersistTime      time.Time     `json:"last_persist_time"`
	UptimeStart          time.Time     `json:"uptime_start"`
}

// MonitorHook 监控钩子接口
type MonitorHook interface {
	// OnTaskStart 任务开始时调用
	OnTaskStart(taskStatus *TaskStatus) error

	// OnTaskProgress 任务进度更新时调用
	OnTaskProgress(taskStatus *TaskStatus) error

	// OnTaskComplete 任务完成时调用
	OnTaskComplete(taskStatus *TaskStatus) error

	// OnTaskFailed 任务失败时调用
	OnTaskFailed(taskStatus *TaskStatus, err error) error
}

// DefaultMonitor 默认监控器实现
type DefaultMonitor struct {
	mu                sync.RWMutex
	tasks             map[string]*TaskStatus
	taskHistory       []*TaskStatus
	hooks             []MonitorHook
	statusFile        string
	maxHistoryRecords int
	stats             *MonitorStats
	persistenceTicker *time.Ticker
	ctx               context.Context
	cancel            context.CancelFunc
}

// MonitorConfig 监控器配置
type MonitorConfig struct {
	StatusFile        string        `json:"status_file" yaml:"status_file"`
	MaxHistoryRecords int           `json:"max_history_records" yaml:"max_history_records"`
	PersistInterval   time.Duration `json:"persist_interval" yaml:"persist_interval"`
	EnableAutoSave    bool          `json:"enable_auto_save" yaml:"enable_auto_save"`
}

// NewMonitor 创建监控器
func NewMonitor(config *MonitorConfig) Monitor {
	if config == nil {
		config = &MonitorConfig{
			StatusFile:        "/tmp/mysql-recycler-status.json",
			MaxHistoryRecords: 1000,
			PersistInterval:   5 * time.Minute,
			EnableAutoSave:    true,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &DefaultMonitor{
		tasks:             make(map[string]*TaskStatus),
		taskHistory:       make([]*TaskStatus, 0),
		hooks:             make([]MonitorHook, 0),
		statusFile:        config.StatusFile,
		maxHistoryRecords: config.MaxHistoryRecords,
		ctx:               ctx,
		cancel:            cancel,
		stats: &MonitorStats{
			UptimeStart: time.Now(),
		},
	}

	// 自动加载持久化状态
	if err := monitor.LoadStatus(); err != nil {
		// 记录错误但不中断初始化，因为可能是第一次运行
		fmt.Printf("Warning: failed to load monitor status: %v\n", err)
	}

	// 启动后台持久化
	if config.EnableAutoSave {
		go monitor.StartBackgroundPersistence(ctx, config.PersistInterval)
	}

	return monitor
}

// StartTask 开始监控任务
func (m *DefaultMonitor) StartTask(task *recycler.RecycleTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	taskStatus := &TaskStatus{
		TaskID:     task.ID,
		Target:     task.Target,
		Status:     task.Status,
		Progress:   task.Progress,
		CreatedAt:  task.CreatedAt,
		StartedAt:  &now,
		LastUpdate: now,
		Metadata:   make(map[string]interface{}),
	}

	m.tasks[task.ID] = taskStatus
	m.stats.ActiveTasks++

	// 调用钩子
	for _, hook := range m.hooks {
		if err := hook.OnTaskStart(taskStatus); err != nil {
			taskStatus.Warnings = append(taskStatus.Warnings, fmt.Sprintf("Hook error: %v", err))
		}
	}

	return nil
}

// UpdateProgress 更新任务进度
func (m *DefaultMonitor) UpdateProgress(taskID string, progress *recycler.TaskProgress) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskStatus, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	taskStatus.Progress = progress
	taskStatus.LastUpdate = time.Now()

	// 调用钩子
	for _, hook := range m.hooks {
		if err := hook.OnTaskProgress(taskStatus); err != nil {
			taskStatus.Warnings = append(taskStatus.Warnings, fmt.Sprintf("Hook error: %v", err))
		}
	}

	return nil
}

// UpdateTaskStatus 更新任务状态
func (m *DefaultMonitor) UpdateTaskStatus(taskID string, status recycler.TaskStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskStatus, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	taskStatus.Status = status
	taskStatus.LastUpdate = time.Now()

	// 立即持久化运行状态，确保跨进程可见
	if status == recycler.StatusRunning {
		go func() {
			if err := m.PersistStatus(); err != nil {
				fmt.Printf("Warning: failed to persist running status: %v\n", err)
			}
		}()
	}

	return nil
}

// CompleteTask 完成任务
func (m *DefaultMonitor) CompleteTask(taskID string, result *recycler.RecycleResult, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskStatus, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	taskStatus.CompletedAt = &now
	taskStatus.LastUpdate = now
	taskStatus.Result = result

	if err != nil {
		taskStatus.Status = recycler.StatusFailed
		taskStatus.ErrorMessage = err.Error()
		m.stats.FailedTasks++

		// 调用失败钩子
		for _, hook := range m.hooks {
			if hookErr := hook.OnTaskFailed(taskStatus, err); hookErr != nil {
				taskStatus.Warnings = append(taskStatus.Warnings, fmt.Sprintf("Hook error: %v", hookErr))
			}
		}
	} else {
		taskStatus.Status = recycler.StatusCompleted
		m.stats.CompletedTasks++

		// 更新统计信息
		if result != nil {
			m.stats.TotalReclaimedSpace += result.ReclaimedSpace
			m.stats.TotalProcessedTables += result.ProcessedTables
		}

		// 调用完成钩子
		for _, hook := range m.hooks {
			if hookErr := hook.OnTaskComplete(taskStatus); hookErr != nil {
				taskStatus.Warnings = append(taskStatus.Warnings, fmt.Sprintf("Hook error: %v", hookErr))
			}
		}
	}

	// 移动到历史记录
	m.addToHistory(taskStatus)
	delete(m.tasks, taskID)
	m.stats.ActiveTasks--

	return nil
}

// GetTaskStatus 获取任务状态
func (m *DefaultMonitor) GetTaskStatus(taskID string) (*TaskStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if taskStatus, exists := m.tasks[taskID]; exists {
		// 返回副本以避免并发修改
		statusCopy := *taskStatus
		return &statusCopy, nil
	}

	// 在历史记录中查找
	for _, taskStatus := range m.taskHistory {
		if taskStatus.TaskID == taskID {
			statusCopy := *taskStatus
			return &statusCopy, nil
		}
	}

	return nil, fmt.Errorf("task %s not found", taskID)
}

// ListActiveTasks 列出活跃任务
func (m *DefaultMonitor) ListActiveTasks() ([]*TaskStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 首先检查内存中的活跃任务
	tasks := make([]*TaskStatus, 0, len(m.tasks))
	for _, task := range m.tasks {
		taskCopy := *task
		tasks = append(tasks, &taskCopy)
	}
	
	// 如果内存中没有活跃任务，检查历史任务中是否有正在运行的任务
	// 这样可以支持跨进程查看正在运行的任务
	if len(tasks) == 0 {
		for _, task := range m.taskHistory {
			// 只显示真正正在运行或等待中的任务，不包括已完成、失败或取消的任务
			if task.Status == recycler.StatusRunning || task.Status == recycler.StatusPending {
				// 检查任务是否还在合理的时间窗口内（避免显示过期的运行中任务）
				if time.Since(task.LastUpdate) < 10*time.Minute {
					taskCopy := *task
					tasks = append(tasks, &taskCopy)
				}
			}
		}
	}

	return tasks, nil
}

// GetTaskHistory 获取任务历史
func (m *DefaultMonitor) GetTaskHistory(limit int) ([]*TaskStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]*TaskStatus, 0)
	count := 0

	// 从最新的开始返回
	for i := len(m.taskHistory) - 1; i >= 0 && (limit == 0 || count < limit); i-- {
		taskCopy := *m.taskHistory[i]
		history = append(history, &taskCopy)
		count++
	}

	return history, nil
}

// GetStats 获取监控统计信息
func (m *DefaultMonitor) GetStats() *MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.ActiveTasks = len(m.tasks)

	// 计算平均任务时长
	if m.stats.CompletedTasks > 0 {
		var totalDuration time.Duration
		count := 0

		for _, task := range m.taskHistory {
			if task.CompletedAt != nil && task.StartedAt != nil {
				totalDuration += task.CompletedAt.Sub(*task.StartedAt)
				count++
			}
		}

		if count > 0 {
			stats.AverageTaskDuration = totalDuration / time.Duration(count)
		}
	}

	return &stats
}

// RegisterHook 注册监控钩子
func (m *DefaultMonitor) RegisterHook(hook MonitorHook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hooks = append(m.hooks, hook)
	return nil
}

// PersistStatus 持久化状态到文件
func (m *DefaultMonitor) PersistStatus() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := struct {
		Tasks       map[string]*TaskStatus `json:"tasks"`
		TaskHistory []*TaskStatus          `json:"task_history"`
		Stats       *MonitorStats          `json:"stats"`
		SavedAt     time.Time              `json:"saved_at"`
	}{
		Tasks:       m.tasks,
		TaskHistory: m.taskHistory,
		Stats:       m.stats,
		SavedAt:     time.Now(),
	}

	file, err := os.Create(m.statusFile)
	if err != nil {
		return fmt.Errorf("failed to create status file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode status data: %w", err)
	}

	m.stats.LastPersistTime = time.Now()
	return nil
}

// LoadStatus 从文件加载状态
func (m *DefaultMonitor) LoadStatus() error {
	file, err := os.Open(m.statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，跳过加载
		}
		return fmt.Errorf("failed to open status file: %w", err)
	}
	defer file.Close()

	var data struct {
		Tasks       map[string]*TaskStatus `json:"tasks"`
		TaskHistory []*TaskStatus          `json:"task_history"`
		Stats       *MonitorStats          `json:"stats"`
		SavedAt     time.Time              `json:"saved_at"`
	}

	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode status data: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 加载历史数据
	if data.TaskHistory != nil {
		m.taskHistory = data.TaskHistory
	}

	// 加载统计信息
	if data.Stats != nil {
		m.stats = data.Stats
		// 重置活跃任务计数，因为重启后活跃任务会丢失
		m.stats.ActiveTasks = 0
	}
	
	// 不加载活跃任务，因为重启后这些任务状态已经不可靠
	// 但可以通过 LoadActiveTasksForStatus() 方法专门为 status 命令加载

	return nil
}

// LoadActiveTasksForStatus 专门为status命令加载活跃任务（支持跨进程监控）
func (m *DefaultMonitor) LoadActiveTasksForStatus() error {
	file, err := os.Open(m.statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，跳过加载
		}
		return fmt.Errorf("failed to open status file: %w", err)
	}
	defer file.Close()

	var data struct {
		Tasks       map[string]*TaskStatus `json:"tasks"`
		TaskHistory []*TaskStatus          `json:"task_history"`
		Stats       *MonitorStats          `json:"stats"`
		SavedAt     time.Time              `json:"saved_at"`
	}

	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode status data: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 关键修复：清空现有的内存任务，以持久化文件为准
	m.tasks = make(map[string]*TaskStatus)

	// 只从文件中加载真正活跃的任务
	if data.Tasks != nil {
		for taskID, task := range data.Tasks {
			// 只加载状态为运行中或等待中且最近更新的任务
			// 不加载已完成、失败或取消的任务
			if (task.Status == recycler.StatusRunning || task.Status == recycler.StatusPending) &&
				time.Since(task.LastUpdate) < 5*time.Minute {
				m.tasks[taskID] = task
			}
		}
	}
	
	// 同步统计信息：从文件中加载最新的统计数据
	if data.Stats != nil {
		// 保留uptime_start，但同步其他统计数据
		oldUptimeStart := m.stats.UptimeStart
		*m.stats = *data.Stats
		m.stats.UptimeStart = oldUptimeStart
	}
	
	// 确保活跃任务计数准确
	m.stats.ActiveTasks = len(m.tasks)

	return nil
}

// StartBackgroundPersistence 启动后台持久化
func (m *DefaultMonitor) StartBackgroundPersistence(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.PersistStatus(); err != nil {
				// 记录错误但不中断服务
				fmt.Printf("Failed to persist status: %v\n", err)
			}
		case <-ctx.Done():
			// 最后一次保存
			m.PersistStatus()
			return ctx.Err()
		}
	}
}

// addToHistory 添加到历史记录
func (m *DefaultMonitor) addToHistory(taskStatus *TaskStatus) {
	m.taskHistory = append(m.taskHistory, taskStatus)

	// 限制历史记录数量
	if len(m.taskHistory) > m.maxHistoryRecords {
		// 删除最旧的记录
		copy(m.taskHistory, m.taskHistory[1:])
		m.taskHistory = m.taskHistory[:len(m.taskHistory)-1]
	}
}

// Shutdown 优雅关闭监控器
func (m *DefaultMonitor) Shutdown() error {
	if m.cancel != nil {
		m.cancel()
	}

	// 最终持久化
	return m.PersistStatus()
}

// NotificationHook 通知钩子实现示例
type NotificationHook struct {
	webhookURL string
	enabled    bool
}

func NewNotificationHook(webhookURL string) *NotificationHook {
	return &NotificationHook{
		webhookURL: webhookURL,
		enabled:    webhookURL != "",
	}
}

func (h *NotificationHook) OnTaskStart(taskStatus *TaskStatus) error {
	if !h.enabled {
		return nil
	}

	message := fmt.Sprintf("🚀 Task %s started on %s",
		taskStatus.TaskID, taskStatus.Target.Node.GetHost())

	return h.sendNotification(message)
}

func (h *NotificationHook) OnTaskProgress(taskStatus *TaskStatus) error {
	if !h.enabled || taskStatus.Progress == nil {
		return nil
	}

	// 只在特定进度点发送通知（避免过于频繁）
	if taskStatus.Progress.CompletedTables%10 == 0 {
		message := fmt.Sprintf("⏳ Task %s progress: %d/%d tables completed",
			taskStatus.TaskID,
			taskStatus.Progress.CompletedTables,
			taskStatus.Progress.TotalTables)

		return h.sendNotification(message)
	}

	return nil
}

func (h *NotificationHook) OnTaskComplete(taskStatus *TaskStatus) error {
	if !h.enabled {
		return nil
	}

	var reclaimedSpace int64
	var processedTables int
	if taskStatus.Result != nil {
		reclaimedSpace = taskStatus.Result.ReclaimedSpace
		processedTables = taskStatus.Result.ProcessedTables
	}

	message := fmt.Sprintf("✅ Task %s completed: %d tables processed, %s reclaimed",
		taskStatus.TaskID, processedTables, formatBytes(reclaimedSpace))

	return h.sendNotification(message)
}

func (h *NotificationHook) OnTaskFailed(taskStatus *TaskStatus, err error) error {
	if !h.enabled {
		return nil
	}

	message := fmt.Sprintf("❌ Task %s failed: %v", taskStatus.TaskID, err)

	return h.sendNotification(message)
}

func (h *NotificationHook) sendNotification(message string) error {
	// 这里可以实现实际的通知发送逻辑
	// 例如发送到Slack、邮件、短信等
	fmt.Printf("Notification: %s\n", message)
	return nil
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	}
}
