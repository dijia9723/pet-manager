package model

import "time"

// 定时任务类型 / 执行结果 / 触发方式取值。
const (
	// JobKindSystem 调用代码注册的系统方法，既可由代码预置也可在页面新建。
	JobKindSystem = "system"
	JobKindHTTP   = "http"
	JobKindShell  = "shell"

	JobRunRunning = "running"
	JobRunSuccess = "success"
	JobRunFail    = "fail"

	JobTriggerSchedule = "schedule"
	JobTriggerManual   = "manual"
)

// Job 定时任务定义，落平台公共库（sys_job）。
//
// Spec 使用标准 5 段 cron（分 时 日 月 周），由内部轻量解析器调度，不引入第三方依赖。
// 内置任务的执行体在代码中注册；行内的 spec / status 允许在"任务管理"页治理，
// 重启时只补缺失行、不覆盖治理结果（与权限点字典同口径）。
type Job struct {
	BaseModel
	Code string `gorm:"size:64;not null" json:"code"`                // 任务编码，代码与调度器按此定位任务
	Name string `gorm:"size:128;not null" json:"name"`               // 展示名
	Kind string `gorm:"size:16;not null;default:system" json:"kind"` // 执行类型，见 JobKind*
	// Spec 5 段 cron 表达式（分 时 日 月 周），如 "30 3 * * *" 表示每日 03:30。
	Spec   string `gorm:"size:64;not null" json:"spec"`
	Status string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
	Remark string `gorm:"size:255" json:"remark"`
	// Builtin 标记代码预置任务，禁止删除及更换执行方法。
	Builtin        bool   `gorm:"not null;default:false" json:"builtin"`
	Config         string `gorm:"type:text;not null;default:'{}'" json:"-"`    // 任务参数 JSON，不对外暴露
	TimeoutSeconds int    `gorm:"not null;default:600" json:"timeout_seconds"` // 单次执行超时（秒）

	LastRunAt   *time.Time `gorm:"index" json:"last_run_at"`     // 最近一次执行开始时间
	LastStatus  string     `gorm:"size:16" json:"last_status"`   // 最近一次执行结果，见 JobRun*
	LastMessage string     `gorm:"size:500" json:"last_message"` // 最近一次执行摘要或错误
	NextRunAt   *time.Time `gorm:"index" json:"next_run_at"`     // 下次调度时间，便于调度器快速取任务
}

// TableName 固定表名；平台定时任务落平台库。
func (Job) TableName() string { return "sys_job" }
