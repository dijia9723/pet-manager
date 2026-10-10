package middleware

import "testing"

// HasAny 是权限校验的唯一判定函数，403 与否全看它。
func TestHasAny(t *testing.T) {
	cases := []struct {
		name     string
		owned    []string
		required []string
		want     bool
	}{
		{"未声明要求时一律放行", nil, nil, true},
		{"未声明要求时即使无权限也放行", []string{}, []string{}, true},
		{"命中单个权限点", []string{"tenant.user.list"}, []string{"tenant.user.list"}, true},
		{"未命中", []string{"tenant.user.list"}, []string{"tenant.user.create"}, false},
		{"多要求满足其一即可", []string{"tenant.role.list"}, []string{"tenant.role.create", "tenant.role.list"}, true},
		{"多要求全不满足", []string{"tenant.role.list"}, []string{"tenant.role.create", "tenant.role.update"}, false},
		{"平台管理员通配", []string{PermissionWildcard}, []string{"tenant.user.create"}, true},
		{"通配对任意要求都成立", []string{PermissionWildcard}, []string{"platform.tenant.list", "tenant.role.delete"}, true},
		{"无任何权限", nil, []string{"tenant.user.list"}, false},
	}

	for _, c := range cases {
		if got := HasAny(c.owned, c.required); got != c.want {
			t.Errorf("%s: HasAny(%v, %v) = %v, 期望 %v", c.name, c.owned, c.required, got, c.want)
		}
	}
}
