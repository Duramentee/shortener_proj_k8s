// Package httpapi 负责 HTTP 层：路由注册、请求解析、响应写出与中间件。
//
// 本层是外部流量进入系统的第一站。它不直接操作数据库或者 Redis，
// 而是调用 store 与 cache 两个包提供的方法，
// 因此「外部请求的形态」与「数据存储的形态」在这里完成转换。
//
// 本包使用 gin 框架承载 HTTP 层。选择 gin 之后，路由注册、路径参数取值、
// JSON 响应写出这三件事由框架承担，本包只需要关心业务分支与状态码。
package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"shortener/internal/cache"
	"shortener/internal/config"
	"shortener/internal/store"
)

// Server 持有处理请求所需的全部依赖。
// 依赖通过结构体字段注入而不是包级变量，好处是构造过程是显式的，
// 并且同一个进程内可以构造出多个互相独立的实例用于测试。
type Server struct {
	cfg    config.Config
	store  *store.Postgres
	cache  *cache.Redis
	logger *slog.Logger
}

// New 构造 Server。
func New(cfg config.Config, pg *store.Postgres, rdb *cache.Redis, logger *slog.Logger) *Server {
	return &Server{
		cfg:    cfg,
		store:  pg,
		cache:  rdb,
		logger: logger,
	}
}

// Routes 构造 gin 引擎、注册全部路由、挂载中间件，并且返回引擎本身。
//
// 本函数已经写好，你不需要修改它。
//
// 返回值的类型是 *gin.Engine。gin 引擎实现了标准库的 http.Handler 接口
// （它拥有 ServeHTTP 方法），因此在 main 函数里可以直接赋值给 http.Server 的 Handler 字段，
// 不需要做任何适配。
func (s *Server) Routes() *gin.Engine {
	// gin.New 构造一个不带任何中间件的引擎。
	// 这里刻意不使用 gin.Default，原因是它自带的日志中间件把内容写到标准输出、格式固定，
	// 与本工程使用 log/slog 输出结构化日志的做法不一致；
	// 下面改为挂载本包自己实现的日志中间件与 panic 恢复中间件。
	engine := gin.New()

	// 把可信代理设置为空。gin 默认信任全部代理，会去解析 X-Forwarded-For 与 X-Real-IP
	// 这两个头部，在容器环境中这会让 c.ClientIP() 返回代理地址而不是真实客户端地址，
	// 并且 gin 会在启动时打印一条「你信任所有代理」的警告。
	// 传入 nil 表示不信任任何代理，此时 c.ClientIP() 返回 TCP 连接的对端地址。
	if err := engine.SetTrustedProxies(nil); err != nil {
		s.logger.Warn("设置可信代理失败，客户端地址的取值可能不准确", "错误", err.Error())
	}

	// 让「路径匹配但请求方法不匹配」返回状态码 405 而不是 404。
	// gin 这一项的默认取值是 false，也就是返回 404；
	// 打开之后行为与 net/http 的 ServeMux 一致，接口契约中的说明不需要改动。
	engine.HandleMethodNotAllowed = true

	// 中间件的挂载顺序是日志在最外层、panic 恢复在内层。
	// 这样排列的原因是：处理函数发生 panic 时，内层的恢复中间件先写出 500 响应，
	// 外层的日志中间件在 c.Next() 返回之后读到的是真实状态码而不是默认的 200。
	engine.Use(s.loggingMiddleware(), s.recoveryMiddleware())

	// 应用接口统一挂在 /api 前缀下面。
	// 使用路由分组之后，下面每一条注册只需要写前缀之后的部分，
	// 前缀需要调整时也只修改这一行。
	api := engine.Group("/api")
	{
		api.GET("/healthz", s.handleHealthz)
		api.GET("/readyz", s.handleReadyz)
		api.POST("/links", s.handleCreateLink)
		api.GET("/links", s.handleListLinks)
		api.DELETE("/links/:code", s.handleDeleteLink)
		api.GET("/stats", s.handleStats)
	}

	// 短码跳转使用一条只包含单片段参数的路由。
	// ":code" 匹配任意单个路径片段，因此 /a1B2c3 会命中它，
	// /api 与 /favicon.ico 这类单片段路径同样会命中它。
	// 生产形态下 nginx 只把「路径恰好是六位字母或数字」的请求转发给本服务，
	// 因此这些路径只在直接访问后端端口时才会到达这里，
	// 这就是 handleRedirect 内部必须校验短码格式的原因。
	engine.GET("/:code", s.handleRedirect)

	return engine
}
