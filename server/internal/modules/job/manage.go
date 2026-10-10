package job

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
)

// CreateInput 是新建任务的入参，Code/Kind/Spec 为核心字段。
type CreateInput struct {
	Code           string          `json:"code" binding:"required,max=64"`
	Name           string          `json:"name" binding:"required,max=128"`
	Kind           string          `json:"kind" binding:"required"`
	Spec           string          `json:"spec" binding:"required,max=64"`
	Status         string          `json:"status"`
	Remark         string          `json:"remark" binding:"max=255"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	Config         ExecutionConfig `json:"config"`
}

// definition 将入参校验并归一化为可落库的任务行：默认停用、默认超时 60s，
// 并预先计算启用状态下的下次执行时间（cron 无有效命中则报错）。
func (s *Service) definition(in CreateInput) (model.Job, error) {
	in.Code, in.Name, in.Spec, in.Remark = strings.TrimSpace(in.Code), strings.TrimSpace(in.Name), strings.TrimSpace(in.Spec), strings.TrimSpace(in.Remark)
	if !jobCodePattern.MatchString(in.Code) {
		return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "任务编码须以字母开头，包含字母、数字、下划线、点或连字符，最多 64 位")
	}
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 128 || utf8.RuneCountInString(in.Remark) > 255 {
		return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "任务名称不能为空或字段超出长度限制")
	}
	if len(in.Spec) > 64 {
		return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "Cron 表达式不能超过 64 位")
	}
	if _, err := parseCron(in.Spec); err != nil {
		return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "%v", err)
	}
	if in.Status == "" {
		in.Status = model.StatusDisabled
	}
	if in.Status != model.StatusEnabled && in.Status != model.StatusDisabled {
		return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "状态必须为 enabled 或 disabled")
	}
	if in.TimeoutSeconds == 0 {
		in.TimeoutSeconds = 60
	}
	if in.TimeoutSeconds < 1 || in.TimeoutSeconds > int(runTimeout/time.Second) {
		return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "执行超时需为 1–600 秒")
	}
	config, err := s.normalizeConfig(in.Kind, in.Config)
	if err != nil {
		return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "%v", err)
	}
	row := model.Job{Code: in.Code, Name: in.Name, Kind: in.Kind, Spec: in.Spec, Status: in.Status, Remark: in.Remark, Config: encodeConfig(config), TimeoutSeconds: in.TimeoutSeconds, Builtin: false}
	if row.Status == model.StatusEnabled {
		row.NextRunAt = s.nextRunAt(row.Spec, time.Now())
		if row.NextRunAt == nil {
			return model.Job{}, errcode.Newf(errcode.ErrInvalidParam, "Cron 表达式没有有效的下次执行时间")
		}
	}
	return row, nil
}

// Create 新建自定义任务；占用已被内置方法保留的编码时直接拒绝。
func (s *Service) Create(ctx context.Context, in CreateInput) (*Item, error) {
	row, err := s.definition(in)
	if err != nil {
		return nil, err
	}
	// 内置方法的编码保留给代码同步，避免新建任务抢占尚未补齐的预置行。
	s.mu.Lock()
	_, reserved := s.builtinFn[row.Code]
	s.mu.Unlock()
	if reserved {
		return nil, errcode.Newf(errcode.ErrConflict, "该编码由系统方法保留，请使用其他任务编码")
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&row).Error; err != nil {
		return nil, writeError(err)
	}
	item := toItem(row)
	return &item, nil
}

// updateDefinition 在行锁保护下修改任务定义：预置任务仅允许改调度、超时与说明，
// 改后重新走 definition 校验并同步重算下次执行时间。
func (s *Service) updateDefinition(ctx context.Context, id uint64, in UpdateInput) error {
	err := s.manager.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return jobLookupError(err)
		}
		if row.Builtin && (in.Name != nil || in.Kind != nil || in.Config != nil) {
			return errcode.Newf(errcode.ErrForbiddenAction, "预置任务只能修改调度、超时和说明")
		}
		name, kind, config, timeout := row.Name, row.Kind, decodeConfig(row), row.TimeoutSeconds
		if in.Name != nil {
			name = *in.Name
		}
		if in.Kind != nil {
			kind = *in.Kind
		}
		if in.Config != nil {
			config = *in.Config
		}
		if in.TimeoutSeconds != nil {
			timeout = *in.TimeoutSeconds
		}
		if in.TimeoutSeconds != nil && timeout == 0 {
			return errcode.Newf(errcode.ErrInvalidParam, "执行超时需为 1–600 秒")
		}
		validated, err := s.definition(CreateInput{Code: row.Code, Name: name, Kind: kind, Spec: in.Spec, Status: row.Status, Remark: in.Remark, Config: config, TimeoutSeconds: timeout})
		if err != nil {
			return err
		}
		return tx.Model(&model.Job{}).Where("id = ?", id).Updates(map[string]any{
			"name": validated.Name, "kind": validated.Kind, "spec": validated.Spec,
			"remark": validated.Remark, "config": validated.Config, "timeout_seconds": validated.TimeoutSeconds,
			"next_run_at": validated.NextRunAt,
		}).Error
	})
	return writeError(err)
}

// writeError 归一化数据库错误：业务错误原样返回，唯一键冲突转为"编码已存在"，其余包装为库错误。
func writeError(err error) error {
	if err == nil {
		return nil
	}
	var business *errcode.Error
	if errors.As(err, &business) {
		return err
	}
	var postgres *pgconn.PgError
	if errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &postgres) && postgres.Code == "23505") {
		return errcode.Newf(errcode.ErrConflict, "任务编码已存在")
	}
	return errcode.Wrap(errcode.ErrDatabase, err)
}

// Delete 删除自定义任务：要求已停用且不在执行中，并在事务内二次校验状态与执行身份，
// 避免删除与并发执行/改配置竞态。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	var row model.Job
	p := s.manager.Platform().WithContext(ctx)
	if err := p.First(&row, id).Error; err != nil {
		return jobLookupError(err)
	}
	if row.Builtin {
		return errcode.Newf(errcode.ErrForbiddenAction, "系统预置任务不能删除")
	}
	if row.Status != model.StatusDisabled {
		return errcode.Newf(errcode.ErrConflict, "请先停用任务再删除")
	}
	_, identity, taskErr := s.task(row)
	if taskErr != nil {
		identity = "job:" + row.Code
	}
	s.mu.Lock()
	if _, busy := s.running[id]; busy {
		s.mu.Unlock()
		return errcode.Newf(errcode.ErrConflict, "执行中的任务不能删除")
	}
	s.running[id] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.running, id); s.mu.Unlock() }()
	release, err := s.acquireRunLocks(ctx, row.Code, identity)
	if err != nil {
		return err
	}
	defer release()
	err = p.Transaction(func(tx *gorm.DB) error {
		var current model.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, id).Error; err != nil {
			return jobLookupError(err)
		}
		if current.Builtin || current.Status != model.StatusDisabled {
			return errcode.Newf(errcode.ErrConflict, "任务状态已变更，请重试")
		}
		_, currentIdentity, taskErr := s.task(current)
		if taskErr != nil {
			currentIdentity = "job:" + current.Code
		}
		if currentIdentity != identity {
			return errcode.Newf(errcode.ErrConflict, "任务执行方法已变更，请重试")
		}
		return tx.Delete(&current).Error
	})
	return writeError(err)
}
