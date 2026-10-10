package db

import "testing"

// SchemaForTenant 是全项目 schema 命名的唯一来源，这里锁定其拼接规则，
// 避免各处自行拼串导致 tenant_<id> 形态被破坏。
func TestSchemaForTenant(t *testing.T) {
	cases := []struct {
		id   uint64
		want string
	}{
		{1, "tenant_1"},
		{42, "tenant_42"},
		{9007199254740993, "tenant_9007199254740993"},
	}
	for _, c := range cases {
		if got := SchemaForTenant(c.id); got != c.want {
			t.Errorf("SchemaForTenant(%d) = %q, 期望 %q", c.id, got, c.want)
		}
	}
}

// ValidateSchema 是租户 schema 名的唯一安全闸门：
// schema 名会被拼进 DDL 与 search_path，所以只允许 tenant_<数字> 这一种形态。
func TestValidateSchema(t *testing.T) {
	valid := []string{"tenant_1", "tenant_42", "tenant_9007199254740993"}
	for _, s := range valid {
		if err := ValidateSchema(s); err != nil {
			t.Errorf("ValidateSchema(%q) 期望通过，实际报错 %v", s, err)
		}
	}

	invalid := []string{
		"",
		"public",
		"tenant_",
		"tenant_abc",
		"tenant_1; DROP SCHEMA public CASCADE",
		"tenant_1, public",
		"tenant_1'",
		`tenant_1"`,
		"tenant_-1",
		"tenant_1 ",
		"TENANT_1",
		"tenant_1_2",
	}
	for _, s := range invalid {
		if err := ValidateSchema(s); err == nil {
			t.Errorf("ValidateSchema(%q) 期望被拒绝，实际通过", s)
		}
	}
}
