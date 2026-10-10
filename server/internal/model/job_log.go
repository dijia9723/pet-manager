package model

import "time"

// JobLog 定时任务执行日志，落平台公共库（sys_job_log）。
//
// 每次调度触发或手动执行都会先落一条 running 记录，结束后回写结果与耗时，
// 平台侧"任务管理"页分页只读查询，不提供修改/删除接口。
type JobLog struct {
	BaseModel
	JobID   uint64 `gorm:"not null;index" json:"job_id"`     // 关联任务 ID（sys_job.id）
	JobCode string `gorm:"size:64;not null" json:"job_code"` // 任务编码快照，任务删除后日志仍可读
	// Trigger 触发方式：schedule 调度触发 / manual 手动执行。
	Trigger string `gorm:"size:16;not null" json:"trigger"`
	// Status 执行状态：running / success / fail。
	Status string `gorm:"size:16;not null;index" json:"status"`

	StartedAt  time.Time  `gorm:"not null;index" json:"started_at"` // 开始时间
	FinishedAt *time.Time `json:"finished_at"`                      // 结束时间，运行中为空
	CostMs     int64      `json:"cost_ms"`                          // 耗时（毫秒）
	// Result 成功时为执行摘要，失败时为错误信息（可能较长，用 text）。
	Result string `gorm:"type:text" json:"result"`
}

// TableName 固定表名；任务执行日志落平台库。
func (JobLog) TableName() string { return "sys_job_log" }
