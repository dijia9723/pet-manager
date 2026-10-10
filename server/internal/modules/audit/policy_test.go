package audit

import (
	"net/http"
	"reflect"
	"testing"
)

// TestDefaultPolicy 校验默认策略：写方法全部记录、读方法不记录。
func TestDefaultPolicy(t *testing.T) {
	p := defaultPolicy()
	for _, m := range defaultWriteMethods {
		if !p.allows(m, "/api/v1/platform/tenants") {
			t.Errorf("默认策略应记录写方法 %s", m)
		}
	}
	if p.allows(http.MethodGet, "/api/v1/platform/tenants") {
		t.Error("默认策略不应记录 GET")
	}
}

// TestPolicyDisabled 校验总开关关闭后不记录任何请求。
func TestPolicyDisabled(t *testing.T) {
	p := mustBuildPolicy(false, defaultWriteMethods, nil)
	if p.allows(http.MethodPost, "/api/v1/platform/tenants") {
		t.Error("总开关关闭后不应记录任何请求")
	}
}

// TestPolicyCustomMethods 校验自定义方法白名单可纳入非写方法（如 GET）。
func TestPolicyCustomMethods(t *testing.T) {
	p := mustBuildPolicy(true, []string{http.MethodGet, http.MethodPost}, nil)
	if !p.allows(http.MethodGet, "/x") {
		t.Error("自定义方法包含 GET 时应记录 GET")
	}
	if p.allows(http.MethodDelete, "/x") {
		t.Error("未列入的方法不应记录")
	}
}

// TestPolicyExcludeRoutes 校验排除路由：精确匹配与 * 通配（不匹配空前缀段）。
func TestPolicyExcludeRoutes(t *testing.T) {
	p := mustBuildPolicy(true, defaultWriteMethods, []string{
		"/api/v1/platform/dicts",
		"/api/v1/platform/configs/*",
	})
	cases := []struct {
		method string
		route  string
		want   bool
	}{
		{http.MethodPost, "/api/v1/platform/dicts", false},            // 精确排除
		{http.MethodPost, "/api/v1/platform/configs/10/items", false}, // 通配排除
		{http.MethodPost, "/api/v1/platform/configs", true},           // * 不匹配空前缀段
		{http.MethodPost, "/api/v1/platform/tenants", true},           // 未排除
		{http.MethodDelete, "/api/v1/platform/dicts/items/1", true},   // 精确模式不带通配
	}
	for _, c := range cases {
		if got := p.allows(c.method, c.route); got != c.want {
			t.Errorf("allows(%s,%s)=%v, want %v", c.method, c.route, got, c.want)
		}
	}
}

// TestParseMethods 校验方法列表解析：trim / 大写 / 去重 / 过滤空项。
func TestParseMethods(t *testing.T) {
	got := parseMethods(" post, GET ,post,delete ")
	want := []string{"POST", "GET", "DELETE"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseMethods = %v, want %v", got, want)
	}
	if len(parseMethods(" , ")) != 0 {
		t.Error("全空输入应得到空列表")
	}
}
