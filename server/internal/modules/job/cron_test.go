package job

import (
	"testing"
	"time"
)

// mustParse 解析表达式，失败即终止用例，简化后续断言。
func mustParse(t *testing.T, expr string) *cronSchedule {
	t.Helper()
	s, err := parseCron(expr)
	if err != nil {
		t.Fatalf("parseCron(%q) 返回错误: %v", expr, err)
	}
	return s
}

// localDate 按进程本地时区构造整分钟时刻，与调度器口径一致。
func localDate(y int, mon time.Month, d, h, min int) time.Time {
	return time.Date(y, mon, d, h, min, 0, 0, time.Local)
}

// TestParseCronInvalid 覆盖段数、越界、非法步长与非数字等非法表达式。
func TestParseCronInvalid(t *testing.T) {
	bad := []string{"", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *", "*/0 * * * *", "a * * * *", "0-60 * * * *"}
	for _, expr := range bad {
		if _, err := parseCron(expr); err == nil {
			t.Fatalf("parseCron(%q) 应当报错", expr)
		}
	}
}

// TestCronDailyFixedTime 覆盖每日固定时刻表达式。
func TestCronDailyFixedTime(t *testing.T) {
	// 每日 03:30
	s := mustParse(t, "30 3 * * *")
	got := s.next(localDate(2026, 10, 9, 10, 0))
	want := localDate(2026, 10, 10, 3, 30)
	if !got.Equal(want) {
		t.Fatalf("每日 03:30 下次触发 = %v, 期望 %v", got, want)
	}
}

// TestCronEvery15Minutes 覆盖 */n 步长写法。
func TestCronEvery15Minutes(t *testing.T) {
	s := mustParse(t, "*/15 * * * *")
	got := s.next(localDate(2026, 10, 9, 10, 7))
	want := localDate(2026, 10, 9, 10, 15)
	if !got.Equal(want) {
		t.Fatalf("每 15 分钟下次触发 = %v, 期望 %v", got, want)
	}
}

// TestCronWeekdayAndMonth 覆盖周字段匹配（跨周顺延）。
func TestCronWeekdayAndMonth(t *testing.T) {
	// 每周一 09:00：2026-10-09 是周五，下周一是 10-12
	s := mustParse(t, "0 9 * * 1")
	got := s.next(localDate(2026, 10, 9, 8, 0))
	want := localDate(2026, 10, 12, 9, 0)
	if !got.Equal(want) {
		t.Fatalf("每周一 09:00 = %v, 期望 %v（周一）", got, want)
	}
}

// TestCronSundayNormalization 覆盖周字段 7 与 0 均表示周日。
func TestCronSundayNormalization(t *testing.T) {
	// 周字段 7 与 0 都表示周日：2026-10-11 是周日
	s := mustParse(t, "0 0 * * 7")
	got := s.next(localDate(2026, 10, 9, 12, 0))
	want := localDate(2026, 10, 11, 0, 0)
	if !got.Equal(want) {
		t.Fatalf("周日(7) = %v, 期望 %v", got, want)
	}
}

// TestCronMonthDay 覆盖每月固定日期的跨月顺延。
func TestCronMonthDay(t *testing.T) {
	// 每月 1 号 00:00
	s := mustParse(t, "0 0 1 * *")
	got := s.next(localDate(2026, 10, 9, 8, 0))
	want := localDate(2026, 11, 1, 0, 0)
	if !got.Equal(want) {
		t.Fatalf("每月 1 号 = %v, 期望 %v", got, want)
	}
}

// TestCronListAndRange 覆盖 a,b 列表与 a-b 区间、周区间组合。
func TestCronListAndRange(t *testing.T) {
	// 工作日 9-18 点中 9:30 与 18:30... 用 "30 9,18 * * 1-5"
	s := mustParse(t, "30 9,18 * * 1-5")
	// 周五 10:00 之后 → 当天 18:30
	got := s.next(localDate(2026, 10, 9, 10, 0))
	want := localDate(2026, 10, 9, 18, 30)
	if !got.Equal(want) {
		t.Fatalf("工作日 9:30/18:30 = %v, 期望 %v", got, want)
	}
	// 周六 → 下周一 9:30
	got = s.next(localDate(2026, 10, 10, 19, 0))
	want = localDate(2026, 10, 12, 9, 30)
	if !got.Equal(want) {
		t.Fatalf("周末顺延周一 = %v, 期望 %v", got, want)
	}
}

// TestCronStrictlyAfter 覆盖 next 严格晚于入参（命中时刻本身不算）。
func TestCronStrictlyAfter(t *testing.T) {
	// 命中时刻本身不作为下次触发，必须严格晚于入参
	s := mustParse(t, "30 3 * * *")
	got := s.next(localDate(2026, 10, 10, 3, 30))
	want := localDate(2026, 10, 11, 3, 30)
	if !got.Equal(want) {
		t.Fatalf("严格晚于 = %v, 期望 %v", got, want)
	}
}
