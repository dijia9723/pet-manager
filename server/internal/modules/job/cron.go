// cron.go 是内置的 5 段 cron 解析器（分 时 日 月 周）。
//
// 不引入 robfig/cron 等第三方依赖：底座只需要"分钟级周期调度 + 可校验的表达式"，
// 自实现约一百行即可覆盖常用写法（* / */n / a-b / a,b,c 及其组合），便于审计与离线部署。
//
// 语义与标准 cron 对齐：
//   - 周字段 0 与 7 都表示周日；
//   - 日与周同时被限制（都不是 *）时，二者满足其一即触发（标准 cron 的 OR 语义）；
//   - 统一按进程本地时区计算（与 GORM NowFunc 的 time.Local 口径一致）。
package job

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cronFieldBounds 依次为 分 / 时 / 日 / 月 / 周 的取值上下界（周允许 0-7，0 与 7 同为周日）。
var cronFieldBounds = [5][2]int{
	{0, 59},
	{0, 23},
	{1, 31},
	{1, 12},
	{0, 7},
}

// cronSchedule 是解析后的分位匹配表：各时间位用定长布尔数组表示是否命中，
// domStar/dowStar 记录"日/周字段是否为 *"，用于 matches 还原标准 cron 的 OR 语义。
type cronSchedule struct {
	minute  [60]bool
	hour    [24]bool
	dom     [32]bool // 下标 1..31
	month   [13]bool // 下标 1..12
	dow     [7]bool  // 下标 0..6，0=周日
	domStar bool     // 日字段为 *（未显式限制）
	dowStar bool     // 周字段为 *（未显式限制）
}

// parseCron 解析 5 段 cron 表达式，非法写法返回带定位信息的错误。
func parseCron(expr string) (*cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron 表达式必须为 5 段（分 时 日 月 周），当前 %d 段", len(fields))
	}

	var s cronSchedule
	setters := []func(int) bool{
		func(v int) bool { s.minute[v] = true; return true },
		func(v int) bool { s.hour[v] = true; return true },
		func(v int) bool { s.dom[v] = true; return true },
		func(v int) bool { s.month[v] = true; return true },
		func(v int) bool { s.dow[v%7] = true; return true }, // 7 归一化为 0（周日）
	}

	for i, field := range fields {
		if field == "*" {
			if i == 2 {
				s.domStar = true
			}
			if i == 4 {
				s.dowStar = true
			}
			fillRange(setters[i], cronFieldBounds[i][0], cronFieldBounds[i][1])
			continue
		}
		if err := parseCronField(field, cronFieldBounds[i][0], cronFieldBounds[i][1], setters[i]); err != nil {
			return nil, fmt.Errorf("cron 第 %d 段 %q 非法: %w", i+1, field, err)
		}
	}
	return &s, nil
}

// fillRange 把闭区间 [lo,hi] 的全部取值标记为命中，用于展开 "*"。
func fillRange(set func(int) bool, lo, hi int) {
	for v := lo; v <= hi; v++ {
		set(v)
	}
}

// parseCronField 处理单个字段：a,b,c 与每段的 a-b / */n / a-b/n / a/n 写法。
func parseCronField(field string, lo, hi int, set func(int) bool) error {
	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return fmt.Errorf("空的取值项")
		}
		rangePart, stepPart, hasStep := strings.Cut(part, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepPart)
			if err != nil || n <= 0 {
				return fmt.Errorf("步长 %q 非法", stepPart)
			}
			step = n
		}

		from, to := lo, hi
		switch {
		case rangePart == "*":
			// 用默认全区间
		case strings.Contains(rangePart, "-"):
			loStr, hiStr, ok := strings.Cut(rangePart, "-")
			if !ok {
				return fmt.Errorf("区间 %q 非法", rangePart)
			}
			a, err := strconv.Atoi(loStr)
			if err != nil {
				return fmt.Errorf("起点 %q 非法", loStr)
			}
			b, err := strconv.Atoi(hiStr)
			if err != nil {
				return fmt.Errorf("终点 %q 非法", hiStr)
			}
			from, to = a, b
		default:
			v, err := strconv.Atoi(rangePart)
			if err != nil {
				return fmt.Errorf("取值 %q 非法", rangePart)
			}
			// 单值带步长（如 5/10）按"从该值到上界"展开，与常见 cron 实现一致
			if hasStep {
				from = v
			} else {
				from, to = v, v
			}
		}

		if from < lo || to > hi || from > to {
			return fmt.Errorf("取值超出区间 [%d,%d]", lo, hi)
		}
		for v := from; v <= to; v += step {
			set(v)
		}
	}
	return nil
}

// next 返回 after 之后第一个匹配的整分钟时刻（秒/纳秒清零）；
// 表达式本身合法但 5 年内都无法命中（如 2 月 31 日）时返回零值。
func (s *cronSchedule) next(after time.Time) time.Time {
	t := after.Truncate(time.Minute).Add(time.Minute)
	// 上限 5 年，覆盖闰年周期；正常表达式当轮即可命中
	limit := after.AddDate(5, 0, 0)
	for !t.After(limit) {
		if s.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}

// matches 判断时刻 t 是否命中本表达式；日/周同时被限制时按标准 cron 取 OR。
func (s *cronSchedule) matches(t time.Time) bool {
	if !s.minute[t.Minute()] || !s.hour[t.Hour()] || !s.month[int(t.Month())] {
		return false
	}
	dayOK := s.dom[t.Day()]
	weekOK := s.dow[int(t.Weekday())]
	switch {
	case s.domStar && s.dowStar:
		return true
	case s.domStar:
		return weekOK
	case s.dowStar:
		return dayOK
	default:
		// 日与周都被显式限制时，标准 cron 取 OR
		return dayOK || weekOK
	}
}
