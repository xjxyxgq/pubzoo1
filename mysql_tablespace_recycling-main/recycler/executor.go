package recycler

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"mysql_tablespace_recycling/cluster"
)

// RecycleMode 回收模式
type RecycleMode int

const (
	ModeIndependent RecycleMode = 1 // 独立节点模式 (sql_log_bin=OFF)
	ModeReplication RecycleMode = 2 // 复制同步模式 (sql_log_bin=ON)
)

// RecycleTask 回收任务
type RecycleTask struct {
	ID        string                `json:"id"`
	Target    *RecycleTarget        `json:"target"`
	Tables    []*TableFragmentation `json:"tables"`
	Priority  int                   `json:"priority"`
	CreatedAt time.Time             `json:"created_at"`
	Status    TaskStatus            `json:"status"`
	Progress  *TaskProgress         `json:"progress,omitempty"`
	Result    *RecycleResult        `json:"result,omitempty"`
	Error     string                `json:"error,omitempty"`
}

// RecycleTarget 回收目标配置
type RecycleTarget struct {
	Node          *cluster.NodeConfig `json:"node"`
	Mode          RecycleMode         `json:"mode"`
	Databases     []string            `json:"databases,omitempty"`
	Tables        []string            `json:"tables,omitempty"`
	ExcludeDBs    []string            `json:"exclude_dbs,omitempty"`
	ExcludeTables []string            `json:"exclude_tables,omitempty"`
	Threshold     int64               `json:"threshold"`
	Concurrency   int                 `json:"concurrency"`
	DryRun        bool                `json:"dry_run"`
	SafetyChecks  bool                `json:"safety_checks"`
}

// TaskStatus 任务状态
type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
	StatusPaused
)

func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	case StatusPaused:
		return "paused"
	default:
		return "unknown"
	}
}

// TaskProgress 任务进度
type TaskProgress struct {
	TaskID             string        `json:"task_id"`
	TotalTables        int           `json:"total_tables"`
	CompletedTables    int           `json:"completed_tables"`
	CurrentTable       string        `json:"current_table"`
	CurrentTableSize   int64         `json:"current_table_size"`
	ProcessedSize      int64         `json:"processed_size"`
	ReclaimedSpace     int64         `json:"reclaimed_space"`
	EstimatedRemaining time.Duration `json:"estimated_remaining"`
	StartTime          time.Time     `json:"start_time"`
	LastUpdate         time.Time     `json:"last_update"`
	ErrorCount         int           `json:"error_count"`
}

// RecycleResult 回收结果
type RecycleResult struct {
	TaskID           string                `json:"task_id"`
	ProcessedTables  int                   `json:"processed_tables"`
	SuccessfulTables int                   `json:"successful_tables"`
	FailedTables     int                   `json:"failed_tables"`
	SkippedTables    int                   `json:"skipped_tables"`
	ReclaimedSpace   int64                 `json:"reclaimed_space"`
	TotalTime        time.Duration         `json:"total_time"`
	TableResults     []*TableRecycleResult `json:"table_results"`
	Errors           []string              `json:"errors,omitempty"`
	Warnings         []string              `json:"warnings,omitempty"`
}

// TableRecycleResult 单个表的回收结果
type TableRecycleResult struct {
	Schema         string        `json:"schema"`
	TableName      string        `json:"table_name"`
	Status         string        `json:"status"`
	BeforeSize     int64         `json:"before_size"`
	AfterSize      int64         `json:"after_size"`
	ReclaimedSpace int64         `json:"reclaimed_space"`
	Duration       time.Duration `json:"duration"`
	Error          string        `json:"error,omitempty"`
}

// TablespaceRecycler 表空间回收器接口
type TablespaceRecycler interface {
	// ExecuteRecycle 执行表空间回收任务
	ExecuteRecycle(ctx context.Context, task *RecycleTask) (*RecycleResult, error)

	// RecycleTable 回收单个表
	RecycleTable(ctx context.Context, target *RecycleTarget, table *TableFragmentation) (*TableRecycleResult, error)

	// EstimateRecycleTime 估算回收时间
	EstimateRecycleTime(target *RecycleTarget) (time.Duration, error)

	// ValidateTarget 验证回收目标
	ValidateTarget(ctx context.Context, target *RecycleTarget) error
}

// DefaultTablespaceRecycler 默认表空间回收器实现
type DefaultTablespaceRecycler struct {
	connectionFactory ConnectionFactory
	analyzer          FragmentationAnalyzer
	validator         MasterValidator
	mu                sync.RWMutex
	activeTasks       map[string]*RecycleTask
	progressCallbacks []ProgressCallback
}

// MasterValidator 主节点验证器接口
type MasterValidator interface {
	ValidateMasterSafety(ctx context.Context, node NodeConfigInterface) error
}

// ProgressCallback 进度回调函数类型
type ProgressCallback func(progress *TaskProgress)

// NewTablespaceRecycler 创建表空间回收器
func NewTablespaceRecycler(
	factory ConnectionFactory,
	analyzer FragmentationAnalyzer,
	validator MasterValidator,
) TablespaceRecycler {
	return &DefaultTablespaceRecycler{
		connectionFactory: factory,
		analyzer:          analyzer,
		validator:         validator,
		activeTasks:       make(map[string]*RecycleTask),
	}
}

// RegisterProgressCallback 注册进度回调
func (r *DefaultTablespaceRecycler) RegisterProgressCallback(callback ProgressCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progressCallbacks = append(r.progressCallbacks, callback)
}

// ExecuteRecycle 执行表空间回收任务
func (r *DefaultTablespaceRecycler) ExecuteRecycle(ctx context.Context, task *RecycleTask) (*RecycleResult, error) {
	// 验证任务
	if err := r.ValidateTarget(ctx, task.Target); err != nil {
		return nil, fmt.Errorf("target validation failed: %w", err)
	}

	// 注册活跃任务
	r.mu.Lock()
	r.activeTasks[task.ID] = task
	r.mu.Unlock()

	// 任务完成后清理
	defer func() {
		r.mu.Lock()
		delete(r.activeTasks, task.ID)
		r.mu.Unlock()
	}()

	// 初始化任务状态
	task.Status = StatusRunning
	task.Progress = &TaskProgress{
		TaskID:      task.ID,
		TotalTables: len(task.Tables),
		StartTime:   time.Now(),
		LastUpdate:  time.Now(),
	}

	result := &RecycleResult{
		TaskID:       task.ID,
		TableResults: make([]*TableRecycleResult, 0, len(task.Tables)),
	}

	// 通知进度开始
	r.notifyProgress(task.Progress)

	startTime := time.Now()

	// 根据模式执行回收
	switch task.Target.Mode {
	case ModeIndependent:
		err := r.executeIndependentMode(ctx, task, result)
		if err != nil {
			task.Status = StatusFailed
			task.Error = err.Error()
			return result, err
		}
	case ModeReplication:
		err := r.executeReplicationMode(ctx, task, result)
		if err != nil {
			task.Status = StatusFailed
			task.Error = err.Error()
			return result, err
		}
	default:
		return nil, fmt.Errorf("unsupported recycle mode: %d", task.Target.Mode)
	}

	// 完成任务
	result.TotalTime = time.Since(startTime)
	result.ProcessedTables = len(result.TableResults)

	for _, tr := range result.TableResults {
		if tr.Status == "success" {
			result.SuccessfulTables++
			result.ReclaimedSpace += tr.ReclaimedSpace
		} else if tr.Status == "failed" {
			result.FailedTables++
		} else {
			result.SkippedTables++
		}
	}

	task.Status = StatusCompleted
	task.Result = result

	// 最终进度通知
	task.Progress.CompletedTables = result.ProcessedTables
	task.Progress.ReclaimedSpace = result.ReclaimedSpace
	task.Progress.LastUpdate = time.Now()
	r.notifyProgress(task.Progress)

	return result, nil
}

// executeIndependentMode 执行独立节点模式
func (r *DefaultTablespaceRecycler) executeIndependentMode(ctx context.Context, task *RecycleTask, result *RecycleResult) error {
	conn, err := r.connectionFactory.CreateConnection(task.Target.Node)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// 关闭binlog记录
	if !task.Target.DryRun {
		_, err = conn.Query("SET SESSION sql_log_bin = OFF")
		if err != nil {
			return fmt.Errorf("failed to disable binlog: %w", err)
		}
	}

	// 并发处理表
	return r.processTablesWithConcurrency(ctx, task, result, conn)
}

// executeReplicationMode 执行复制同步模式
func (r *DefaultTablespaceRecycler) executeReplicationMode(ctx context.Context, task *RecycleTask, result *RecycleResult) error {
	conn, err := r.connectionFactory.CreateConnection(task.Target.Node)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// 保持binlog开启，串行处理以避免过大的复制延迟
	return r.processTablesSequentially(ctx, task, result, conn)
}

// processTablesWithConcurrency 并发处理表
func (r *DefaultTablespaceRecycler) processTablesWithConcurrency(ctx context.Context, task *RecycleTask, result *RecycleResult, conn DBConnection) error {
	concurrency := task.Target.Concurrency
	if concurrency <= 0 {
		concurrency = 3 // 默认并发数
	}

	// 创建信号量控制并发
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, table := range task.Tables {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(idx int, tbl *TableFragmentation) {
			defer func() {
				<-sem
				wg.Done()
			}()

			// 更新当前处理的表
			task.Progress.CurrentTable = fmt.Sprintf("%s.%s", tbl.Schema, tbl.TableName)
			task.Progress.CurrentTableSize = tbl.TotalSize
			r.notifyProgress(task.Progress)

			// 回收表
			tableResult := r.recycleTableInternal(ctx, task.Target, tbl, conn)

			// 更新结果
			mu.Lock()
			result.TableResults = append(result.TableResults, tableResult)
			task.Progress.CompletedTables = len(result.TableResults)
			task.Progress.ProcessedSize += tbl.TotalSize
			if tableResult.Status == "success" {
				task.Progress.ReclaimedSpace += tableResult.ReclaimedSpace
			}
			task.Progress.LastUpdate = time.Now()

			// 估算剩余时间
			if task.Progress.CompletedTables > 0 {
				avgTime := time.Since(task.Progress.StartTime) / time.Duration(task.Progress.CompletedTables)
				remaining := len(task.Tables) - task.Progress.CompletedTables
				task.Progress.EstimatedRemaining = avgTime * time.Duration(remaining)
			}
			mu.Unlock()

			r.notifyProgress(task.Progress)

			//time.Sleep(30 * time.Second)

		}(i, table)
	}

	wg.Wait()
	return nil
}

// processTablesSequentially 串行处理表
func (r *DefaultTablespaceRecycler) processTablesSequentially(ctx context.Context, task *RecycleTask, result *RecycleResult, conn DBConnection) error {
	for i, table := range task.Tables {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 更新进度
		task.Progress.CurrentTable = fmt.Sprintf("%s.%s", table.Schema, table.TableName)
		task.Progress.CurrentTableSize = table.TotalSize
		task.Progress.CompletedTables = i
		r.notifyProgress(task.Progress)

		// 回收表
		tableResult := r.recycleTableInternal(ctx, task.Target, table, conn)
		result.TableResults = append(result.TableResults, tableResult)

		// 更新统计
		task.Progress.ProcessedSize += table.TotalSize
		if tableResult.Status == "success" {
			task.Progress.ReclaimedSpace += tableResult.ReclaimedSpace
		}

		// 估算剩余时间
		if i > 0 {
			avgTime := time.Since(task.Progress.StartTime) / time.Duration(i+1)
			remaining := len(task.Tables) - i - 1
			task.Progress.EstimatedRemaining = avgTime * time.Duration(remaining)
		}

		task.Progress.LastUpdate = time.Now()
		r.notifyProgress(task.Progress)
	}

	return nil
}

// RecycleTable 回收单个表
func (r *DefaultTablespaceRecycler) RecycleTable(ctx context.Context, target *RecycleTarget, table *TableFragmentation) (*TableRecycleResult, error) {
	// 为 ALTER TABLE 操作创建专用连接，使用 ALTER 超时
	conn, err := r.createAlterConnection(target.Node)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	return r.recycleTableInternal(ctx, target, table, conn), nil
}

// createAlterConnection 创建 ALTER TABLE 专用连接
func (r *DefaultTablespaceRecycler) createAlterConnection(node *cluster.NodeConfig) (DBConnection, error) {
	// 创建使用 ALTER 超时的节点配置副本
	alterNodeConfig := &cluster.NodeConfig{
		Host:     node.Host,
		Port:     node.Port,
		Username: node.Username,
		Password: node.Password,
		Database: node.Database,
		Timeout:  node.Timeout,

		ConnectTimeout: node.ConnectTimeout,
		// 对于 ALTER TABLE 操作，使用 ALTER 超时作为查询超时
		QueryTimeout:     node.AlterTimeout,
		AlterTimeout:     node.AlterTimeout,
		DiscoveryTimeout: node.DiscoveryTimeout,
	}

	return r.connectionFactory.CreateConnection(alterNodeConfig)
}

// recycleTableInternal 内部表回收实现
func (r *DefaultTablespaceRecycler) recycleTableInternal(ctx context.Context, target *RecycleTarget, table *TableFragmentation, conn DBConnection) *TableRecycleResult {
	result := &TableRecycleResult{
		Schema:     table.Schema,
		TableName:  table.TableName,
		BeforeSize: table.TotalSize,
		Status:     "failed",
	}

	startTime := time.Now()
	defer func() {
		result.Duration = time.Since(startTime)
	}()

	// 检查表是否被锁定
	if err := r.checkTableLocks(ctx, conn, table.Schema, table.TableName); err != nil {
		result.Error = fmt.Sprintf("table is locked: %v", err)
		return result
	}

	// 模拟运行模式
	if target.DryRun {
		result.Status = "dry_run"
		result.ReclaimedSpace = int64(float64(table.DataFree) * 0.85) // 估算85%的回收率
		result.AfterSize = result.BeforeSize - result.ReclaimedSpace
		return result
	}

	// 执行ALTER TABLE重建
	alterSQL := fmt.Sprintf("ALTER TABLE `%s`.`%s` ENGINE = %s",
		table.Schema, table.TableName, table.Engine)

	_, err := conn.Query(alterSQL)
	if err != nil {
		result.Error = fmt.Sprintf("alter table failed: %v", err)
		return result
	}

	// 获取回收后的表信息
	updatedTable, err := r.analyzer.GetTableFragmentation(ctx, target.Node, table.Schema, table.TableName)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get updated table info: %v", err)
		return result
	}

	result.AfterSize = updatedTable.TotalSize
	result.ReclaimedSpace = result.BeforeSize - result.AfterSize
	result.Status = "success"

	return result
}

// checkTableLocks 检查表锁状态
func (r *DefaultTablespaceRecycler) checkTableLocks(ctx context.Context, conn DBConnection, schema, table string) error {
	query := `
		SELECT COUNT(*) 
		FROM information_schema.INNODB_LOCKS 
		WHERE lock_table = CONCAT(?, '.', ?)
	`

	var lockCount int
	err := conn.QueryRow(query, schema, table).Scan(&lockCount)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check locks: %w", err)
	}

	if lockCount > 0 {
		return fmt.Errorf("table has %d active locks", lockCount)
	}

	return nil
}

// ValidateTarget 验证回收目标
func (r *DefaultTablespaceRecycler) ValidateTarget(ctx context.Context, target *RecycleTarget) error {
	if target.Node == nil {
		return fmt.Errorf("target node is nil")
	}

	// 安全检查
	if target.SafetyChecks && r.validator != nil {
		if err := r.validator.ValidateMasterSafety(ctx, target.Node); err != nil {
			return fmt.Errorf("safety validation failed: %w", err)
		}
	}

	// 验证连接
	conn, err := r.connectionFactory.CreateConnection(target.Node)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer conn.Close()

	// 检查权限
	if err := r.checkPermissions(ctx, conn); err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	return nil
}

// checkPermissions 检查所需权限
func (r *DefaultTablespaceRecycler) checkPermissions(ctx context.Context, conn DBConnection) error {
	// 检查ALTER权限
	_, err := conn.Query("SHOW GRANTS")
	if err != nil {
		return fmt.Errorf("failed to check grants: %w", err)
	}

	// 这里可以添加更详细的权限检查逻辑
	return nil
}

// EstimateRecycleTime 估算回收时间
func (r *DefaultTablespaceRecycler) EstimateRecycleTime(target *RecycleTarget) (time.Duration, error) {
	if len(target.Tables) == 0 {
		return 0, nil
	}

	var totalTime time.Duration

	// 根据历史数据或表大小估算时间
	// 简化实现：每GB数据约需要2分钟
	var totalSize int64
	// 这里需要访问target中的表信息，暂时用简化逻辑
	totalSize = 1024 * 1024 * 1024 // 假设1GB

	estimatedMinutes := float64(totalSize) / float64(1024*1024*1024) * 2
	totalTime = time.Duration(estimatedMinutes) * time.Minute

	// 考虑并发因子
	if target.Concurrency > 1 {
		totalTime = totalTime / time.Duration(target.Concurrency)
	}

	return totalTime, nil
}

// notifyProgress 通知进度更新
func (r *DefaultTablespaceRecycler) notifyProgress(progress *TaskProgress) {
	r.mu.RLock()
	callbacks := r.progressCallbacks
	r.mu.RUnlock()

	for _, callback := range callbacks {
		go callback(progress)
	}
}

// GetActiveTask 获取活跃任务
func (r *DefaultTablespaceRecycler) GetActiveTask(taskID string) *RecycleTask {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.activeTasks[taskID]
}

// GetActiveTasks 获取所有活跃任务
func (r *DefaultTablespaceRecycler) GetActiveTasks() []*RecycleTask {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tasks := make([]*RecycleTask, 0, len(r.activeTasks))
	for _, task := range r.activeTasks {
		tasks = append(tasks, task)
	}

	return tasks
}
