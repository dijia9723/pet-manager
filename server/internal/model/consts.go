package model

// 通用状态值，sys_* 与 t_* 的 status 列共用。
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	// StatusDeleted 仅用于租户主表：标记"待销毁"，行仍存活、进入宽限期，
	// 到期由系统定时任务物理回收 schema 与平台侧数据；宽限期内可恢复。
	StatusDeleted = "deleted"
)

// 用户类型：区分平台管理员与租户账号，同时作为权限点的 scope 取值。
const (
	UserTypePlatform = "platform"
	UserTypeTenant   = "tenant"
)

// 权限点类型：menu 进菜单树，button 只做按钮级控制。
const (
	PermTypeMenu   = "menu"
	PermTypeButton = "button"
)

// 参数配置的取值类型，供前端表单控件选择。
const (
	ConfigTypeString = "string"
	ConfigTypeNumber = "number"
	ConfigTypeBool   = "bool"
	ConfigTypeJSON   = "json"
)

// 登录日志结果。与审计日志的 HTTP 码 int 口径不同，属业务语义值。
const (
	LoginStatusSuccess = "success"
	LoginStatusFail    = "fail"
)
