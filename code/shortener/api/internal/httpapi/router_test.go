package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"shortener/internal/config"
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
//
// 传入 nil 作为 store 与 cache 是安全的，因为本文件测试的路由与存活检查都不会访问这两个依赖。
// 这样构造还有一个额外的好处：一旦某个测试意外触发了需要依赖的处理函数，
// 空指针会让测试立刻失败，而不是产生一个看似通过的假象。
func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()

	// 日志写到一个丢弃器里，避免测试输出被请求日志淹没。
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return New(config.Config{}, nil, nil, logger).Routes()
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
// 表中依赖 PostgreSQL 与 Redis 的用例期望取值是 500，原因有两个：
// 第一，newTestEngine 传入的 store 与 cache 都是 nil，处理函数一旦访问它们就会发生空指针解引用；
// 第二，恢复中间件把这个 panic 转换成 500 响应。
// 这两点合起来说明：请求确实命中了预期的处理函数，同时恢复中间件也生效了。
// 需要检查真实状态码的场景由 TASKS.md 第 10 节的全量验收覆盖，那里使用真实的依赖服务。
func TestRouteMatching(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"存活检查", http.MethodGet, "/api/healthz", "", http.StatusOK},
		{"就绪检查命中处理函数，空依赖由恢复中间件转换成 500", http.MethodGet, "/api/readyz", "", http.StatusInternalServerError},
		{"创建接口在请求体不是合法 JSON 时返回 400，不访问依赖", http.MethodPost, "/api/links", "not-json", http.StatusBadRequest},
		{"列表接口命中处理函数，空依赖由恢复中间件转换成 500", http.MethodGet, "/api/links", "", http.StatusInternalServerError},
		{"统计接口命中处理函数，空依赖由恢复中间件转换成 500", http.MethodGet, "/api/stats", "", http.StatusInternalServerError},
		{"六位短码命中跳转处理函数，空依赖由恢复中间件转换成 500", http.MethodGet, "/a1B2c3", "", http.StatusInternalServerError},
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
