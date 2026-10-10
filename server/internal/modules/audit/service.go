// Package audit 承载审计日志的采集与查询。
//
// 采集侧：中间件非阻塞投递到带缓冲队列，单 worker 落库，审计失败绝不影响主请求；
// 查询侧：平台侧只读列表，支持租户 / 账号 / 方法 / 路径 / 状态 / 时间筛选。
package audit

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/middleware"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
)

const (
	// queueSize 是审计投递缓冲深度，按"峰值写请求数"估算，满则丢弃而非阻塞。
	queueSize = 1024
	// writeTimeout 是单条落库超时，避免慢库拖住 worker。
	writeTimeout = 3 * time.Second
)

// Service 同时实现 middleware.AuditRecorder（采集）与平台查询接口（只读）。
type Service struct {
	manager *db.Manager
	log     *zap.Logger

	// policy 是采集策略快照，整体原子替换；请求路径只做一次原子读，零锁零 DB。
	policy    atomic.Pointer[policy]
	queue     chan middleware.AuditEntry // 采集投递缓冲队列，满则丢弃（非阻塞）
	stop      chan struct{}              // 关闭即通知 worker 停止并 flush
	done      chan struct{}              // worker 退出信号，Close 等待其关闭
	running   atomic.Bool                // 保证 Run 幂等
	closeOnce sync.Once                  // 保证 stop 只关闭一次
	dropped   atomic.Uint64              // 累计丢弃条数，用于告警节流
}

// NewService 构造审计服务；先装载内置默认策略兜底，Run 时再回源 sys_config。
func NewService(manager *db.Manager, log *zap.Logger) *Service {
	s := &Service{
		manager: manager,
		log:     log,
		queue:   make(chan middleware.AuditEntry, queueSize),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	// 启动前先用内置默认策略兜底，Run 时再回源 sys_config。
	s.policy.Store(defaultPolicy())
	return s
}

// ShouldRecord 实现采集决策：总开关 / HTTP 方法 / 排除路由均由参数配置治理。
// 路由优先传 gin 模板（c.FullPath），未命中路由时传实际路径。
func (s *Service) ShouldRecord(method, route string) bool {
	p := s.policy.Load()
	if p == nil {
		p = defaultPolicy()
	}
	return p.allows(method, route)
}

// refreshPolicy 从 sys_config 重新加载策略；加载失败时保留当前快照，避免配置库抖动影响审计。
func (s *Service) refreshPolicy(ctx context.Context) {
	p := loadPolicy(ctx, s.manager, s.log)
	if p != nil {
		s.policy.Store(p)
	}
}

// Submit 实现 middleware.AuditRecorder。
// 非阻塞投递：队列满即丢弃并计数告警，绝不阻塞 HTTP 主请求。
func (s *Service) Submit(e middleware.AuditEntry) {
	select {
	case s.queue <- e:
	default:
		n := s.dropped.Add(1)
		// 首次与每满 1000 条告警一次，队列持续满时不刷屏
		if n == 1 || n%1000 == 0 {
			s.log.Warn("审计队列已满，丢弃审计日志",
				zap.Uint64("dropped_total", n), zap.String("request_id", e.RequestID))
		}
	}
}

// Run 启动落库 worker 与策略定时刷新。重复调用安全（幂等）。
func (s *Service) Run(ctx context.Context) {
	if !s.running.CompareAndSwap(false, true) {
		return
	}
	// 同步回源一次策略，确保监听前已采用参数表配置；失败则继续用默认快照。
	s.refreshPolicy(ctx)
	go s.loop(ctx)
}

// Close 停止 worker 并把队列中剩余记录 flush 掉。用于优雅退出。
func (s *Service) Close() {
	if !s.running.Load() {
		return
	}
	s.closeOnce.Do(func() { close(s.stop) })
	<-s.done
}

func (s *Service) loop(ctx context.Context) {
	defer close(s.done)
	// 与落库共用单 worker：策略刷新频率低、有超时，不值得单开 goroutine。
	ticker := time.NewTicker(policyTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.drain()
			return
		case <-s.stop:
			s.drain()
			return
		case e := <-s.queue:
			s.write(e)
		case <-ticker.C:
			s.refreshPolicy(ctx)
		}
	}
}

// drain 尽量写完队列里已缓冲的记录（退出前 flush）。
func (s *Service) drain() {
	for {
		select {
		case e := <-s.queue:
			s.write(e)
		default:
			return
		}
	}
}

func (s *Service) write(e middleware.AuditEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	row := model.AuditLog{
		TenantID:  e.TenantID,
		UserType:  e.UserType,
		UserID:    e.UserID,
		Username:  e.Username,
		Method:    e.Method,
		Path:      e.Path,
		Route:     e.Route,
		Status:    e.Status,
		ClientIP:  e.ClientIP,
		UserAgent: e.UserAgent,
		LatencyMs: e.LatencyMs,
		RequestID: e.RequestID,
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&row).Error; err != nil {
		s.log.Warn("写入审计日志失败", zap.Error(err), zap.String("request_id", e.RequestID))
	}
}

// —— 查询 ——

// Item 是对外的审计日志条目。
type Item struct {
	ID        uint64    `json:"id"`         // 主键
	TenantID  uint64    `json:"tenant_id"`  // 所属租户，0 表示平台侧事件
	UserType  string    `json:"user_type"`  // 操作者类型：platform / tenant
	UserID    uint64    `json:"user_id"`    // 操作者 ID
	Username  string    `json:"username"`   // 操作者登录名
	Method    string    `json:"method"`     // HTTP 方法
	Path      string    `json:"path"`       // 实际请求路径
	Route     string    `json:"route"`      // gin 路由模板（不含具体 id）
	Status    int       `json:"status"`     // HTTP 响应状态码
	ClientIP  string    `json:"client_ip"`  // 客户端 IP
	UserAgent string    `json:"user_agent"` // User-Agent
	LatencyMs int64     `json:"latency_ms"` // 请求耗时（毫秒）
	RequestID string    `json:"request_id"` // 请求追踪 ID
	CreatedAt time.Time `json:"created_at"` // 记录时间
}

// ListQuery 是审计日志的列表查询条件，字段全部可选。
type ListQuery struct {
	pagination.Query
	TenantID uint64 `form:"tenant_id"` // 按租户筛选，0/不传为全部
	UserID   uint64 `form:"user_id"`   // 按操作者 ID 筛选
	Username string `form:"username"`  // 按操作者登录名模糊筛选
	Method   string `form:"method"`    // 按 HTTP 方法精确筛选
	Path     string `form:"path"`      // 按请求路径模糊筛选
	Status   int    `form:"status"`    // 按响应状态码筛选
	Start    string `form:"start"`     // 起始时间（含）
	End      string `form:"end"`       // 结束时间（含）
}

// timeLayouts 是时间筛选参数可接受的格式，宽松兼容前端常见传法。
var timeLayouts = []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}

// parseBound 解析时间边界；isEnd 且仅给到日期时取次日零点，以覆盖整天。
func parseBound(v string, isEnd bool) (time.Time, error) {
	for _, layout := range timeLayouts {
		t, err := time.ParseInLocation(layout, v, time.Local)
		if err != nil {
			continue
		}
		// 只给到日期时，结束边界取次日零点，覆盖整天
		if isEnd && layout == "2006-01-02" {
			return t.AddDate(0, 0, 1), nil
		}
		return t, nil
	}
	return time.Time{}, errcode.Newf(errcode.ErrInvalidParam, "时间格式不合法: %s", v)
}

// List 平台侧分页查询审计日志，按时间倒序。
func (s *Service) List(ctx context.Context, in ListQuery) (*response.Page[Item], error) {
	q := s.manager.Platform().WithContext(ctx).Model(&model.AuditLog{})

	if in.TenantID > 0 {
		q = q.Where("tenant_id = ?", in.TenantID)
	}
	if in.UserID > 0 {
		q = q.Where("user_id = ?", in.UserID)
	}
	if kw := strings.TrimSpace(in.Username); kw != "" {
		q = q.Where("username ILIKE ?", "%"+kw+"%")
	}
	if m := strings.TrimSpace(in.Method); m != "" {
		q = q.Where("method = ?", strings.ToUpper(m))
	}
	if p := strings.TrimSpace(in.Path); p != "" {
		q = q.Where("path ILIKE ?", "%"+p+"%")
	}
	if in.Status > 0 {
		q = q.Where("status = ?", in.Status)
	}
	if s := strings.TrimSpace(in.Start); s != "" {
		t, err := parseBound(s, false)
		if err != nil {
			return nil, err
		}
		q = q.Where("created_at >= ?", t)
	}
	if e := strings.TrimSpace(in.End); e != "" {
		t, err := parseBound(e, true)
		if err != nil {
			return nil, err
		}
		q = q.Where("created_at <= ?", t)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	var rows []model.AuditLog
	if err := q.Order("id desc").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	list := make([]Item, 0, len(rows))
	for _, r := range rows {
		list = append(list, Item{
			ID:        r.ID,
			TenantID:  r.TenantID,
			UserType:  r.UserType,
			UserID:    r.UserID,
			Username:  r.Username,
			Method:    r.Method,
			Path:      r.Path,
			Route:     r.Route,
			Status:    r.Status,
			ClientIP:  r.ClientIP,
			UserAgent: r.UserAgent,
			LatencyMs: r.LatencyMs,
			RequestID: r.RequestID,
			CreatedAt: r.CreatedAt,
		})
	}
	return response.NewPage(list, total), nil
}
