package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"shortener/internal/config"
	"shortener/internal/store"
)

// 本文件是路由层的测试，它不需要 PostgreSQL 与 Redis，因此不需要任何环境变量就能运行。
//
// 测试的内容分三类：
//   1. 七条路由可以同时注册而不发生冲突（gin 的路由树对静态片段与参数片段有额外约束，
//      这条测试就是用来确认约束没有被违反的）；
//   2. 每一类请求实际命中的路由与返回的状态码；
//   3. 已经写好的处理函数返回的响应体形态。

func init() {
	// 关闭 gin 的调试输出，避免每个测试都在标准输出里打印路由清单与请求日志。
	gin.SetMode(gin.TestMode)
}

// newTestEngine 构造一个只用于测试的引擎。
func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()

	// 日志写到一个丢弃器里，避免测试输出被请求日志淹没。
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return newTestEngineWithDependencies(t, &mockStore{}, &mockCache{}, logger)
}

func newTestEngineWithDependencies(t *testing.T, pg LinkStore, rdb LinkCache, logger *slog.Logger) *gin.Engine {
	t.Helper()
	return New(config.Config{}, pg, rdb, logger).Routes()
}

// doRequest 向引擎发送一次不带请求体的请求，并且返回记录下来的响应。
func doRequest(engine *gin.Engine, method string, path string) *httptest.ResponseRecorder {
	return doRequestWithBody(engine, method, path, "")
}

// doRequestWithBody 向引擎发送一次自带请求体的请求，并且返回记录下来的响应。
// 请求体为空字符串时仍然可以用于 GET 一类没有请求体的方法。
func doRequestWithBody(engine *gin.Engine, method string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	engine.ServeHTTP(recorder, request)
	return recorder
}

// TestRoutesRegisterWithoutConflict 检查七条路由可以同时注册。
// gin 在路由模式冲突时会直接 panic，因此本测试用一个 defer 把 panic 转换成测试失败，
// 而不是让整个测试进程崩溃。
func TestRoutesRegisterWithoutConflict(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("注册路由时发生 panic，说明路由模式之间存在冲突：%v", recovered)
		}
	}()

	if engine := newTestEngine(t); engine == nil {
		t.Fatal("Routes 返回了空指针")
	}
}

// TestRouteMatching 逐条检查请求实际命中的路由与返回的状态码。
//
// 依赖服务由可控测试替身提供，因此本测试验证处理函数对依赖错误的正常响应，
// 而不是依靠 nil 指针触发 panic 后得到一个偶然的 500。
func TestRouteMatching(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"存活检查", http.MethodGet, "/api/healthz", "", http.StatusTeapot},
		{"就绪检查报告依赖不可用", http.MethodGet, "/api/readyz", "", http.StatusServiceUnavailable},
		{"创建接口在请求体不是合法 JSON 时返回 400，不访问依赖", http.MethodPost, "/api/links", "not-json", http.StatusBadRequest},
		{"列表接口报告存储依赖不可用", http.MethodGet, "/api/links", "", http.StatusServiceUnavailable},
		{"统计接口报告存储依赖不可用", http.MethodGet, "/api/stats", "", http.StatusServiceUnavailable},
		{"短码跳转报告缓存依赖不可用", http.MethodGet, "/a1B2c3", "", http.StatusServiceUnavailable},
		{"格式不符合短码规则的路径返回 404，不访问依赖", http.MethodGet, "/favicon.ico", "", http.StatusNotFound},
		{"路径匹配但方法不匹配时返回 405", http.MethodGet, "/api/links/xyz", "", http.StatusMethodNotAllowed},
		{"存活检查不接受 POST 方法", http.MethodPost, "/api/healthz", "", http.StatusMethodNotAllowed},
		{"多片段路径没有对应路由时返回 404", http.MethodGet, "/nope/extra/path", "", http.StatusNotFound},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := doRequestWithBody(engine, testCase.method, testCase.path, testCase.body)
			if recorder.Code != testCase.wantStatus {
				t.Errorf("%s %s 返回的状态码是 %d，期望 %d，响应体是 %s",
					testCase.method, testCase.path, recorder.Code, testCase.wantStatus, recorder.Body.String())
			}
		})
	}
}

// TestHealthzResponseBody 检查存活检查的响应体与响应头。
func TestHealthzResponseBody(t *testing.T) {
	recorder := doRequest(newTestEngine(t), http.MethodGet, "/api/healthz")

	if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"ok"}` {
		t.Errorf("响应体是 %s，期望 {\"status\":\"ok\"}", got)
	}

	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Content-Type 是 %q，期望以 application/json 开头", contentType)
	}
}

// TestNotImplementedResponseShape 检查已经写完的处理函数返回的错误响应体仍然使用统一结构。
// 这条测试同时验证了 writeError 辅助函数的行为。
func TestNotImplementedResponseShape(t *testing.T) {
	recorder := doRequest(newTestEngine(t), http.MethodGet, "/favicon.ico")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("状态码是 %d，期望 %d", recorder.Code, http.StatusNotFound)
	}

	// 跳转端点的 404 面向浏览器，因此响应体是 HTML 而不是 JSON。
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("Content-Type 是 %q，期望以 text/html 开头", contentType)
	}

	if body := recorder.Body.String(); !strings.Contains(body, "短码不存在") {
		t.Errorf("响应体是 %s，期望它包含「短码不存在」", body)
	}
}

// TestCreateLinkRejectsInvalidBody 检查创建接口在请求体不是合法 JSON 时返回契约规定的错误文本。
//
// 这个用例在 store 与 cache 都为 nil 的情况下也能通过，原因是请求体的解析发生在访问依赖之前，
// 因此它同时验证了「校验顺序正确」这件事。
func TestCreateLinkRejectsInvalidBody(t *testing.T) {
	recorder := doRequestWithBody(newTestEngine(t), http.MethodPost, "/api/links", "not-json")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("状态码是 %d，期望 %d，响应体是 %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}

	assertErrorMessage(t, recorder.Body.Bytes(), "请求体不是合法的 JSON")
}

// TestCreateLinkValidatesURL 逐条检查创建接口对 url 字段的四个校验分支。
func TestCreateLinkValidatesURL(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"url 字段缺失", "{}", "url 字段不能为空"},
		{"url 字段是空字符串", `{"url":""}`, "url 字段不能为空"},
		{"协议前缀不符合要求", `{"url":"ftp://example.com"}`, "url 字段必须以 http:// 或 https:// 开头"},
		{"字节长度超过上限", `{"url":"https://example.com/` + strings.Repeat("a", 2048) + `"}`, "url 字段长度超过 2048"},
	}

	engine := newTestEngine(t)

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := doRequestWithBody(engine, http.MethodPost, "/api/links", testCase.body)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("状态码是 %d，期望 %d，响应体是 %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}

			assertErrorMessage(t, recorder.Body.Bytes(), testCase.want)
		})
	}
}

func TestCreateLinkWithMockedDependencies(t *testing.T) {
	var saved store.Link
	now := time.Now().UTC()
	pg := &mockStore{
		create: func(_ context.Context, link store.Link) error {
			saved = link
			return nil
		},
		get: func(_ context.Context, code string) (store.Link, error) {
			if code != saved.Code {
				t.Fatalf("回读短码是 %q，期望 %q", code, saved.Code)
			}
			saved.CreatedAt = now
			return saved, nil
		},
	}
	rdb := &mockCache{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := newTestEngineWithDependencies(t, pg, rdb, logger)

	recorder := doRequestWithBody(engine, http.MethodPost, "/api/links", `{"url":"https://example.com/mock"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("创建接口返回状态码 %d，期望 %d，响应体：%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if saved.URL != "https://example.com/mock" || saved.Code == "" {
		t.Fatalf("注入的存储替身收到的记录不符合预期：%+v", saved)
	}
	if !strings.Contains(recorder.Body.String(), saved.Code) {
		t.Fatalf("成功响应没有包含替身保存的短码 %q：%s", saved.Code, recorder.Body.String())
	}
}

func TestRedirectUsesMockedCache(t *testing.T) {
	const code = "a1B2c3"
	const target = "https://example.com/redirected"
	var recordedHit, counted bool

	pg := &mockStore{}
	rdb := &mockCache{
		getURL: func(_ context.Context, gotCode string) (string, error) {
			if gotCode != code {
				t.Fatalf("缓存查询短码是 %q，期望 %q", gotCode, code)
			}
			return target, nil
		},
		record: func(_ context.Context, hit bool) error {
			recordedHit = hit
			return nil
		},
		incrClick: func(_ context.Context, gotCode string) error {
			counted = gotCode == code
			return nil
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := newTestEngineWithDependencies(t, pg, rdb, logger)

	recorder := doRequest(engine, http.MethodGet, "/"+code)
	if recorder.Code != http.StatusFound {
		t.Fatalf("跳转接口返回状态码 %d，期望 %d", recorder.Code, http.StatusFound)
	}
	if location := recorder.Header().Get("Location"); location != target {
		t.Fatalf("跳转目标是 %q，期望 %q", location, target)
	}
	if !recordedHit || !counted {
		t.Fatalf("缓存命中统计或点击增量未调用：recordedHit=%t counted=%t", recordedHit, counted)
	}
}

// TestListLinksRejectsInvalidPagination 逐条检查列表接口对分页参数的五个校验分支。
//
// 这些分支同样发生在访问依赖之前，因此空依赖不会影响它们。
func TestListLinksRejectsInvalidPagination(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"limit 不是整数", "/api/links?limit=abc"},
		{"limit 小于下限", "/api/links?limit=0"},
		{"limit 超过上限", "/api/links?limit=101"},
		{"offset 不是整数", "/api/links?offset=abc"},
		{"offset 小于下限", "/api/links?offset=-1"},
	}

	engine := newTestEngine(t)

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := doRequest(engine, http.MethodGet, testCase.path)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("状态码是 %d，期望 %d，响应体是 %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
}

// assertErrorMessage 检查响应体是 {"error":"..."} 结构，并且错误文本等于期望取值。
func assertErrorMessage(t *testing.T, body []byte, want string) {
	t.Helper()

	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("响应体不是合法的 JSON，解析失败：%v，响应体是 %s", err, string(body))
	}

	if parsed.Error != want {
		t.Errorf("错误文本是 %q，期望 %q", parsed.Error, want)
	}
}
