// Package permission 承载平台侧的权限点（菜单）可视化配置。
//
// 权限点字典是"菜单 + 按钮"的唯一来源：
//   - 平台侧菜单、租户侧菜单、角色授权树都从 sys_permission 读取；
//   - 这里提供增删改，使菜单/按钮无需改代码重启即可治理（seed 只补不覆盖）；
//   - 变更会影响所有租户的权限解析，因此写操作后全量失效权限缓存。
package permission

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
)

// Node 是权限点树节点，含菜单/按钮的全部可配置字段。
type Node struct {
	ID       uint64  `json:"id"`                 // 权限点 ID
	ParentID uint64  `json:"parent_id"`          // 父节点 ID，0 为根
	Code     string  `json:"code"`               // 权限点编码
	Name     string  `json:"name"`               // 显示名称
	Type     string  `json:"type"`               // menu / button
	Scope    string  `json:"scope"`              // platform / tenant
	Path     string  `json:"path,omitempty"`     // 菜单路由
	Icon     string  `json:"icon,omitempty"`     // 图标名
	Sort     int     `json:"sort"`               // 排序
	Children []*Node `json:"children,omitempty"` // 子节点
}

// Input 是新增/编辑权限点的入参。
type Input struct {
	ParentID uint64 `json:"parent_id"`                                      // 父节点 ID，0 为根
	Code     string `json:"code" binding:"required,max=128"`                // 权限点编码
	Name     string `json:"name" binding:"required,max=64"`                 // 显示名称
	Type     string `json:"type" binding:"required,oneof=menu button"`      // menu / button
	Scope    string `json:"scope" binding:"required,oneof=platform tenant"` // platform / tenant
	Path     string `json:"path" binding:"max=128"`                         // 菜单路由
	Icon     string `json:"icon" binding:"max=64"`                          // 图标名
	Sort     int    `json:"sort"`                                           // 排序
}

// Service 承载平台侧权限点（菜单）配置，写操作后全量失效权限缓存。
type Service struct {
	manager  *db.Manager    // 数据库管理器
	sessions *session.Store // 会话与权限缓存
	log      *zap.Logger    // 日志
}

// NewService 组装 permission 服务。
func NewService(manager *db.Manager, sessions *session.Store, log *zap.Logger) *Service {
	return &Service{manager: manager, sessions: sessions, log: log}
}

// Tree 返回全部权限点树（平台 + 租户两个 scope），供配置页展示。
func (s *Service) Tree(ctx context.Context) ([]*Node, error) {
	var list []model.Permission
	if err := s.manager.Platform().WithContext(ctx).
		Order("sort asc, id asc").Find(&list).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return buildTree(list), nil
}

// Create 新增权限点。
func (s *Service) Create(ctx context.Context, in Input) (*Node, error) {
	in.Code = strings.TrimSpace(in.Code)
	if err := s.validate(ctx, 0, in); err != nil {
		return nil, err
	}

	p := model.Permission{
		ParentID: in.ParentID,
		Code:     in.Code,
		Name:     in.Name,
		Type:     in.Type,
		Scope:    in.Scope,
		Path:     in.Path,
		Icon:     in.Icon,
		Sort:     in.Sort,
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&p).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	s.invalidate(ctx)
	node := toNode(p)
	return &node, nil
}

// Update 编辑权限点。
func (s *Service) Update(ctx context.Context, id uint64, in Input) (*Node, error) {
	tx := s.manager.Platform().WithContext(ctx)
	var p model.Permission
	if err := tx.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	in.Code = strings.TrimSpace(in.Code)
	if err := s.validate(ctx, id, in); err != nil {
		return nil, err
	}

	p.ParentID = in.ParentID
	p.Code = in.Code
	p.Name = in.Name
	p.Type = in.Type
	p.Scope = in.Scope
	p.Path = in.Path
	p.Icon = in.Icon
	p.Sort = in.Sort
	if err := tx.Save(&p).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	s.invalidate(ctx)
	node := toNode(p)
	return &node, nil
}

// Delete 软删权限点。存在存活子节点直接拒绝。
// 不做跨租户同步清理：残留在各租户 schema 的 t_role_permission 行无害，
// 且权限解析已按 deleted_at 过滤，角色下次保存时自然丢弃。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	tx := s.manager.Platform().WithContext(ctx)

	var p model.Permission
	if err := tx.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.ErrNotFound)
		}
		return errcode.Wrap(errcode.ErrDatabase, err)
	}

	var children int64
	if err := tx.Model(&model.Permission{}).Where("parent_id = ?", id).Count(&children).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if children > 0 {
		return errcode.Newf(errcode.ErrConflict, "存在 %d 个存活子节点，请先删除子节点", children)
	}

	res := tx.Delete(&model.Permission{}, id)
	if res.Error != nil {
		return errcode.Wrap(errcode.ErrDatabase, res.Error)
	}
	if res.RowsAffected == 0 {
		return errcode.New(errcode.ErrNotFound)
	}
	s.invalidate(ctx)
	return nil
}

// validate 校验编码存活唯一、父节点存在且 scope 一致、父节点不得为自身或自身子孙（防环）。
func (s *Service) validate(ctx context.Context, selfID uint64, in Input) error {
	tx := s.manager.Platform().WithContext(ctx)

	if in.Code == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "权限点编码不能为空")
	}

	q := tx.Model(&model.Permission{}).Where("code = ?", in.Code)
	if selfID > 0 {
		q = q.Where("id <> ?", selfID)
	}
	var dup int64
	if err := q.Count(&dup).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if dup > 0 {
		return errcode.Newf(errcode.ErrConflict, "权限点编码已存在")
	}

	if in.ParentID == 0 {
		return nil
	}
	if in.ParentID == selfID {
		return errcode.Newf(errcode.ErrInvalidParam, "父节点不能是自身")
	}

	var parent model.Permission
	if err := tx.First(&parent, in.ParentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.Newf(errcode.ErrInvalidParam, "父节点不存在")
		}
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if parent.Scope != in.Scope {
		return errcode.Newf(errcode.ErrInvalidParam, "父节点与当前节点 scope 不一致")
	}

	if selfID > 0 {
		cycle, err := s.wouldCycle(ctx, selfID, in.ParentID)
		if err != nil {
			return err
		}
		if cycle {
			return errcode.Newf(errcode.ErrInvalidParam, "父节点不能是自身或自身子孙")
		}
	}
	return nil
}

// wouldCycle 从候选父节点沿父指针上溯，若遇到 selfID 说明挂载会形成环。
func (s *Service) wouldCycle(ctx context.Context, selfID, parentID uint64) (bool, error) {
	var all []model.Permission
	if err := s.manager.Platform().WithContext(ctx).
		Select("id, parent_id").Find(&all).Error; err != nil {
		return false, errcode.Wrap(errcode.ErrDatabase, err)
	}
	parentOf := make(map[uint64]uint64, len(all))
	for _, p := range all {
		parentOf[p.ID] = p.ParentID
	}
	for cur := parentID; cur != 0; cur = parentOf[cur] {
		if cur == selfID {
			return true, nil
		}
	}
	return false, nil
}

// invalidate 全量失效所有租户的权限缓存：权限点字典变更影响面是全平台。
func (s *Service) invalidate(ctx context.Context) {
	if err := s.sessions.InvalidateAllPermissions(ctx); err != nil {
		s.log.Warn("失效全量权限缓存失败", zap.Error(err))
	}
}

// toNode 把权限点模型映射为树节点。
func toNode(p model.Permission) Node {
	return Node{
		ID: p.ID, ParentID: p.ParentID, Code: p.Code, Name: p.Name,
		Type: p.Type, Scope: p.Scope, Path: p.Path, Icon: p.Icon, Sort: p.Sort,
	}
}

// buildTree 把扁平权限点列表按 ParentID 组装成菜单配置树。
func buildTree(list []model.Permission) []*Node {
	nodes := make(map[uint64]*Node, len(list))
	for _, p := range list {
		n := toNode(p)
		nodes[p.ID] = &n
	}
	roots := make([]*Node, 0, len(list))
	for _, p := range list {
		node := nodes[p.ID]
		parent, ok := nodes[p.ParentID]
		if !ok {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}
