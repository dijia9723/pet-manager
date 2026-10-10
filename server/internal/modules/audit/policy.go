// 审计采集策略：决定"哪些请求要落审计日志"。
//
// 策略不写死在代码 / 配置文件里，而是落在平台参数表 sys_config（参数配置页可直接治理，
// 无需发版），audit Service 以 TTL 缓存原子替换，读取零 DB 开销。
// 约定的参数键（group_key = auditGroup，状态需 enabled，缺失 / 停用 / 非法值均回退默认）：
//
//	audit.enabled         bool   审计总开关，默认 true
//	audit.methods         string 纳入审计的 HTTP 方法，逗号分隔，默认 POST,PUT,PATCH,DELETE
//	audit.exclude_routes  json   排除的路由模板数组，支持 * 通配，默认 []
//
// 排除规则优先匹配 gin 路由模板（c.FullPath，形如 /api/v1/platform/tenants/:id），
// 路由未命中（404）时回退实际路径。默认策略保持"登录后写操作全量记录"的历史行为。
package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
)

const (
	// auditGroup 是审计策略参数在 sys_config 中的分组键。
	auditGroup = "audit"

	keyEnabled       = "audit.enabled"        // 审计总开关（bool）
	keyMethods       = "audit.methods"        // 纳入审计的 HTTP 方法白名单（逗号分隔）
	keyExcludeRoutes = "audit.exclude_routes" // 排除的路由模板（JSON 数组，支持 * 通配）

	// policyTTL 是策略缓存有效期：参数页改动最迟此时长后被各实例感知，无需主动失效。
	policyTTL = 60 * time.Second
	// policyLoadTimeout 是单次策略加载超时，不能让配置库抖动拖住审计 worker。
	policyLoadTimeout = 3 * time.Second
)

// defaultWriteMethods 是默认纳入审计的写方法白名单，读请求默认不记录。
var defaultWriteMethods = []string{
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
}

// policy 是不可变的审计采集策略快照，整体原子替换、不做局部修改。
type policy struct {
	enabled bool
	// methods 是纳入采集的 HTTP 方法白名单（大写），未命中直接跳过。
	methods map[string]struct{}
	// excludes 已按通配编译为锚定正则（* -> .*）。
	excludes []*regexp.Regexp
}

// defaultPolicy 保持中间件早期硬编码的行为：登录后的写操作全部记录。
func defaultPolicy() *policy {
	return mustBuildPolicy(true, defaultWriteMethods, nil)
}

// allows 判定指定请求方法 + 路由是否记录。
func (p *policy) allows(method, route string) bool {
	if !p.enabled {
		return false
	}
	if _, ok := p.methods[strings.ToUpper(method)]; !ok {
		return false
	}
	for _, re := range p.excludes {
		if re.MatchString(route) {
			return false
		}
	}
	return true
}

// loadPolicy 从 sys_config 读取启用状态的 audit 分组参数并构建策略。
// 任何键缺失 / 停用 / 取值非法都回退默认值并告警，绝不因配置问题关掉审计或阻断启动。
func loadPolicy(ctx context.Context, manager *db.Manager, log *zap.Logger) *policy {
	loadCtx, cancel := context.WithTimeout(ctx, policyLoadTimeout)
	defer cancel()

	var rows []model.Config
	if err := manager.Platform().WithContext(loadCtx).
		Where("group_key = ? AND status = ?", auditGroup, model.StatusEnabled).
		Find(&rows).Error; err != nil {
		log.Warn("加载审计策略失败，沿用当前策略", zap.Error(err))
		return nil
	}

	values := make(map[string]string, len(rows))
	for _, r := range rows {
		values[r.ConfigKey] = r.Value
	}

	enabled := true
	if raw, ok := values[keyEnabled]; ok {
		v, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			log.Warn("审计参数 audit.enabled 取值非法，按默认 true 处理", zap.String("value", raw))
		} else {
			enabled = v
		}
	}

	methods := defaultWriteMethods
	if raw, ok := values[keyMethods]; ok {
		if parsed := parseMethods(raw); len(parsed) > 0 {
			methods = parsed
		} else {
			log.Warn("审计参数 audit.methods 为空或非法，回退默认写方法", zap.String("value", raw))
		}
	}

	var patterns []string
	if raw, ok := values[keyExcludeRoutes]; ok && strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &patterns); err != nil {
			log.Warn("审计参数 audit.exclude_routes 不是合法 JSON 数组，忽略排除配置", zap.Error(err))
			patterns = nil
		}
	}

	return mustBuildPolicy(enabled, methods, patterns)
}

// parseMethods 解析逗号分隔的 HTTP 方法列表，自动 trim / 大写 / 去重。
func parseMethods(raw string) []string {
	seen := make(map[string]struct{})
	var methods []string
	for _, m := range strings.Split(raw, ",") {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		methods = append(methods, m)
	}
	return methods
}

// mustBuildPolicy 编译策略；通配模式只含 * 与已转义字面量，理论上不会编译失败，
// 失败的个别模式直接忽略，不影响其余规则。
func mustBuildPolicy(enabled bool, methods []string, excludePatterns []string) *policy {
	m := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		m[method] = struct{}{}
	}

	var excludes []*regexp.Regexp
	for _, pattern := range excludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		// 先整体转义，再把通配符还原为正则：* 跨任意字符（含 /）。
		escaped := regexp.QuoteMeta(pattern)
		escaped = strings.ReplaceAll(escaped, `\*`, ".*")
		re, err := regexp.Compile("^" + escaped + "$")
		if err != nil {
			continue
		}
		excludes = append(excludes, re)
	}

	return &policy{enabled: enabled, methods: m, excludes: excludes}
}
