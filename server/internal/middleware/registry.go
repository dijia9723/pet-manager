// Package middleware 承载全站 HTTP 拦截器（中间件）。
//
// 为什么要有 Registry：
// 中间件会随项目推进不断变多（审计、灰度、幂等、指标、埋点、SSE 保活…），
// 如果直接往 gin 上依次 Use，顺序就会被"代码书写顺序"绑架，插入新拦截器很容易踩空。
// 这里改成"按名字注册 + 按序位排序"，新增拦截器只需选一个 order 空档注册即可，
// 不必关心别处代码怎么写、也不必改动已有文件。
package middleware

import (
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
)

// 预置序位。数字越小越靠前执行；同序位按名字字典序稳定排序。
// 新增拦截器时在相邻两档之间取中间值即可（例如 65、125）。
const (
	OrderRecovery        = 10
	OrderRequestID       = 20
	OrderSecurityHeaders = 30
	OrderCORS            = 40
	OrderRateLimit       = 50
	OrderAccessLog       = 60

	// 鉴权链路，顺序敏感，不建议打乱
	OrderAuth          = 100 // 解析凭证
	OrderTenantContext = 110 // 解析并校验租户
	OrderTenantScope   = 120 // 开启租户事务（search_path）
	OrderRBAC          = 130 // 权限点校验

	OrderAudit = 900 // 收尾类
)

// entry 是注册表内部条目：名字用于去重与稳定排序，order 决定执行先后。
type entry struct {
	name    string
	order   int
	handler gin.HandlerFunc
}

// Registry 是一个有序、可插拔的拦截器注册表。
type Registry struct {
	mu      sync.RWMutex
	entries map[string]entry
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]entry)}
}

// Add 注册/覆盖一个拦截器。同名重复注册以最后一次为准，便于测试时替换实现。
func (r *Registry) Add(name string, order int, h gin.HandlerFunc) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = entry{name: name, order: order, handler: h}
	return r
}

// Remove 摘掉一个拦截器（例如灰度期临时关闭某个能力）。
func (r *Registry) Remove(name string) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, name)
	return r
}

// Has 判断某拦截器是否已注册。
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[name]
	return ok
}

// Names 返回当前已注册的拦截器名（按执行顺序），用于启动日志自证。
func (r *Registry) Names() []string {
	sorted := r.sorted()
	out := make([]string, 0, len(sorted))
	for _, e := range sorted {
		out = append(out, e.name)
	}
	return out
}

// Handlers 返回排序后的拦截器链，装配到 gin.Engine。
func (r *Registry) Handlers() []gin.HandlerFunc {
	sorted := r.sorted()
	out := make([]gin.HandlerFunc, 0, len(sorted))
	for _, e := range sorted {
		out = append(out, e.handler)
	}
	return out
}

// sorted 返回按 order 升序、order 相同按名字升序的稳定性排序结果。
func (r *Registry) sorted() []entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]entry, 0, len(r.entries))
	for _, e := range r.entries {
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].order != list[j].order {
			return list[i].order < list[j].order
		}
		return list[i].name < list[j].name
	})
	return list
}
