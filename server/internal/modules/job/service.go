// Package job 承载平台侧任务管理：系统方法、HTTP/curl 与 Shell 任务的配置、调度和执行日志。
//
// 设计与权限点字典同口径：
//   - 内置任务由代码在启动装配期 RegisterBuiltin 注册执行体，syncBuiltins 只补不覆盖，
//     sys_job 里的 cron / 启停状态允许在页面治理，重启不会回滚治理结果；
//   - 调度器单进程 tick（30s）+ Redis 分布式锁兜底多实例，同一任务不会并发执行；
//   - 每次执行先落 running 日志再回写结果，任务页可分页追溯。
package job

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"pet-manager/server/internal/infra/cache"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
)

const (
	// tickInterval 是调度扫描周期：到期判断粒度为分钟，30s 扫描足够且不压库。
	tickInterval = 30 * time.Second
	// startupDelay 错开启动瞬间的迁移/建连高峰。
	startupDelay = 5 * time.Second
	// runTimeout 限制单次任务最长执行时间，避免坏任务永久占锁。
	runTimeout = 10 * time.Minute
	// runLockTTL 是分布式锁兜底 TTL（必须大于 runTimeout，防止执行中锁先过期）。
	runLockTTL = 30 * time.Minute
	// maxMessageLen 截断写入 sys_job.last_message 的长度，详细内容看 sys_job_log。
	maxMessageLen = 500
)

// TaskFunc 是可被调度的任务执行体，返回的字符串作为执行摘要落日志。
type TaskFunc func(ctx context.Context) (string, error)

// Builtin 声明一个代码内置任务。
type Builtin struct {
	Code   string
	Name   string
	Spec   string // 5 段 cron，注册时即校验
	Remark string
	Fn     TaskFunc
}

// Service 负责任务注册、调度循环与平台查询，同时实现 router.Module。
type Service struct {
	manager *db.Manager
	cache   *cache.Client
	log     *zap.Logger

	mu        sync.Mutex
	builtinFn map[string]TaskFunc // 方法编码 -> 执行体
	methods   map[string]Method   // 方法编码 -> 目录项
	builtins  []Builtin           // 待补齐到 sys_job 的内置任务定义
	shellPath string              // shell 任务使用的解释器路径
	// running 是进程内执行守卫，配合 Redis 锁形成双保险。
	running map[uint64]struct{}

	stop      chan struct{} // 通知调度循环退出
	done      chan struct{} // 调度循环退出完成信号
	started   atomic.Bool   // 防止 Run 重复启动
	closeOnce sync.Once
}

// NewService 构造任务服务；options 可覆盖 shell 解释器路径（默认 sh）。
func NewService(manager *db.Manager, cacheClient *cache.Client, log *zap.Logger, options ...ExecutorOptions) *Service {
	shellPath := "sh"
	if len(options) > 0 && options[0].ShellPath != "" {
		shellPath = options[0].ShellPath
	}
	return &Service{
		manager:   manager,
		cache:     cacheClient,
		log:       log,
		builtinFn: make(map[string]TaskFunc),
		methods:   make(map[string]Method),
		shellPath: shellPath,
		running:   make(map[uint64]struct{}),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// RegisterBuiltin 注册内置任务执行体，仅允许在 Run 之前（启动装配期）调用。
// cron 表达式在注册时即校验，坏表达式属于开发期问题，直接 panic 暴露。
func (s *Service) RegisterBuiltin(b Builtin) {
	if b.Code == "" || b.Fn == nil {
		panic(fmt.Sprintf("内置任务注册不完整: %+v", b))
	}
	if _, err := parseCron(b.Spec); err != nil {
		panic(fmt.Sprintf("内置任务 %s 的 cron 非法: %v", b.Code, err))
	}
	s.RegisterMethod(Method{Code: b.Code, Name: b.Name, Remark: b.Remark, Fn: b.Fn})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.builtins = append(s.builtins, b)
}

// Run 补齐内置任务行并启动调度循环，重复调用安全（幂等）。
func (s *Service) Run(ctx context.Context) {
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	if err := s.syncBuiltins(ctx); err != nil {
		// 任务字典落库失败不阻断启动：每轮 tick 都会重试补齐，调度循环照常起
		s.log.Error("初始化内置任务失败", zap.Error(err))
	}
	go s.loop(ctx)
}

// Close 停止调度循环（不中断已在执行的任务，它们由 runTimeout 兜底）。
func (s *Service) Close() {
	if !s.started.Load() {
		return
	}
	s.closeOnce.Do(func() { close(s.stop) })
	<-s.done
}

// loop 是调度主循环：启动延迟后开始，按 tickInterval 周期性扫描到期任务，
// 收到 ctx 取消或 Close 信号即退出并关闭 done。
func (s *Service) loop(ctx context.Context) {
	defer close(s.done)
	initial := time.NewTimer(startupDelay)
	defer initial.Stop()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-initial.C:
			s.dispatch(ctx)
		case <-ticker.C:
			s.dispatch(ctx)
		}
	}
}

// dispatch 扫描已启用且到期的任务并各自起 goroutine 执行；
// 是否真的执行由进程守卫 + Redis 锁最终决定。
func (s *Service) dispatch(ctx context.Context) {
	if err := s.syncBuiltins(ctx); err != nil {
		s.log.Warn("补齐内置任务失败", zap.Error(err))
	}

	var jobs []model.Job
	if err := s.manager.Platform().WithContext(ctx).
		Where("status = ?", model.StatusEnabled).Find(&jobs).Error; err != nil {
		s.log.Warn("扫描定时任务失败", zap.Error(err))
		return
	}

	now := time.Now()
	for i := range jobs {
		j := jobs[i]
		if j.NextRunAt == nil || j.NextRunAt.After(now) {
			continue
		}
		go func(j model.Job) {
			if err := s.execute(context.WithoutCancel(ctx), j.ID, model.JobTriggerSchedule); err != nil {
				s.log.Warn("调度执行任务失败", zap.String("code", j.Code), zap.Error(err))
			}
		}(j)
	}
}

// execute 执行一次任务并全程留痕。
// 返回错误仅表示"没能进入执行"（不存在 / 正在执行 / 抢锁失败）；
// 任务自身的业务成败只体现在执行日志与任务最近状态上，不向调度器返回错误。
func (s *Service) execute(ctx context.Context, jobID uint64, trigger string) error {
	run, err := s.prepareRun(ctx, jobID, trigger)
	if err != nil {
		return err
	}
	if run != nil {
		run()
	}
	return nil
}

// prepareRun 在异步派发前占住任务及方法锁，避免连续点击或多实例重复接单。
func (s *Service) prepareRun(ctx context.Context, jobID uint64, trigger string) (func(), error) {
	var j model.Job
	p := s.manager.Platform().WithContext(ctx)
	if err := p.First(&j, jobID).Error; err != nil {
		return nil, jobLookupError(err)
	}
	s.mu.Lock()
	if _, busy := s.running[j.ID]; busy {
		s.mu.Unlock()
		return nil, errcode.Newf(errcode.ErrConflict, "任务正在执行中，请稍后再试")
	}
	s.running[j.ID] = struct{}{}
	s.mu.Unlock()
	releaseLocal := func() { s.mu.Lock(); delete(s.running, j.ID); s.mu.Unlock() }
	keep := false
	defer func() {
		if !keep {
			releaseLocal()
		}
	}()

	fn, identity, taskErr := s.task(j)
	if taskErr != nil {
		identity = "job:" + j.Code
	}
	releaseRedis, err := s.acquireRunLocks(ctx, j.Code, identity)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !keep {
			releaseRedis()
		}
	}()
	// 抢锁后重新校验，已经被其他实例完成/停用/修改的调度不应继续执行。
	var current model.Job
	if err := p.First(&current, jobID).Error; err != nil {
		return nil, jobLookupError(err)
	}
	if trigger == model.JobTriggerSchedule && (current.Status != model.StatusEnabled || current.NextRunAt == nil || current.NextRunAt.After(time.Now())) {
		return nil, nil
	}
	currentFn, currentIdentity, currentErr := s.task(current)
	if currentErr != nil {
		currentIdentity = "job:" + current.Code
	}
	if currentIdentity != identity {
		return nil, errcode.Newf(errcode.ErrConflict, "执行参数已变更，请重试")
	}
	j, fn, taskErr = current, currentFn, currentErr
	if taskErr != nil {
		fn = func(context.Context) (string, error) { return "", taskErr }
	}
	started := time.Now()
	logRow := s.openLog(ctx, j, trigger, started)
	if logRow == nil {
		return nil, errcode.Newf(errcode.ErrDatabase, "无法记录执行日志，任务未启动")
	}
	_ = p.Model(&model.Job{}).Where("id = ?", j.ID).Updates(map[string]any{
		"last_run_at": started, "last_status": model.JobRunRunning, "last_message": "执行中",
	}).Error
	keep = true
	return func() {
		defer releaseLocal()
		defer releaseRedis()
		timeout := j.TimeoutSeconds
		if timeout < 1 || timeout > int(runTimeout/time.Second) {
			timeout = int(runTimeout / time.Second)
		}
		runCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()
		summary, runErr := invoke(runCtx, fn)
		s.completeRun(context.Background(), j, logRow, started, limitOutput(summary), runErr)
	}, nil
}

// jobLookupError 把"记录不存在"归一化为 404，其余包装为库错误。
func jobLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.New(errcode.ErrNotFound)
	}
	return errcode.Wrap(errcode.ErrDatabase, err)
}

// openLog 落一条 running 执行记录，返回行指针（拿不到 ID 时返回 nil）。
func (s *Service) openLog(ctx context.Context, j model.Job, trigger string, started time.Time) *model.JobLog {
	row := &model.JobLog{
		JobID:     j.ID,
		JobCode:   j.Code,
		Trigger:   trigger,
		Status:    model.JobRunRunning,
		StartedAt: started,
	}
	if err := s.manager.Platform().WithContext(ctx).Create(row).Error; err != nil {
		s.log.Warn("写入任务执行日志失败", zap.String("code", j.Code), zap.Error(err))
		return nil
	}
	return row
}

// completeRun 回写执行日志并刷新任务的最近执行状态与下次执行时间。
// 失败时 summary 作为错误上下文前缀拼在 runErr 前。
func (s *Service) completeRun(ctx context.Context, j model.Job, row *model.JobLog, started time.Time, summary string, runErr error) {
	finished := time.Now()
	status := model.JobRunSuccess
	result := strings.TrimSpace(summary)
	if runErr != nil {
		status = model.JobRunFail
		if result != "" {
			result = result + ": " + runErr.Error()
		} else {
			result = runErr.Error()
		}
	}
	if result == "" {
		result = "执行完成"
	}

	p := s.manager.Platform().WithContext(ctx)
	if row != nil {
		if err := p.Model(&model.JobLog{}).Where("id = ?", row.ID).Updates(map[string]any{
			"status":      status,
			"finished_at": finished,
			"cost_ms":     finished.Sub(started).Milliseconds(),
			"result":      result,
		}).Error; err != nil {
			s.log.Warn("回写任务执行日志失败", zap.String("code", j.Code), zap.Error(err))
		}
	}

	lastMsg := result
	if len([]rune(lastMsg)) > maxMessageLen {
		lastMsg = string([]rune(lastMsg)[:maxMessageLen])
	}
	err := p.Transaction(func(tx *gorm.DB) error {
		var latest model.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&latest, j.ID).Error; err != nil {
			return err
		}
		updates := map[string]any{"last_run_at": started, "last_status": status, "last_message": lastMsg}
		if row != nil && row.Trigger == model.JobTriggerSchedule {
			var next *time.Time
			if latest.Status == model.StatusEnabled {
				next = s.nextRunAt(latest.Spec, finished)
			}
			updates["next_run_at"] = next
		}
		return tx.Model(&model.Job{}).Where("id = ?", j.ID).Updates(updates).Error
	})
	if err != nil {
		s.log.Warn("刷新任务最近执行状态失败", zap.String("code", j.Code), zap.Error(err))
	}
}

// nextRunAt 按 cron 计算 from 之后的下次执行时间；表达式非法时返回 nil（清空调度）。
func (s *Service) nextRunAt(spec string, from time.Time) *time.Time {
	sc, err := parseCron(spec)
	if err != nil {
		return nil
	}
	next := sc.next(from)
	if next.IsZero() {
		return nil
	}
	return &next
}

// syncBuiltins 把代码注册的内置任务补齐到 sys_job（只补不覆盖）。
// 与权限点 seed 同口径：已存在（含被软删）的 code 一律跳过，页面治理结果不被重启覆盖。
func (s *Service) syncBuiltins(ctx context.Context) error {
	s.mu.Lock()
	defs := append([]Builtin(nil), s.builtins...)
	s.mu.Unlock()
	if len(defs) == 0 {
		return nil
	}

	p := s.manager.Platform().WithContext(ctx)
	var existing []model.Job
	if err := p.Unscoped().Select("code").Find(&existing).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	has := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		has[e.Code] = struct{}{}
	}

	for _, b := range defs {
		if _, ok := has[b.Code]; ok {
			continue
		}
		now := time.Now()
		row := model.Job{
			Code:           b.Code,
			Name:           b.Name,
			Kind:           model.JobKindSystem,
			Spec:           b.Spec,
			Status:         model.StatusEnabled,
			Remark:         b.Remark,
			Builtin:        true,
			Config:         encodeConfig(ExecutionConfig{Handler: b.Code}),
			TimeoutSeconds: int(runTimeout / time.Second),
			NextRunAt:      s.nextRunAt(b.Spec, now),
		}
		if err := p.Create(&row).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		s.log.Info("已注册内置定时任务", zap.String("code", b.Code), zap.String("spec", b.Spec))
	}
	return nil
}

// —— 平台管理接口 ——

// Item 是任务列表返回结构。
type Item struct {
	ID             uint64          `json:"id"`
	Code           string          `json:"code"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Spec           string          `json:"spec"`
	Status         string          `json:"status"`
	Remark         string          `json:"remark"`
	Builtin        bool            `json:"builtin"`
	Config         ExecutionConfig `json:"config"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	LastRunAt      *time.Time      `json:"last_run_at"`
	LastStatus     string          `json:"last_status"`
	LastMessage    string          `json:"last_message"`
	NextRunAt      *time.Time      `json:"next_run_at"`
	CreatedAt      time.Time       `json:"created_at"`
}

// LogItem 是执行日志返回结构。
type LogItem struct {
	ID         uint64     `json:"id"`
	JobID      uint64     `json:"job_id"`
	JobCode    string     `json:"job_code"`
	Trigger    string     `json:"trigger"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	CostMs     int64      `json:"cost_ms"`
	Result     string     `json:"result"`
}

// UpdateInput 修改任务调度参数。
type UpdateInput struct {
	Spec           string           `json:"spec" binding:"required,max=64"`
	Remark         string           `json:"remark" binding:"max=255"`
	Name           *string          `json:"name" binding:"omitempty,max=128"`
	Kind           *string          `json:"kind"`
	Config         *ExecutionConfig `json:"config"`
	TimeoutSeconds *int             `json:"timeout_seconds"`
}

// LogListQuery 是执行日志分页查询。
type LogListQuery struct {
	pagination.Query
	Status  string `form:"status"`
	Trigger string `form:"trigger"`
}

// List 平台侧任务列表，按 ID 升序（与注册顺序一致）。
func (s *Service) List(ctx context.Context) ([]Item, error) {
	var rows []model.Job
	if err := s.manager.Platform().WithContext(ctx).Order("id asc").Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	items := make([]Item, 0, len(rows))
	for _, r := range rows {
		items = append(items, toItem(r))
	}
	return items, nil
}

// Trigger 手动执行一次：仅做存在性与忙碌校验，实际执行异步进行，结果看执行日志。
func (s *Service) Trigger(ctx context.Context, id uint64) error {
	run, err := s.prepareRun(ctx, id, model.JobTriggerManual)
	if err != nil {
		return err
	}
	if run != nil {
		go run()
	}
	return nil
}

// SetEnabled 启用/停用任务；启用时重算下次执行时间，停用时清空。
func (s *Service) SetEnabled(ctx context.Context, id uint64, enabled bool) error {
	err := s.manager.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var j model.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&j, id).Error; err != nil {
			return jobLookupError(err)
		}
		status := model.StatusDisabled
		var nextRun *time.Time
		if enabled {
			status = model.StatusEnabled
			validated, err := s.definition(CreateInput{Code: j.Code, Name: j.Name, Kind: j.Kind, Spec: j.Spec,
				Status: status, Remark: j.Remark, Config: decodeConfig(j), TimeoutSeconds: j.TimeoutSeconds})
			if err != nil {
				return err
			}
			nextRun = validated.NextRunAt
		}
		return tx.Model(&model.Job{}).Where("id = ?", id).
			Updates(map[string]any{"status": status, "next_run_at": nextRun}).Error
	})
	return writeError(err)
}

// Update 修改任务定义，写入即校验；启用中的任务同步重算下次执行时间。
func (s *Service) Update(ctx context.Context, id uint64, in UpdateInput) error {
	return s.updateDefinition(ctx, id, in)
}

// ListLogs 分页查询某任务的执行日志，按开始时间倒序。
func (s *Service) ListLogs(ctx context.Context, jobID uint64, q LogListQuery) (*response.Page[LogItem], error) {
	var j model.Job
	if err := s.manager.Platform().WithContext(ctx).Unscoped().First(&j, jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	tx := s.manager.Platform().WithContext(ctx).Model(&model.JobLog{}).Where("job_id = ?", jobID)
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Trigger != "" {
		tx = tx.Where("trigger = ?", q.Trigger)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	var rows []model.JobLog
	if err := tx.Order("started_at desc, id desc").
		Offset(q.Offset()).Limit(q.Limit()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	items := make([]LogItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, LogItem{
			ID:         r.ID,
			JobID:      r.JobID,
			JobCode:    r.JobCode,
			Trigger:    r.Trigger,
			Status:     r.Status,
			StartedAt:  r.StartedAt,
			FinishedAt: r.FinishedAt,
			CostMs:     r.CostMs,
			Result:     r.Result,
		})
	}
	return response.NewPage(items, total), nil
}

// toItem 把任务行转为列表/详情返回结构，并还原其执行配置。
func toItem(r model.Job) Item {
	return Item{
		ID:             r.ID,
		Code:           r.Code,
		Name:           r.Name,
		Kind:           r.Kind,
		Spec:           r.Spec,
		Status:         r.Status,
		Remark:         r.Remark,
		Builtin:        r.Builtin,
		Config:         decodeConfig(r),
		TimeoutSeconds: r.TimeoutSeconds,
		LastRunAt:      r.LastRunAt,
		LastStatus:     r.LastStatus,
		LastMessage:    r.LastMessage,
		NextRunAt:      r.NextRunAt,
		CreatedAt:      r.CreatedAt,
	}
}
