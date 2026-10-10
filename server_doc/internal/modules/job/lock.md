# 执行锁

> [开发者入口](../../../../README.md) · [任务管理](service.md) · [执行器](executor.md)

## 职责

为任务执行提供 Redis 互斥锁，保证同一任务的多次执行、以及多个任务调用同一系统方法时不会重入。

## 关键函数

`acquireRunLocks(ctx, code, identity) (release func(), err error)`

## 设计要点

- **锁键**：先锁不可修改的任务编码 `job:编码`；system 类型再叠加方法锁（`identity` 即方法名），兼容原有预置任务锁键，避免同一方法跨任务重入。
- **所有权校验释放**：每把锁的值是本次随机 token，释放时用 Lua CAS 比对后再删，防止误删已过期后被他人重建的同名锁。
- **全得或全弃**：任一把锁抢占失败即释放已获得的锁，并返回冲突错误（`errcode.ErrConflict`：“任务或同一系统方法正在执行中”）。

## 约定与坑

- 锁键统一经 `cache.Key("lock","job","run", value)` 生成，带全局 key 前缀。
- `runLockTTL` 为锁的过期时间；释放使用独立的 3 秒超时上下文，避免释放动作被调用方已取消的 ctx 拖死。
- 释放函数可安全重复调用（幂等）。
