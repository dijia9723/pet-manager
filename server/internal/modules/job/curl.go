package job

// 本文件把 curl 文本解析为 ExecutionConfig，随后交由 Go HTTP 客户端执行；
// curl 命令本身不会被送入 Shell，因此不构成命令注入面。
import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

// parseCurl 解析请求语法后交给 Go HTTP 客户端，curl 文本不会被送入 Shell。
func parseCurl(command string) (ExecutionConfig, error) {
	words, err := curlWords(command)
	if err != nil {
		return ExecutionConfig{}, err
	}
	if len(words) < 2 || (words[0] != "curl" && words[0] != "curl.exe") {
		return ExecutionConfig{}, errors.New("请输入以 curl 开头的 HTTP 请求命令")
	}
	config := ExecutionConfig{Headers: map[string]string{}}
	var data []string
	explicitMethod := ""
	get := false
	setURL := func(value string) error {
		if config.URL != "" {
			return errors.New("每个任务只支持一个请求地址")
		}
		config.URL = value
		return nil
	}
	for i := 1; i < len(words); i++ {
		word := words[i]
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			if err := setURL(word); err != nil {
				return ExecutionConfig{}, err
			}
			continue
		}
		switch word {
		case "-s", "-S", "-sS", "-Ss", "--silent", "--show-error", "-L", "--location", "--compressed":
			continue
		case "-G", "--get":
			get = true
			continue
		case "-I", "--head":
			explicitMethod = http.MethodHead
			continue
		}
		option, value, inline := word, "", false
		if strings.HasPrefix(word, "--") {
			if before, after, found := strings.Cut(word, "="); found {
				option, value, inline = before, after, true
			}
		} else if len(word) > 2 && strings.ContainsRune("XHdub", rune(word[1])) {
			option, value, inline = word[:2], word[2:], true
		}
		switch option {
		case "-X", "--request", "-H", "--header", "-d", "--data", "--data-raw", "--data-binary", "--data-urlencode", "--json", "-u", "--user", "-b", "--cookie", "--url":
		default:
			return ExecutionConfig{}, fmt.Errorf("不支持 curl 选项 %s，请使用请求表单；支持 -X、-H、-d、--data-raw、--json、-u、-b、-G", option)
		}
		if !inline {
			i++
			if i >= len(words) {
				return ExecutionConfig{}, fmt.Errorf("curl 选项 %s 缺少参数", option)
			}
			value = words[i]
		}
		switch option {
		case "-X", "--request":
			explicitMethod = strings.ToUpper(value)
		case "--url":
			if err := setURL(value); err != nil {
				return ExecutionConfig{}, err
			}
		case "-H", "--header":
			key, val, found := strings.Cut(value, ":")
			if !found || strings.TrimSpace(key) == "" {
				return ExecutionConfig{}, errors.New("请求头格式应为 Header-Name: value")
			}
			key, val = http.CanonicalHeaderKey(strings.TrimSpace(key)), strings.TrimSpace(val)
			if previous, found := config.Headers[key]; found {
				val = previous + ", " + val
			}
			config.Headers[key] = val
		case "-u", "--user":
			if !strings.Contains(value, ":") {
				value += ":"
			}
			config.Headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(value))
		case "-b", "--cookie":
			if !strings.Contains(value, "=") {
				return ExecutionConfig{}, errors.New("Cookie 必须直接填写键值，不支持读取服务器文件")
			}
			config.Headers["Cookie"] = value
		default:
			if option != "--data-raw" && strings.HasPrefix(value, "@") {
				return ExecutionConfig{}, errors.New("curl 任务不支持读取服务器文件，请直接填写请求体")
			}
			if option == "--data-urlencode" {
				if key, val, found := strings.Cut(value, "="); found {
					value = url.QueryEscape(key) + "=" + url.QueryEscape(val)
				} else {
					value = url.QueryEscape(value)
				}
			}
			if option == "--json" {
				config.Headers["Content-Type"] = "application/json"
				config.Headers["Accept"] = "application/json"
			}
			data = append(data, value)
		}
	}
	config.Body = strings.Join(data, "&")
	config.Method = http.MethodGet
	if len(data) > 0 {
		config.Method = http.MethodPost
		if config.Headers["Content-Type"] == "" {
			config.Headers["Content-Type"] = "application/x-www-form-urlencoded"
		}
	}
	if get {
		u, err := url.Parse(config.URL)
		if err != nil {
			return ExecutionConfig{}, errors.New("curl 请求地址不合法")
		}
		if config.Body != "" {
			if u.RawQuery != "" {
				u.RawQuery += "&"
			}
			u.RawQuery += config.Body
		}
		config.URL, config.Body, config.Method = u.String(), "", http.MethodGet
	}
	if explicitMethod != "" {
		config.Method = explicitMethod
	}
	if err := validateHTTP(config); err != nil {
		return ExecutionConfig{}, err
	}
	return config, nil
}

// 引号与续行遵循常见 POSIX curl 写法；管道、重定向和命令连接不能成为请求的一部分。
func curlWords(command string) ([]string, error) {
	command = strings.ReplaceAll(strings.ReplaceAll(command, "\\\r\n", ""), "\\\n", "")
	var words []string
	var word strings.Builder
	var quote rune
	started := false
	chars := []rune(command)
	for i := 0; i < len(chars); i++ {
		char := chars[i]
		if char == 0 {
			return nil, errors.New("curl 命令不能包含空字符")
		}
		if quote != 0 {
			if char == quote {
				quote = 0
				continue
			}
			if char == '\\' && quote == '"' && i+1 < len(chars) && strings.ContainsRune("\\\"$`", chars[i+1]) {
				i++
				char = chars[i]
			}
			word.WriteRune(char)
			continue
		}
		if unicode.IsSpace(char) {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		started = true
		switch char {
		case '\'', '"':
			quote = char
		case '\\':
			i++
			if i >= len(chars) {
				return nil, errors.New("curl 命令以未完成的转义结尾")
			}
			word.WriteRune(chars[i])
		case ';', '|', '&', '<', '>', '`':
			return nil, errors.New("curl 任务只接受 HTTP 请求，不支持管道、重定向或连接多个命令")
		default:
			word.WriteRune(char)
		}
	}
	if quote != 0 {
		return nil, errors.New("curl 命令的引号未闭合")
	}
	if started {
		words = append(words, word.String())
	}
	return words, nil
}
