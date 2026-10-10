// Package rbac 负责权限点字典、租户角色与授权关系。
package rbac

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
)

// —— DTO ——

// PermissionNode 是权限点树节点，用于角色授权的树形勾选。
type PermissionNode struct {
	ID       uint64            `json:"id"`                 // 权限点 ID
	ParentID uint64            `json:"parent_id"`          // 父节点 ID，0 为根
	Code     string            `json:"code"`               // 权限点编码
	Name     string            `json:"name"`               // 显示名称
	Type     string            `json:"type"`               // menu / button
	Path     string            `json:"path,omitempty"`     // 菜单路由
	Icon     string            `json:"icon,omitempty"`     // 图标名
	Children []*PermissionNode `json:"children,omitempty"` // 子节点
}

// RoleItem 是角色列表/详情返回结构。
type RoleItem struct {
	ID            uint64   `json:"id"`             // 角色 ID
	Name          string   `json:"name"`           // 角色名称
	Code          string   `json:"code"`           // 角色编码（owner/staff 为预置）
	IsSystem      bool     `json:"is_system"`      // 是否预置角色（不可删除、编码锁定）
	Remark        string   `json:"remark"`         // 备注
	PermissionIDs []uint64 `json:"permission_ids"` // 已授权权限点 ID 集合
	UserCount     int64    `json:"user_count"`     // 引用该角色的账号数
}

// RoleInput 是新增/编辑角色的入参。
type RoleInput struct {
	Name          string   `json:"name" binding:"required,max=64"` // 角色名称
	Code          string   `json:"code" binding:"required,max=64"` // 角色编码
	Remark        string   `json:"remark" binding:"max=255"`       // 备注
	PermissionIDs []uint64 `json:"permission_ids"`                 // 授权的权限点 ID 集合
}

// —— Service ——

// Service 承载权限点字典读取、租户角色与授权管理，并实现权限解析器与路由注册。
type Service struct {
	manager  *db.Manager    // 数据库管理器
	sessions *session.Store // 会话与权限缓存
	log      *zap.Logger    // 日志
}

// NewService 组装 rbac 服务。
func NewService(manager *db.Manager, sessions *session.Store, log *zap.Logger) *Service {
	return &Service{manager: manager, sessions: sessions, log: log}
}

// ResolvePermissions 实现 middleware.PermissionResolver。
// 主账号（owner 角色）直接返回全部租户级权限点，这样平台后续新增权限点时无需回填历史租户。
func (s *Service) ResolvePermissions(ctx context.Context, tenantID, userID uint64, schema string) ([]string, error) {
	perms := make([]string, 0, 32)
	err := s.manager.TenantTx(ctx, schema, func(tx *gorm.DB) error {
		// 注意：这几条是裸 SQL，GORM 的软删作用域不会自动生效，
		// 必须手写 deleted_at IS NULL，否则被软删的权限点/角色仍会解析生效。
		var ownerCount int64
		if err := tx.Raw(`SELECT count(*) FROM t_user_role ur
			JOIN t_role r ON r.id = ur.role_id AND r.deleted_at IS NULL
			WHERE ur.user_id = ? AND r.code = ?`, userID, model.RoleCodeOwner).Scan(&ownerCount).Error; err != nil {
			return err
		}
		if ownerCount > 0 {
			return tx.Raw(`SELECT code FROM sys_permission WHERE scope = ? AND deleted_at IS NULL`,
				model.UserTypeTenant).Scan(&perms).Error
		}
		return tx.Raw(`SELECT DISTINCT p.code FROM t_user_role ur
			JOIN t_role_permission rp ON rp.role_id = ur.role_id
			JOIN sys_permission p ON p.id = rp.permission_id AND p.deleted_at IS NULL
			WHERE ur.user_id = ?`, userID).Scan(&perms).Error
	})
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return perms, nil
}

// PermissionTree 返回租户侧可见的权限点树。
func (s *Service) PermissionTree(ctx context.Context) ([]*PermissionNode, error) {
	var list []model.Permission
	if err := s.manager.Platform().WithContext(ctx).
		Where("scope = ?", model.UserTypeTenant).
		Order("sort asc, id asc").Find(&list).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return buildTree(list), nil
}

// ListRoles 返回当前租户的角色，并带上授权与引用账号数。
func (s *Service) ListRoles(ctx context.Context) ([]RoleItem, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}

	var roles []model.Role
	if err := tx.Order("id asc").Find(&roles).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if len(roles) == 0 {
		return []RoleItem{}, nil
	}

	roleIDs := make([]uint64, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.ID)
	}

	var bindings []model.RolePermission
	if err := tx.Where("role_id IN ?", roleIDs).Find(&bindings).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	permMap := make(map[uint64][]uint64, len(roles))
	for _, b := range bindings {
		permMap[b.RoleID] = append(permMap[b.RoleID], b.PermissionID)
	}

	var userCounts []struct {
		RoleID uint64
		Total  int64
	}
	if err := tx.Model(&model.UserRole{}).
		Select("role_id, count(*) as total").
		Where("role_id IN ?", roleIDs).
		Group("role_id").Scan(&userCounts).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	countMap := make(map[uint64]int64, len(userCounts))
	for _, c := range userCounts {
		countMap[c.RoleID] = c.Total
	}

	out := make([]RoleItem, 0, len(roles))
	for _, r := range roles {
		perms := permMap[r.ID]
		if perms == nil {
			perms = []uint64{}
		}
		out = append(out, RoleItem{
			ID:            r.ID,
			Name:          r.Name,
			Code:          r.Code,
			IsSystem:      r.IsSystem,
			Remark:        r.Remark,
			PermissionIDs: perms,
			UserCount:     countMap[r.ID],
		})
	}
	return out, nil
}

// CreateRole 新建角色并写入授权关系。
func (s *Service) CreateRole(ctx context.Context, in RoleInput) (uint64, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return 0, err
	}

	var exists int64
	if err := tx.Model(&model.Role{}).Where("code = ?", in.Code).Count(&exists).Error; err != nil {
		return 0, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if exists > 0 {
		return 0, errcode.New(errcode.ErrRoleCodeTaken)
	}

	role := model.Role{Name: in.Name, Code: in.Code, Remark: in.Remark}
	if err := tx.Create(&role).Error; err != nil {
		return 0, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if err := s.replacePermissions(tx, role.ID, in.PermissionIDs); err != nil {
		return 0, err
	}
	s.invalidate(ctx)
	return role.ID, nil
}

// UpdateRole 更新角色信息与授权。
func (s *Service) UpdateRole(ctx context.Context, id uint64, in RoleInput) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}

	var role model.Role
	if err := tx.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.ErrNotFound)
		}
		return errcode.Wrap(errcode.ErrDatabase, err)
	}

	// 预置角色只允许改名称/备注，不允许改编码，避免破坏 owner 判定逻辑
	if role.IsSystem && in.Code != role.Code {
		return errcode.New(errcode.ErrSystemRoleLocked)
	}

	if !role.IsSystem {
		var dup int64
		if err := tx.Model(&model.Role{}).Where("code = ? AND id <> ?", in.Code, id).Count(&dup).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if dup > 0 {
			return errcode.New(errcode.ErrRoleCodeTaken)
		}
		role.Code = in.Code
	}

	role.Name = in.Name
	role.Remark = in.Remark
	if err := tx.Save(&role).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if err := s.replacePermissions(tx, role.ID, in.PermissionIDs); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// DeleteRole 删除角色，预置角色与仍被引用的角色不允许删除。
func (s *Service) DeleteRole(ctx context.Context, id uint64) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}

	var role model.Role
	if err := tx.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.ErrNotFound)
		}
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if role.IsSystem {
		return errcode.New(errcode.ErrSystemRoleLocked)
	}

	var used int64
	if err := tx.Model(&model.UserRole{}).Where("role_id = ?", id).Count(&used).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if used > 0 {
		return errcode.New(errcode.ErrRoleInUse)
	}

	if err := tx.Where("role_id = ?", id).Delete(&model.RolePermission{}).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if err := tx.Delete(&model.Role{}, id).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	s.invalidate(ctx)
	return nil
}

// SeedTenantRoles 为新开通的租户预置角色：主账号（全量）与店员（基础业务）。
// 返回角色 ID 供后续绑定账号使用。
func (s *Service) SeedTenantRoles(ctx context.Context, tx *gorm.DB) (map[string]uint64, error) {
	var perms []model.Permission
	if err := s.manager.Platform().WithContext(ctx).
		Where("scope = ?", model.UserTypeTenant).Find(&perms).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	allIDs := make([]uint64, 0, len(perms))
	staffIDs := make([]uint64, 0, len(perms))
	for _, p := range perms {
		allIDs = append(allIDs, p.ID)
		if defaultStaffCodes[p.Code] {
			staffIDs = append(staffIDs, p.ID)
		}
	}

	result := make(map[string]uint64, 2)
	presets := []struct {
		code  string
		name  string
		perm  []uint64
		order int
	}{
		{model.RoleCodeOwner, model.RoleNameOwner, allIDs, 0},
		{model.RoleCodeStaff, model.RoleNameStaff, staffIDs, 1},
	}
	for _, p := range presets {
		role := model.Role{Name: p.name, Code: p.code, IsSystem: true}
		if err := tx.Create(&role).Error; err != nil {
			return nil, errcode.Wrap(errcode.ErrDatabase, err)
		}
		if err := s.replacePermissions(tx, role.ID, p.perm); err != nil {
			return nil, err
		}
		result[p.code] = role.ID
	}
	return result, nil
}

// replacePermissions 全量重写角色的授权关系：先清空再写入，去重处理重复权限点。
// 调用方必须传入事务句柄，保证与角色主表写入同事务原子生效。
func (s *Service) replacePermissions(tx *gorm.DB, roleID uint64, permIDs []uint64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if len(permIDs) == 0 {
		return nil
	}
	rows := make([]model.RolePermission, 0, len(permIDs))
	seen := make(map[uint64]struct{}, len(permIDs))
	for _, pid := range permIDs {
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		rows = append(rows, model.RolePermission{RoleID: roleID, PermissionID: pid})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	return nil
}

// invalidate 让该租户下所有账号的权限缓存失效，保证授权变更立即生效。
func (s *Service) invalidate(ctx context.Context) {
	if err := s.sessions.InvalidateTenantPermissions(ctx, ctxkey.TenantID(ctx)); err != nil {
		s.log.Warn("失效权限缓存失败", zap.Error(err))
	}
}

// defaultStaffCodes 是店员角色的默认权限点，可在后台调整。
var defaultStaffCodes = map[string]bool{
	"tenant.dashboard":    true,
	"tenant.user.list":    true,
	"tenant.role.list":    true,
	"tenant.profile.view": true,
}

// buildTree 把扁平权限点列表按 ParentID 组装成授权勾选树。
func buildTree(list []model.Permission) []*PermissionNode {
	nodes := make(map[uint64]*PermissionNode, len(list))
	for _, p := range list {
		nodes[p.ID] = &PermissionNode{
			ID: p.ID, ParentID: p.ParentID, Code: p.Code, Name: p.Name,
			Type: p.Type, Path: p.Path, Icon: p.Icon,
		}
	}
	roots := make([]*PermissionNode, 0, len(list))
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
