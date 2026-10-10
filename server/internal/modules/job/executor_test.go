package job

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"pet-manager/server/internal/model"
)

// TestCurlRequests 覆盖 curl 语法解析：合法写法可解析，管道/重定向/文件读取等被拒绝。
func TestCurlRequests(t *testing.T) {
	tests := []struct{ command, method, body string }{
		{`curl 'https://example.com/api?a=1&b=2'`, "GET", ""},
		{`curl -XPOST https://example.com/api -H 'Content-Type: application/json' --data-raw '{"a":"quoted value"}'`, "POST", `{"a":"quoted value"}`},
		{"curl --url https://example.com/api \\\n --json '{\"a\":1}'", "POST", `{"a":1}`},
		{`curl -I https://example.com/api`, "HEAD", ""},
		{`curl https://example.com/api -G --data-urlencode 'name=毛孩子'`, "GET", ""},
		{`curl https://example.com/api --data-raw '@literal-body'`, "POST", "@literal-body"},
		{`curl https://example.com/api --data-raw "{\"a\":\"line\\nvalue\"}"`, "POST", `{"a":"line\nvalue"}`},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			config, err := parseCurl(test.command)
			if err != nil {
				t.Fatal(err)
			}
			if config.Method != test.method || config.Body != test.body {
				t.Fatalf("config=%+v", config)
			}
		})
	}
	invalid := []string{
		`curl https://example.com/api; echo unsafe`,
		`curl https://example.com/api | sh`,
		`curl https://example.com/api > output.txt`,
		`curl https://example.com/api --data-binary @/etc/passwd`,
		`curl file:///etc/passwd`,
		`curl https://example.com/api https://example.com/second`,
		`curl https://example.com/api -H 'unclosed`,
		`curl https://example.com/api -X`,
		`curl https://example.com/api -o local.txt`,
	}
	for _, command := range invalid {
		if _, err := parseCurl(command); err == nil {
			t.Errorf("应拒绝 %s", command)
		}
	}
}

// TestHTTPExecution 覆盖 HTTP 执行：成功、非 2xx、输出截断与超时中断。
func TestHTTPExecution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream failed"))
			return
		}
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/large" {
			_, _ = w.Write([]byte(strings.Repeat("x", maxOutputBytes*2)))
			return
		}
		if r.Method != "POST" || r.Header.Get("X-Task") != "test" {
			t.Errorf("请求参数未传递")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["name"] != "pet" {
			t.Errorf("请求体错误: %v", err)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	config, err := parseCurl(`curl -X POST '` + server.URL + `/ok' -H 'X-Task: test' --json '{"name":"pet"}'`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runHTTP(context.Background(), config)
	if err != nil || !strings.Contains(result, `{"ok":true}`) {
		t.Fatalf("%s %v", result, err)
	}
	result, err = runHTTP(context.Background(), ExecutionConfig{URL: server.URL + "/fail", Method: "GET"})
	if err == nil || !strings.Contains(result, "502") || !strings.Contains(result, "upstream failed") {
		t.Fatalf("HTTP失败未记录: %s %v", result, err)
	}
	result, err = runHTTP(context.Background(), ExecutionConfig{URL: server.URL + "/large", Method: "GET"})
	if err != nil || len(result) > maxOutputBytes+200 || !strings.Contains(result, "已截断") {
		t.Fatalf("输出未限制: %d %v", len(result), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = runHTTP(ctx, ExecutionConfig{URL: server.URL + "/slow", Method: "GET"})
	if err == nil || ctx.Err() == nil {
		t.Fatalf("HTTP超时未中断: %v", err)
	}
}

// TestShellExecution 覆盖脚本任务：参数/中文/错误输出、文件脚本与超时终止子进程。
func TestShellExecution(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("当前环境未安装 sh")
	}
	config := ExecutionConfig{Script: "printf 'arg=%s\\n' \"$1\"\nprintf 'stderr-check\\n' >&2\n", Args: []string{"a b; literal"}}
	result, err := runShell(context.Background(), shell, config)
	if err != nil || !strings.Contains(result, "arg=a b; literal") || !strings.Contains(result, "stderr-check") {
		t.Fatalf("脚本/参数/错误输出: %q %v", result, err)
	}
	config.Script = "printf 'failed-output\\n' >&2\nexit 7\n"
	result, err = runShell(context.Background(), shell, config)
	if err == nil || !strings.Contains(result, "failed-output") {
		t.Fatalf("退出错误未返回: %q %v", result, err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "script with spaces.sh")
	if err := os.WriteFile(file, []byte("printf 'file=%s\\n' \"$1\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = runShell(context.Background(), shell, ExecutionConfig{ScriptPath: file, Args: []string{"ok"}, WorkDir: dir})
	if err != nil || !strings.Contains(result, "file=ok") {
		t.Fatalf("文件脚本: %q %v", result, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = runShell(ctx, shell, ExecutionConfig{Script: "sleep 10 &\nwait\n"})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 3*time.Second {
		t.Fatalf("脚本超时/子进程终止失败: %v, %v", err, time.Since(started))
	}
}

// TestDefinitionAndMethods 覆盖任务定义默认值与系统方法注册、调度身份及 panic 兜底。
func TestDefinitionAndMethods(t *testing.T) {
	service := NewService(nil, nil, zap.NewNop())
	called := false
	service.RegisterMethod(Method{Code: "test_method", Name: "测试方法", Fn: func(ctx context.Context) (string, error) { called = true; return "ok", nil }})
	in := CreateInput{Code: "custom_task", Name: "自建任务", Kind: "system", Spec: "*/15 * * * *", Config: ExecutionConfig{Handler: "test_method"}}
	row, err := service.definition(in)
	if err != nil {
		t.Fatal(err)
	}
	if row.Builtin || row.Status != model.StatusDisabled || row.TimeoutSeconds != 60 || row.NextRunAt != nil {
		t.Fatalf("新任务默认设置错误: %+v", row)
	}
	fn, identity, err := service.task(row)
	if err != nil || identity != "test_method" {
		t.Fatalf("未按方法编码调度: %q %v", identity, err)
	}
	if _, err := invoke(context.Background(), fn); err != nil || !called {
		t.Fatal("系统方法未执行")
	}
	if decodeConfig(model.Job{Code: "legacy", Kind: "system", Config: "{}"}).Handler != "legacy" {
		t.Fatal("旧预置任务迁移不兼容")
	}
	in.Config.Handler = "not_registered"
	if _, err := service.definition(in); err == nil {
		t.Fatal("未注册方法应拒绝")
	}
	in.Config.Handler = "test_method"
	for _, timeout := range []int{-1, 601} {
		in.TimeoutSeconds = timeout
		if _, err := service.definition(in); err == nil {
			t.Fatal("非法超时应拒绝")
		}
	}
	if len(service.Methods().Methods) != 1 {
		t.Fatal("方法目录缺失")
	}
	_, err = invoke(context.Background(), func(context.Context) (string, error) { panic("test panic") })
	if err == nil || !strings.Contains(err.Error(), "test panic") {
		t.Fatal("系统方法 panic 未转为失败结果")
	}
}

// TestOutputBoundsAndHeaderValidation 覆盖脚本输出限长与请求头白名单校验。
func TestOutputBoundsAndHeaderValidation(t *testing.T) {
	writer := &boundedOutput{}
	_, _ = writer.Write([]byte(strings.Repeat("a", maxOutputBytes+100)))
	if len(writer.String()) > maxOutputBytes+100 || !strings.Contains(writer.String(), "已截断") {
		t.Fatal("脚本输出未限制")
	}
	for _, headers := range []map[string]string{{"Bad Header": "x"}, {"X-Test": "a\r\nb"}, {"Content-Length": "3"}} {
		if err := validateHTTP(ExecutionConfig{URL: "https://example.com", Method: "GET", Headers: headers}); err == nil {
			t.Fatal("非法请求头应拒绝")
		}
	}
}
