// Package seed 负责首次启动时的初始化数据。
//
// 两件原则：
//  1. 幂等：可以重复执行，已存在的数据不重建、不覆盖用户改动；
//  2. 权限点字典"只补不覆盖"：新增权限点时改这里的 permDefs，
//     启动只创建缺失的 code；已存在的 code（含被软删的）一律跳过。
//     名称/路径/图标/排序等改动由平台侧"菜单配置"页治理，代码改动不再覆盖线上配置。
package seed

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/security"
)

// Run 执行全部种子数据。
func Run(ctx context.Context, m *db.Manager, cfg config.SeedConfig, log *zap.Logger) error {
	if err := seedPermissions(ctx, m); err != nil {
		return err
	}
	if err := alignMenuStructure(ctx, m); err != nil {
		return err
	}
	if err := seedPlatformAdmin(ctx, m, cfg, log); err != nil {
		return err
	}
	if err := seedAuditConfigs(ctx, m); err != nil {
		return err
	}
	if err := seedTenantConfigs(ctx, m); err != nil {
		return err
	}
	if err := seedStoragePlans(ctx, m); err != nil {
		return err
	}
	return nil
}

// auditConfigDefs 是审计采集策略的默认参数（sys_config，group_key=audit）。
// 只在参数配置页提供治理入口，读取与热更新由 audit 模块的 policy 负责（约 60s 生效）。
var auditConfigDefs = []model.Config{
	{
		ConfigKey: "audit.enabled",
		Name:      "审计日志开关",
		Value:     "true",
		GroupKey:  "audit",
		Type:      "bool",
		Remark:    "审计总开关，取值 true/false；停用后登录后的写操作不再落审计库，改动约 60 秒生效",
		Status:    model.StatusEnabled,
	},
	{
		ConfigKey: "audit.methods",
		Name:      "审计记录的请求方法",
		Value:     "POST,PUT,PATCH,DELETE",
		GroupKey:  "audit",
		Type:      "string",
		Remark:    "纳入审计的 HTTP 方法，逗号分隔；默认仅写操作，需要记录读请求可追加 GET，改动约 60 秒生效",
		Status:    model.StatusEnabled,
	},
	{
		ConfigKey: "audit.exclude_routes",
		Name:      "审计排除路由",
		Value:     "[]",
		GroupKey:  "audit",
		Type:      "json",
		Remark:    `不记录的路由模板 JSON 数组，支持 * 通配，如 ["/api/v1/platform/dicts/*"]；匹配 gin 路由模板，改动约 60 秒生效`,
		Status:    model.StatusEnabled,
	},
}

// seedAuditConfigs 只补不覆盖预置审计策略参数：
// 已存在（含被管理员软删）的键一律跳过，参数页的治理结果不被重启覆盖。
func seedAuditConfigs(ctx context.Context, m *db.Manager) error {
	tx := m.Platform().WithContext(ctx)

	var existing []model.Config
	if err := tx.Unscoped().Where("group_key = ?", "audit").Find(&existing).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	byKey := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		byKey[c.ConfigKey] = struct{}{}
	}

	for _, def := range auditConfigDefs {
		if _, ok := byKey[def.ConfigKey]; ok {
			continue
		}
		c := def
		if err := tx.Create(&c).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
	}
	return nil
}

// tenantConfigDefs 是租户生命周期相关默认参数（sys_config，group_key=tenant）。
var tenantConfigDefs = []model.Config{
	{
		ConfigKey: "tenant.purge_after_days",
		Name:      "租户删除宽限天数",
		Value:     "7",
		GroupKey:  "tenant",
		Type:      model.ConfigTypeNumber,
		Remark:    "平台删除租户后保留数据的天数，到期由系统定时任务「租户数据销毁」物理删除数据库；0 表示下次任务执行即销毁",
		Status:    model.StatusEnabled,
	},
}

// seedTenantConfigs 只补不覆盖预置租户参数，口径同 seedAuditConfigs。
func seedTenantConfigs(ctx context.Context, m *db.Manager) error {
	tx := m.Platform().WithContext(ctx)

	var existing []model.Config
	if err := tx.Unscoped().Where("group_key = ?", "tenant").Find(&existing).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	byKey := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		byKey[c.ConfigKey] = struct{}{}
	}

	for _, def := range tenantConfigDefs {
		if _, ok := byKey[def.ConfigKey]; ok {
			continue
		}
		c := def
		if err := tx.Create(&c).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
	}
	return nil
}

// permDef 是权限点字典的声明式定义。
type permDef struct {
	Code   string
	Name   string
	Type   string
	Scope  string
	Path   string
	Icon   string
	Sort   int
	Parent string // 父权限点编码，空表示根节点
}

// permDefs 是全站权限点清单。
//
// 平台侧管理员在中间件里按通配 "*" 处理，这里仍然登记平台权限点，
// 目的是让"平台菜单"也能从同一张字典表下发（/auth/menus），保持菜单与权限同源。
var permDefs = []permDef{
	{Code: "platform.server", Name: "服务器管理", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/servers", Icon: "server", Sort: 18},
	{Code: "platform.server.list", Name: "查看服务器/测试连接", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.server", Sort: 1},
	{Code: "platform.server.create", Name: "新增服务器", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.server", Sort: 2},
	{Code: "platform.server.update", Name: "编辑服务器", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.server", Sort: 3},
	{Code: "platform.server.delete", Name: "删除服务器", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.server", Sort: 4},
	{Code: "platform.server.exec", Name: "进入服务器 SSH 终端", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.server", Sort: 5},
	{Code: "platform.website", Name: "网站管理", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/websites", Icon: "website", Sort: 19},
	{Code: "platform.website.list", Name: "查看网站", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.website", Sort: 1},
	{Code: "platform.website.create", Name: "分配网站", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.website", Sort: 2},
	{Code: "platform.website.update", Name: "编辑网站", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.website", Sort: 3},
	{Code: "platform.website.provision", Name: "初始化隔离目录", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.website", Sort: 4},
	{Code: "platform.website.deploy", Name: "部署 Directus CMS", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.website", Sort: 5},
	{Code: "platform.website.files", Name: "浏览/下载网站文件", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.website", Sort: 6},
	{Code: "platform.website.write", Name: "维护网站文件", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.website", Sort: 7},
	{Code: "tenant.website", Name: "网站管理", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/tenant/websites", Icon: "website", Sort: 40},
	{Code: "tenant.website.list", Name: "查看本租户网站与 CMS", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.website", Sort: 1},
	{Code: "tenant.website.files", Name: "浏览/下载本租户网站文件", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.website", Sort: 2},
	{Code: "tenant.website.write", Name: "维护本租户网站文件", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.website", Sort: 3},
	{Code: "platform.storage", Name: "存储资源", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/storage", Icon: "storage", Sort: 28},
	{Code: "platform.storage.list", Name: "查看存储资源与配额", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.storage", Sort: 1},
	{Code: "platform.storage.upload", Name: "上传文件", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.storage", Sort: 2},
	{Code: "platform.storage.delete", Name: "删除存储资源", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.storage", Sort: 3},
	{Code: "platform.storage.allocate", Name: "分配套餐与扩容", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.storage", Sort: 4},
	{Code: "tenant.storage", Name: "存储资源", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/tenant/storage", Icon: "storage", Sort: 39},
	{Code: "tenant.storage.list", Name: "查看本租户存储资源", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.storage", Sort: 1},
	{Code: "tenant.storage.upload", Name: "上传文件", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.storage", Sort: 2},
	{Code: "tenant.storage.delete", Name: "删除本租户资源", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.storage", Sort: 3},
	{Code: "platform.notification", Name: "信息通知", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/notifications", Icon: "notification", Sort: 27},
	{Code: "platform.notification.list", Name: "查看通知与回执", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.notification", Sort: 1},
	{Code: "platform.notification.create", Name: "新建/编辑通知草稿", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.notification", Sort: 2},
	{Code: "platform.notification.publish", Name: "发布通知", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.notification", Sort: 3},
	{Code: "platform.notification.revoke", Name: "撤回通知", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Parent: "platform.notification", Sort: 4},
	{Code: "platform.message", Name: "消息中心", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/messages", Icon: "notification", Sort: 28},
	{Code: "tenant.message", Name: "消息中心", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/messages", Icon: "notification", Sort: 30},
	{Code: "tenant.notification.list", Name: "查看本租户发布消息与回执", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.message", Sort: 1},
	{Code: "tenant.notification.create", Name: "新建/编辑本租户消息", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.message", Sort: 2},
	{Code: "tenant.notification.publish", Name: "发布本租户消息", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.message", Sort: 3},
	{Code: "tenant.notification.revoke", Name: "撤回本租户消息", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Parent: "tenant.message", Sort: 4},
	// —— 平台侧菜单 ——
	{Code: "platform.dashboard", Name: "工作台", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/dashboard", Icon: "dashboard", Sort: 1},
	{Code: "platform.tenant", Name: "租户管理", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/tenants", Icon: "shop", Sort: 10},
	{Code: "platform.tenant.list", Name: "查看租户", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 11, Parent: "platform.tenant"},
	{Code: "platform.tenant.create", Name: "新建租户", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 12, Parent: "platform.tenant"},
	{Code: "platform.tenant.update", Name: "编辑/启停租户", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 13, Parent: "platform.tenant"},
	{Code: "platform.tenant.delete", Name: "删除/恢复租户", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 14, Parent: "platform.tenant"},
	{Code: "platform.tenant.destroy", Name: "立即销毁租户", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 15, Parent: "platform.tenant"},

	// 任务管理：系统定时任务（如租户数据销毁）的调度治理与执行日志
	{Code: "platform.job", Name: "任务管理", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/jobs", Icon: "clock", Sort: 20},
	{Code: "platform.job.list", Name: "查看任务", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 21, Parent: "platform.job"},
	{Code: "platform.job.update", Name: "编辑/启停任务", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 22, Parent: "platform.job"},
	{Code: "platform.job.trigger", Name: "立即执行任务", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 23, Parent: "platform.job"},
	{Code: "platform.job.log", Name: "查看执行日志", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 24, Parent: "platform.job"},
	{Code: "platform.job.create", Name: "新建任务", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 25, Parent: "platform.job"},
	{Code: "platform.job.delete", Name: "删除任务", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 26, Parent: "platform.job"},

	// 日志管理：父菜单不带路由，仅作分组；子项为审计日志（历史节点）与登录日志
	{Code: "platform.log", Name: "日志管理", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Icon: "log", Sort: 30},
	{Code: "platform.audit", Name: "审计日志", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/audit", Icon: "safety", Sort: 31, Parent: "platform.log"},
	{Code: "platform.audit.list", Name: "查看审计日志", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 32, Parent: "platform.audit"},
	{Code: "platform.loginlog", Name: "登录日志", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/login-logs", Icon: "log", Sort: 33, Parent: "platform.log"},
	{Code: "platform.loginlog.list", Name: "查看登录日志", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 34, Parent: "platform.loginlog"},

	{Code: "platform.permission", Name: "菜单配置", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/permissions", Icon: "setting", Sort: 40},
	{Code: "platform.permission.list", Name: "查看权限点", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 41, Parent: "platform.permission"},
	{Code: "platform.permission.create", Name: "新建权限点", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 42, Parent: "platform.permission"},
	{Code: "platform.permission.update", Name: "编辑权限点", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 43, Parent: "platform.permission"},
	{Code: "platform.permission.delete", Name: "删除权限点", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 44, Parent: "platform.permission"},

	// 配置管理：父菜单不带路由，仅作分组；子项为参数配置与字典管理
	{Code: "platform.config", Name: "配置管理", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Icon: "config", Sort: 50},
	{Code: "platform.param", Name: "参数配置", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/configs", Icon: "config", Sort: 51, Parent: "platform.config"},
	{Code: "platform.param.list", Name: "查看参数", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 52, Parent: "platform.param"},
	{Code: "platform.param.create", Name: "新建参数", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 53, Parent: "platform.param"},
	{Code: "platform.param.update", Name: "编辑参数", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 54, Parent: "platform.param"},
	{Code: "platform.param.delete", Name: "删除参数", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 55, Parent: "platform.param"},
	{Code: "platform.dict", Name: "字典管理", Type: model.PermTypeMenu, Scope: model.UserTypePlatform, Path: "/platform/dicts", Icon: "appstore", Sort: 56, Parent: "platform.config"},
	{Code: "platform.dict.list", Name: "查看字典", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 57, Parent: "platform.dict"},
	{Code: "platform.dict.create", Name: "新建字典", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 58, Parent: "platform.dict"},
	{Code: "platform.dict.update", Name: "编辑字典", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 59, Parent: "platform.dict"},
	{Code: "platform.dict.delete", Name: "删除字典", Type: model.PermTypeButton, Scope: model.UserTypePlatform, Sort: 60, Parent: "platform.dict"},

	// —— 租户侧菜单 ——
	{Code: "tenant.dashboard", Name: "工作台", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/dashboard", Icon: "dashboard", Sort: 1},
	{Code: "tenant.user", Name: "子账号管理", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/tenant/users", Icon: "team", Sort: 10},
	{Code: "tenant.user.list", Name: "查看子账号", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 11, Parent: "tenant.user"},
	{Code: "tenant.user.create", Name: "新建子账号", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 12, Parent: "tenant.user"},
	{Code: "tenant.user.update", Name: "编辑子账号", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 13, Parent: "tenant.user"},
	{Code: "tenant.user.delete", Name: "删除子账号", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 14, Parent: "tenant.user"},
	{Code: "tenant.user.reset", Name: "重置密码", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 15, Parent: "tenant.user"},

	{Code: "tenant.role", Name: "角色授权", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/tenant/roles", Icon: "safety", Sort: 20},
	{Code: "tenant.role.list", Name: "查看角色", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 21, Parent: "tenant.role"},
	{Code: "tenant.role.create", Name: "新建角色", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 22, Parent: "tenant.role"},
	{Code: "tenant.role.update", Name: "编辑角色/授权", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 23, Parent: "tenant.role"},
	{Code: "tenant.role.delete", Name: "删除角色", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 24, Parent: "tenant.role"},

	// 配置管理（租户侧）：租户自有的参数与字典，数据落租户 schema（t_config/t_dict/t_dict_item），
	// 与平台侧 sys_config/sys_dict 互不相通
	{Code: "tenant.config", Name: "配置管理", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Icon: "config", Sort: 40},
	{Code: "tenant.param", Name: "参数配置", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/tenant/configs", Icon: "config", Sort: 41, Parent: "tenant.config"},
	{Code: "tenant.param.list", Name: "查看参数", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 42, Parent: "tenant.param"},
	{Code: "tenant.param.create", Name: "新建参数", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 43, Parent: "tenant.param"},
	{Code: "tenant.param.update", Name: "编辑参数", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 44, Parent: "tenant.param"},
	{Code: "tenant.param.delete", Name: "删除参数", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 45, Parent: "tenant.param"},
	{Code: "tenant.dict", Name: "字典管理", Type: model.PermTypeMenu, Scope: model.UserTypeTenant, Path: "/tenant/dicts", Icon: "appstore", Sort: 46, Parent: "tenant.config"},
	{Code: "tenant.dict.list", Name: "查看字典", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 47, Parent: "tenant.dict"},
	{Code: "tenant.dict.create", Name: "新建字典", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 48, Parent: "tenant.dict"},
	{Code: "tenant.dict.update", Name: "编辑字典", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 49, Parent: "tenant.dict"},
	{Code: "tenant.dict.delete", Name: "删除字典", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 50, Parent: "tenant.dict"},

	// 店铺信息只有接口没有独立页面，因此只登记按钮级权限点，不出菜单
	{Code: "tenant.profile.view", Name: "查看店铺信息", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 31},
	{Code: "tenant.profile.update", Name: "编辑店铺信息", Type: model.PermTypeButton, Scope: model.UserTypeTenant, Sort: 32},
}

// seedPermissions 按 permDefs 补建缺失的权限点：只补不覆盖，
// 已存在的 code（含被软删的）一律跳过、不复活，避免与"菜单配置"页的治理结果冲突。
func seedPermissions(ctx context.Context, m *db.Manager) error {
	tx := m.Platform().WithContext(ctx)

	// 用 Unscoped 连软删行一起读：已存在的 code 一律跳过、不复活。
	// 软删是管理员在"菜单配置"页的有意删除，重启若重建会与配置页治理冲突，
	// 且父节点可能已被删，重建子节点会得到悬空挂载。
	var existing []model.Permission
	if err := tx.Unscoped().Find(&existing).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	byCode := make(map[string]*model.Permission, len(existing))
	for i := range existing {
		byCode[existing[i].Code] = &existing[i]
	}

	for _, def := range permDefs {
		// 只补不覆盖：code 已存在（无论存活还是软删）就跳过，不改任何字段
		if _, ok := byCode[def.Code]; ok {
			continue
		}

		parentID := uint64(0)
		if def.Parent != "" {
			parent, ok := byCode[def.Parent]
			if !ok {
				// 父节点必须先于子节点声明，这只是书写顺序错误，属于开发期问题
				return errcode.Newf(errcode.ErrInternal, "权限点 %s 的父节点 %s 尚未定义", def.Code, def.Parent)
			}
			parentID = parent.ID
		}

		p := model.Permission{
			ParentID: parentID,
			Code:     def.Code,
			Name:     def.Name,
			Type:     def.Type,
			Scope:    def.Scope,
			Path:     def.Path,
			Icon:     def.Icon,
			Sort:     def.Sort,
		}
		if err := tx.Create(&p).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		byCode[def.Code] = &p
	}
	return nil
}

// menuAlignments 声明需要做一次性结构对齐的节点：把 Child 挂到 Parent 下。
//
// 为什么需要：权限点 seed 采用"只补不覆盖"，新增的父分组菜单无法自动改动
// 数据库里已作为根节点存在的历史节点（如 platform.audit）。这里做一次幂等对齐：
// 仅在 Child 仍为根（parent_id = 0）时改写；管理员若已在"菜单配置"页调整过则尊重现状、不覆盖。
var menuAlignments = []struct{ Child, Parent string }{
	{Child: "platform.audit", Parent: "platform.log"},
}

// alignMenuStructure 执行一次性菜单结构对齐：仅在子节点仍是根节点时改写父级，
// 管理员已在"菜单配置"页调整过的结构不覆盖。
func alignMenuStructure(ctx context.Context, m *db.Manager) error {
	tx := m.Platform().WithContext(ctx)
	for _, a := range menuAlignments {
		var child model.Permission
		if err := tx.Where("code = ?", a.Child).First(&child).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if child.ParentID != 0 {
			continue
		}
		var parent model.Permission
		if err := tx.Where("code = ?", a.Parent).First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if err := tx.Model(&model.Permission{}).Where("id = ?", child.ID).
			Update("parent_id", parent.ID).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
	}
	return nil
}

// seedPlatformAdmin 首次启动创建平台管理员账号；
// 账号已存在时不改动口令，避免把管理员改过的密码重置回默认值。
func seedPlatformAdmin(ctx context.Context, m *db.Manager, cfg config.SeedConfig, log *zap.Logger) error {
	username := cfg.PlatformAdminUsername
	if username == "" {
		username = "admin"
	}
	tx := m.Platform().WithContext(ctx)

	var u model.PlatformUser
	err := tx.Where("username = ?", username).First(&u).Error
	if err == nil {
		// 已存在则不动口令：避免每次重启把管理员改过的密码重置回默认值
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}

	password := cfg.PlatformAdminPassword
	if password == "" {
		return errcode.Newf(errcode.ErrInternal, "未配置平台管理员初始口令")
	}
	hashed, err := security.HashPassword(password)
	if err != nil {
		return errcode.Wrap(errcode.ErrInternal, err)
	}

	admin := model.PlatformUser{
		Username: username,
		Password: hashed,
		Nickname: "平台管理员",
		Status:   model.StatusEnabled,
	}
	if err := tx.Create(&admin).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	log.Warn("已创建平台管理员初始账号，请登录后立即修改口令",
		zap.String("username", username))
	return nil
}
