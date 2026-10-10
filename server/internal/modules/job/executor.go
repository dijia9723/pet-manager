package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"pet-manager/server/internal/model"
)

// maxOutputBytes 限制单次任务输出/请求体/脚本的大小（64 KiB），
// 既防止大输出撑爆日志，也作为 curl、脚本、请求体的统一上限。
const maxOutputBytes = 64 * 1024

// ExecutorOptions 是可选的执行器配置；目前仅暴露 Shell 解释器路径。
type ExecutorOptions struct{ ShellPath string }

// Method 是系统允许在任务页选择的执行方法，不会自动生成定时任务。
type Method struct {
	Code   string   `json:"code"`
	Name   string   `json:"name"`
	Remark string   `json:"remark"`
	Fn     TaskFunc `json:"-" swaggerignore:"true"`
}

// MethodCatalog 是方法目录接口的返回体，ShellAvailable 表示当前主机能否找到解释器。
type MethodCatalog struct {
	Methods        []Method `json:"methods"`
	ShellAvailable bool     `json:"shell_available"`
}

// ExecutionConfig 仅保存当前类型的配置；HTTP 可使用表单或 curl 请求语法。
type ExecutionConfig struct {
	Handler    string            `json:"handler,omitempty"`
	URL        string            `json:"url,omitempty"`
	Method     string            `json:"method,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Curl       string            `json:"curl,omitempty"`
	Script     string            `json:"script,omitempty"`
	ScriptPath string            `json:"script_path,omitempty"`
	Args       []string          `json:"args,omitempty"`
	WorkDir    string            `json:"work_dir,omitempty"`
}

// jobCodePattern 约束任务编码：字母开头，仅含字母数字与 _ . -，长度不超过 64。
var jobCodePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$`)

// RegisterMethod 注册一个可被 /jobs/methods 选择并用于系统任务的执行方法。
// 必须在调度器启动前完成；注册项缺字段、编码重复或启动后注册都会 panic，暴露装配错误。
func (s *Service) RegisterMethod(method Method) {
	if !jobCodePattern.MatchString(method.Code) || strings.TrimSpace(method.Name) == "" || method.Fn == nil {
		panic("任务方法注册不完整")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started.Load() {
		panic("任务方法必须在调度器启动前注册")
	}
	if _, exists := s.builtinFn[method.Code]; exists {
		panic("任务方法编码重复: " + method.Code)
	}
	s.builtinFn[method.Code] = method.Fn
	s.methods[method.Code] = method
}

// Methods 返回方法目录（按编码排序）并探测解释器是否可用，供前端选择执行方法。
func (s *Service) Methods() MethodCatalog {
	s.mu.Lock()
	methods := make([]Method, 0, len(s.methods))
	for _, method := range s.methods {
		methods = append(methods, method)
	}
	s.mu.Unlock()
	sort.Slice(methods, func(i, j int) bool { return methods[i].Code < methods[j].Code })
	_, err := exec.LookPath(s.shellPath)
	return MethodCatalog{Methods: methods, ShellAvailable: err == nil}
}

// encodeConfig 把执行配置序列化为 JSON 存入 sys_job.config。
func encodeConfig(config ExecutionConfig) string {
	encoded, _ := json.Marshal(config)
	return string(encoded)
}

// decodeConfig 从任务行还原执行配置；旧版预置任务的 config 为空时，
// 按约定回退到以任务编码作为系统方法名，兼容历史数据。
func decodeConfig(j model.Job) ExecutionConfig {
	var config ExecutionConfig
	_ = json.Unmarshal([]byte(j.Config), &config)
	if j.Kind == model.JobKindSystem && config.Handler == "" {
		config.Handler = j.Code
	}
	return config
}

// normalizeConfig 按任务类型校验并规范化执行配置：
// system 校验方法已注册，http 可解析 curl 或表单，shell 校验脚本/路径/参数边界。
func (s *Service) normalizeConfig(kind string, config ExecutionConfig) (ExecutionConfig, error) {
	switch kind {
	case model.JobKindSystem:
		handler := strings.TrimSpace(config.Handler)
		s.mu.Lock()
		fn := s.builtinFn[handler]
		s.mu.Unlock()
		if fn == nil {
			return ExecutionConfig{}, errors.New("请选择系统已注册的执行方法")
		}
		return ExecutionConfig{Handler: handler}, nil
	case model.JobKindHTTP:
		if strings.TrimSpace(config.Curl) != "" {
			if len(config.Curl) > maxOutputBytes {
				return ExecutionConfig{}, errors.New("curl 命令不能超过 64 KiB")
			}
			parsed, err := parseCurl(config.Curl)
			if err != nil {
				return ExecutionConfig{}, err
			}
			parsed.Curl = config.Curl
			config = parsed
		}
		config = ExecutionConfig{URL: strings.TrimSpace(config.URL), Method: strings.ToUpper(strings.TrimSpace(config.Method)), Headers: config.Headers, Body: config.Body, Curl: config.Curl}
		if config.Method == "" {
			config.Method = http.MethodGet
		}
		if err := validateHTTP(config); err != nil {
			return ExecutionConfig{}, err
		}
		return config, nil
	case model.JobKindShell:
		config = ExecutionConfig{Script: config.Script, ScriptPath: strings.TrimSpace(config.ScriptPath), Args: config.Args, WorkDir: strings.TrimSpace(config.WorkDir)}
		if (strings.TrimSpace(config.Script) == "") == (config.ScriptPath == "") {
			return ExecutionConfig{}, errors.New("请填写脚本内容或脚本路径，两者只能选一个")
		}
		if len(config.Script) > maxOutputBytes {
			return ExecutionConfig{}, errors.New("脚本内容不能超过 64 KiB")
		}
		if strings.ContainsRune(config.Script, 0) {
			return ExecutionConfig{}, errors.New("脚本内容不能包含空字符")
		}
		if config.ScriptPath != "" && (!filepath.IsAbs(config.ScriptPath) || len(config.ScriptPath) > 4096 || strings.ContainsRune(config.ScriptPath, 0)) {
			return ExecutionConfig{}, errors.New("脚本路径必须是服务器上的绝对路径")
		}
		if config.WorkDir != "" && (!filepath.IsAbs(config.WorkDir) || len(config.WorkDir) > 4096 || strings.ContainsRune(config.WorkDir, 0)) {
			return ExecutionConfig{}, errors.New("工作目录必须是服务器上的绝对路径")
		}
		if len(config.Args) > 64 {
			return ExecutionConfig{}, errors.New("脚本参数不能超过 64 项")
		}
		for _, arg := range config.Args {
			if strings.ContainsRune(arg, 0) || len(arg) > 4096 {
				return ExecutionConfig{}, errors.New("脚本参数包含空字符或超出长度限制")
			}
		}
		if _, err := exec.LookPath(s.shellPath); err != nil {
			return ExecutionConfig{}, errors.New("服务器未找到 sh，请配置 jobs.shell_path")
		}
		return config, nil
	default:
		return ExecutionConfig{}, errors.New("任务类型必须是 system、http 或 shell")
	}
}

// validateHTTP 校验 HTTP 目标：仅允许 http/https、拒绝 URL 内嵌账号、白名单方法与请求头字段。
func validateHTTP(config ExecutionConfig) error {
	u, err := url.Parse(config.URL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || len(config.URL) > 8192 {
		return errors.New("请求地址必须是有效的 HTTP/HTTPS URL")
	}
	if u.User != nil {
		return errors.New("请在请求头配置认证，不要在 URL 中填写用户名密码")
	}
	switch config.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
	default:
		return errors.New("不支持该 HTTP 请求方法")
	}
	if len(config.Body) > maxOutputBytes {
		return errors.New("请求体不能超过 64 KiB")
	}
	if len(config.Headers) > 64 {
		return errors.New("请求头不能超过 64 项")
	}
	for key, value := range config.Headers {
		if key == "" || len(key) > 256 || strings.ContainsAny(value, "\r\n\x00") || len(value) > 8192 {
			return errors.New("请求头名称或内容不合法")
		}
		for _, char := range key {
			if !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", char) {
				return errors.New("请求头名称不合法")
			}
		}
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Transfer-Encoding") {
			return errors.New("Content-Length 和 Transfer-Encoding 由系统自动设置")
		}
	}
	return nil
}

// task 把任务行解析为可执行体与"调度身份"（锁键标识）：
// system 返回方法名，http/shell 返回 "job:编码"；身份用于跨实例互斥锁。
func (s *Service) task(j model.Job) (TaskFunc, string, error) {
	config, err := s.normalizeConfig(j.Kind, decodeConfig(j))
	if err != nil {
		return nil, "", err
	}
	switch j.Kind {
	case model.JobKindSystem:
		s.mu.Lock()
		fn := s.builtinFn[config.Handler]
		s.mu.Unlock()
		// 多个任务调用同一系统方法时也要串行，避免业务方法重入。
		return fn, config.Handler, nil
	case model.JobKindHTTP:
		return func(ctx context.Context) (string, error) { return runHTTP(ctx, config) }, "job:" + j.Code, nil
	case model.JobKindShell:
		return func(ctx context.Context) (string, error) { return runShell(ctx, s.shellPath, config) }, "job:" + j.Code, nil
	}
	return nil, "", errors.New("未知任务类型")
}

// invoke 执行任务体并把 panic 兜底转为错误，避免单个坏任务拖垮调度 goroutine；
// 上下文已取消时以 ctx.Err() 为准（超时/取消优先于任务自身返回）。
func invoke(ctx context.Context, fn TaskFunc) (result string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("任务执行异常: %v", recovered)
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
	}()
	return fn(ctx)
}

// runHTTP 发起一次 HTTP 请求：限制重定向次数与协议，读取体量受限，非 2xx 视为失败。
func runHTTP(ctx context.Context, config ExecutionConfig) (string, error) {
	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, strings.NewReader(config.Body))
	if err != nil {
		return "", err
	}
	for key, value := range config.Headers {
		if strings.EqualFold(key, "Host") {
			req.Host = value
		} else {
			req.Header.Set(key, value)
		}
	}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("重定向次数超过 5 次")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return errors.New("重定向地址必须为 HTTP/HTTPS")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOutputBytes+1))
	if err != nil {
		return "", err
	}
	result := "HTTP " + resp.Status + "\n" + limitOutput(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, errors.New("接口返回非 2xx 状态码")
	}
	return result, nil
}

// runShell 调用配置的解释器执行脚本：脚本内容经 stdin 送入（不落 argv），
// 文件脚本以路径执行；configureProcess 负责在超时/取消时终止整个进程组。
func runShell(ctx context.Context, shellPath string, config ExecutionConfig) (string, error) {
	args := []string{"-s", "--"}
	if config.ScriptPath != "" {
		args = []string{"--", config.ScriptPath}
	}
	args = append(args, config.Args...)
	cmd := exec.CommandContext(ctx, shellPath, args...)
	if config.ScriptPath == "" {
		cmd.Stdin = strings.NewReader(strings.ReplaceAll(config.Script, "\r\n", "\n"))
	}
	cmd.Dir = config.WorkDir
	cmd.WaitDelay = time.Second
	configureProcess(cmd)
	output := &boundedOutput{}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return output.String(), err
}

// boundedOutput 是并发安全的限长缓冲，模拟 io.Writer 供脚本 stdout/stderr 共用。
type boundedOutput struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

// Write 始终返回已写入长度，超出上限的部分丢弃并标记截断，避免调用方报错。
func (output *boundedOutput) Write(data []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	room := maxOutputBytes - len(output.data)
	if len(data) > room {
		output.truncated = true
	}
	output.data = append(output.data, data[:min(len(data), room)]...)
	return len(data), nil
}

// String 输出缓冲内容，非法 UTF-8 字节替换为 �，截断时追加提示。
func (output *boundedOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	result := strings.ToValidUTF8(string(output.data), "�")
	if output.truncated {
		result += "\n[输出已截断，最多保留 64 KiB]"
	}
	return result
}

// limitOutput 把任意来源的文本裁剪到 64 KiB，按 rune 边界切分以免切断多字节字符。
func limitOutput(result string) string {
	if len(result) <= maxOutputBytes {
		return strings.ToValidUTF8(result, "�")
	}
	end := maxOutputBytes
	for end > 0 && !utf8.RuneStart(result[end]) {
		end--
	}
	return strings.ToValidUTF8(result[:end], "�") + "\n[输出已截断，最多保留 64 KiB]"
}
