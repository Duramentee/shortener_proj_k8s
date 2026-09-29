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

// doRequest 向引擎发送一次请求并且返回记录下来的响应。
func doRequest(engine *gin.Engine, method string, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
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
// 表中「尚未实现」的几个用例返回 501，是因为对应的处理函数还没有填写；
// 等到第 4 组到第 7 组完成之后，这些用例的期望取值需要同步更新。
func TestRouteMatching(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"存活检查", http.MethodGet, "/api/healthz", http.StatusOK},
		{"就绪检查尚未实现", http.MethodGet, "/api/readyz", http.StatusNotImplemented},
		{"列表接口尚未实现", http.MethodGet, "/api/links", http.StatusNotImplemented},
		{"统计接口尚未实现", http.MethodGet, "/api/stats", http.StatusNotImplemented},
		{"六位短码命中跳转处理函数", http.MethodGet, "/a1B2c3", http.StatusNotImplemented},
		{"单片段的其他路径同样命中跳转处理函数", http.MethodGet, "/favicon.ico", http.StatusNotImplemented},
		{"路径匹配但方法不匹配时返回 405", http.MethodGet, "/api/links/xyz", http.StatusMethodNotAllowed},
		{"存活检查不接受 POST 方法", http.MethodPost, "/api/healthz", http.StatusMethodNotAllowed},
		{"多片段路径没有对应路由时返回 404", http.MethodGet, "/nope/extra/path", http.StatusNotFound},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := doRequest(engine, testCase.method, testCase.path)
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

// TestNotImplementedResponseShape 检查未实现的处理函数返回统一结构的错误响应体。
// 这条测试同时验证了 writeError 辅助函数的行为。
func TestNotImplementedResponseShape(t *testing.T) {
	recorder := doRequest(newTestEngine(t), http.MethodGet, "/api/readyz")

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法的 JSON，解析失败：%v，响应体是 %s", err, recorder.Body.String())
	}
	if !strings.Contains(body.Error, "尚未实现") {
		t.Errorf("错误文本是 %q，期望它包含「尚未实现」", body.Error)
	}
	if !strings.Contains(body.Error, "第 7 组") {
		t.Errorf("错误文本是 %q，期望它指明任务分组为第 7 组", body.Error)
	}
}
