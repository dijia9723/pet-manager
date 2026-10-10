// Package loginlog 承载登录日志的采集与查询。
//
// 采集侧：auth 模块在登录成功/失败后调用 Record 落库（best-effort：失败仅告警，
// 绝不影响登录响应）；查询侧：平台侧只读列表。
// 登录是低频操作，采用同步写而非 audit 那套非阻塞队列；本模块只读，不暴露任何写接口。
package loginlog

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/modules/auth"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
)

// writeTimeout 是单条落库超时，避免慢库拖住登录请求。
const writeTimeout = 3 * time.Second

// Service 同时实现 auth.LoginRecorder（采集）与平台查询接口（只读）。
type Service struct {
	manager *db.Manager
	log     *zap.Logger
}

// NewService 构造登录日志服务；采集为 best-effort 同步写，失败仅告警。
func NewService(manager *db.Manager, log *zap.Logger) *Service {
	return &Service{manager: manager, log: log}
}

// Record 实现 auth.LoginRecorder：写入一条登录日志。
// best-effort：使用与请求生命周期解耦的超时上下文，失败只告警，绝不向上抛错。
func (s *Service) Record(ctx context.Context, r auth.LoginRecord) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()

	row := model.LoginLog{
		Username:   r.Username,
		UserType:   r.UserType,
		TenantID:   r.TenantID,
		TenantCode: r.TenantCode,
		ClientIP:   r.ClientIP,
		UserAgent:  r.UserAgent,
		Status:     r.Status,
		Message:    r.Message,
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&row).Error; err != nil {
		s.log.Warn("写入登录日志失败", zap.String("username", r.Username), zap.Error(err))
	}
}

// —— 查询 ——

// Item 是对外的登录日志条目。
type Item struct {
	ID         uint64    `json:"id"`          // 主键
	Username   string    `json:"username"`    // 登录名
	UserType   string    `json:"user_type"`   // 登录主体类型：platform / tenant
	TenantID   uint64    `json:"tenant_id"`   // 所属租户，0 表示平台侧
	TenantCode string    `json:"tenant_code"` // 租户编码（便于排查定位）
	ClientIP   string    `json:"client_ip"`   // 客户端 IP
	UserAgent  string    `json:"user_agent"`  // User-Agent
	Status     string    `json:"status"`      // 结果：success / fail
	Message    string    `json:"message"`     // 结果说明（失败原因等）
	CreatedAt  time.Time `json:"created_at"`  // 记录时间
}

// ListQuery 是登录日志的列表查询条件，字段全部可选。
type ListQuery struct {
	pagination.Query
	Username string `form:"username"`  // 按登录名模糊筛选
	UserType string `form:"user_type"` // 按登录主体类型筛选
	TenantID uint64 `form:"tenant_id"` // 按租户筛选，0/不传为全部
	Status   string `form:"status"`    // 按结果筛选 success/fail
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

// List 平台侧分页查询登录日志，按时间倒序。
func (s *Service) List(ctx context.Context, in ListQuery) (*response.Page[Item], error) {
	q := s.manager.Platform().WithContext(ctx).Model(&model.LoginLog{})

	if kw := strings.TrimSpace(in.Username); kw != "" {
		q = q.Where("username ILIKE ?", "%"+kw+"%")
	}
	if ut := strings.TrimSpace(in.UserType); ut != "" {
		q = q.Where("user_type = ?", ut)
	}
	if in.TenantID > 0 {
		q = q.Where("tenant_id = ?", in.TenantID)
	}
	if st := strings.TrimSpace(in.Status); st != "" {
		q = q.Where("status = ?", st)
	}
	if s1 := strings.TrimSpace(in.Start); s1 != "" {
		t, err := parseBound(s1, false)
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

	var rows []model.LoginLog
	if err := q.Order("id desc").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	list := make([]Item, 0, len(rows))
	for _, r := range rows {
		list = append(list, Item{
			ID:         r.ID,
			Username:   r.Username,
			UserType:   r.UserType,
			TenantID:   r.TenantID,
			TenantCode: r.TenantCode,
			ClientIP:   r.ClientIP,
			UserAgent:  r.UserAgent,
			Status:     r.Status,
			Message:    r.Message,
			CreatedAt:  r.CreatedAt,
		})
	}
	return response.NewPage(list, total), nil
}
