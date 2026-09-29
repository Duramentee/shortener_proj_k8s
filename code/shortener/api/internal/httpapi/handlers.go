package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"shortener/internal/store"
	"shortener/internal/todo"
)

// ---------------------------------------------------------------------------
// 响应结构与请求结构。这些结构体定义的就是 README 第 4 节冻结的接口契约，
// 因此它们与 store.Link 是分开定义的：
// 数据库的列可以随表结构变化，而接口契约一旦冻结就不能再改，
// 两者分开之后，表结构的调整不会直接改变对外接口的形态。
//
// 下面的结构体、转换函数与辅助函数都已经写好，你不需要修改它们。
// ---------------------------------------------------------------------------

// errorResponse 是全部错误响应的统一结构，对应契约中的 {"error":"..."}。
type errorResponse struct {
	Error string `json:"error"`
}

// healthResponse 是存活检查的响应结构。
type healthResponse struct {
	Status string `json:"status"`
}

// linkResponse 是单条短链接在接口中的形态。
// createdAt 使用 time.Time 类型，序列化时由标准库写成 RFC 3339 格式，
// 前端的 new Date(...) 可以直接解析这个格式。
type linkResponse struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	Clicks    int64     `json:"clicks"`
	CreatedAt time.Time `json:"createdAt"`
}

// linkListResponse 是列表接口的响应结构。
type linkListResponse struct {
	Items  []linkResponse `json:"items"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// createLinkRequest 是创建接口的请求结构。
type createLinkRequest struct {
	URL string `json:"url"`
}

// statsResponse 是统计接口的响应结构。
type statsResponse struct {
	Links        int64   `json:"links"`
	Clicks       int64   `json:"clicks"`
	CacheHits    int64   `json:"cacheHits"`
	CacheMisses  int64   `json:"cacheMisses"`
	CacheHitRate float64 `json:"cacheHitRate"`
}

// readinessResponse 是就绪探针的响应结构。
// Checks 使用映射类型，键是依赖名称，取值是 "ok" 或者具体的错误原因。
type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// toLinkResponse 把存储层结构转换成接口结构。
func toLinkResponse(link store.Link) linkResponse {
	return linkResponse{
		Code:      link.Code,
		URL:       link.URL,
		Clicks:    link.Clicks,
		CreatedAt: link.CreatedAt,
	}
}

// toLinkResponses 批量转换，并且保证结果不是 nil。
// 返回长度为 0 的切片而不是 nil 是必要的：nil 切片会被序列化成 null，
// 而接口契约规定 items 字段始终是数组，前端的 LinkTable 组件也直接依赖这一点。
func toLinkResponses(links []store.Link) []linkResponse {
	items := make([]linkResponse, 0, len(links))
	for _, link := range links {
		items = append(items, toLinkResponse(link))
	}
	return items
}

// ---------------------------------------------------------------------------
// 请求解析与响应写出的辅助函数。这些函数已经写好，你不需要修改它们。
// ---------------------------------------------------------------------------

// maxRequestBodyBytes 是请求体的字节数上限。
// 使用 http.MaxBytesReader 限制请求体大小，而不是先读取 Content-Length 头部再做判断，
// 原因是 Content-Length 头部由客户端提供，可以被伪造；
// 而 MaxBytesReader 是在读取过程中实际达到上限时报错，因此不受客户端声明的影响。
const maxRequestBodyBytes = 64 * 1024

// writeError 按接口契约中统一的错误结构写出错误响应。
//
// gin 的 c.JSON 会自动把 Content-Type 设置为 application/json; charset=utf-8
// 并且完成序列化与写出，因此这里不需要手写响应头。
func (s *Server) writeError(c *gin.Context, status int, message string) {
	c.JSON(status, errorResponse{Error: message})
}

// decodeJSON 把请求体解析到 dst 指向的结构体。
// 解析失败时返回错误，由调用方决定转换成哪一个状态码。
//
// 本工程刻意不使用 gin 的 c.ShouldBindJSON，原因有两条。
// 第一条是接口契约要求区分「请求体不是合法的 JSON」与「url 字段缺失或者为空」这两种情况，
// 并且各自返回固定的错误文本；c.ShouldBindJSON 搭配 binding 标签使用时，
// 这两类失败会混合成同一个错误对象，错误文本由框架生成，与契约中的文本不一致。
// 第二条是契约对 url 字段的校验规则（长度上限、协议前缀）属于业务规则而不是结构约束，
// 写在处理函数里比写成结构体标签更容易读懂，也更容易改。
func decodeJSON(c *gin.Context, dst any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	return json.NewDecoder(c.Request.Body).Decode(dst)
}

// ---------------------------------------------------------------------------
// 处理函数。handleHealthz 已经写好，其余六个由你实现，任务分组编号写在各自上方。
//
// 全部处理函数的第一个参数都是 *gin.Context，它是 gin 对「一次请求的上下文」的封装，
// 同时提供读取请求（c.Request、c.Param、c.Query）与写出响应（c.JSON、c.Data、c.Redirect）
// 两类能力。与 net/http 的对照关系见 GO-CHEATSHEET.md 第 13 节。
// ---------------------------------------------------------------------------

// handleHealthz 是存活探针的检查端点，对应接口契约中的 GET /api/healthz。
//
// 本函数已经写好，你不需要修改它。
//
// 它刻意不检查 PostgreSQL 与 Redis，只证明进程还能够响应请求。
// 这样设计的原因是：存活探针失败时 kubelet 会重启容器，
// 如果存活探针也检查依赖，那么依赖不可用时容器会被反复重启，
// 而重启既不能修复依赖，又会让「依赖故障」这个原因被「容器不断重启」这个表象掩盖，
// 排障时反而更难定位。检查依赖是就绪探针的职责，见第 7 组任务。
func (s *Server) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

// handleReadyz 是就绪探针的检查端点，对应接口契约中的 GET /api/readyz。
//
// TODO(第 7 组)：由你实现本函数。
//
// 实现要求：
//  1. 依次检查 PostgreSQL 与 Redis 是否可达，调用 store.Postgres 的 Ping 方法与
//     cache.Redis 的 Ping 方法。
//  2. 两个依赖都可达时返回状态码 200，响应体中的 status 字段为 "ready"，
//     checks 字段中 postgres 与 redis 两个键的取值都是 "ok"。
//  3. 任意一个依赖不可用时返回状态码 503，响应体中的 status 字段为 "not ready"，
//     checks 字段中不可用那个依赖的取值写上具体的错误原因。
//  4. 检查时使用的 context 必须带超时，例如 2 秒。原因是：如果依赖处于
//     「TCP 连接可以建立但是不响应」的状态（这种状态在容器网络里很常见），
//     不带超时的检查会一直等待，就绪探针每次都要等到 kubelet 那边超时才有结果，
//     而 kubelet 会因此重启容器，反而制造出额外的故障。
//     使用 context.WithTimeout 可以在 2 秒内得到一个明确的结论。
//  5. 检查必须覆盖两个依赖，不能因为 PostgreSQL 检查失败就跳过 Redis 的检查。
//     原因是 checks 字段同时返回两个依赖的状态，排障时一次就能看清全部原因，
//     而不是修好一个之后再发现另一个也有问题。
//
// 需要你添加的导入：context、time。
//
// 验收方式：见 TASKS.md 第 7 组的验收表。
func (s *Server) handleReadyz(c *gin.Context) {
	s.writeError(c, http.StatusNotImplemented, todo.Message("httpapi.Server.handleReadyz", 7))
}

// handleCreateLink 创建短链接，对应接口契约中的 POST /api/links。
//
// TODO(第 4 组)：由你实现本函数。
//
// 实现要求：
//  1. 调用 decodeJSON(c, &请求变量) 解析请求体，解析失败时返回 400 与
//     {"error":"请求体不是合法的 JSON"}。
//  2. 依次校验 url 字段，每一种情况对应一个固定的状态码与错误文本，文本必须与契约一致：
//     url 缺失或者为空字符串时返回 400 与 {"error":"url 字段不能为空"}；
//     url 不以 http:// 或者 https:// 开头时返回 400 与
//     {"error":"url 字段必须以 http:// 或 https:// 开头"}；
//     url 的字节长度超过 2048 时返回 400 与 {"error":"url 字段长度超过 2048"}。
//     注意长度限制的单位是字节数而不是字符数，这一点在含中文的网址上会表现出差异，
//     你可以把它作为一个值得记录的现象写进笔记。
//  3. 调用 shortcode.Generate 生成短码，然后调用 s.store.CreateLink 写入数据库。
//     短码冲突时数据库会返回主键冲突错误，此时必须重新生成短码再试一次，
//     重试次数限制为 5 次；5 次仍然冲突时返回 500。
//     判断冲突的方式见 store.Postgres.CreateLink 的注释。
//  4. 创建成功时返回 201 与创建出来的记录，响应体结构使用 linkResponse，
//     转换函数 toLinkResponse 已经写好，直接调用即可。
//  5. 数据库返回其他错误时返回 503 与 {"error":"依赖服务不可用"}。
//
// 需要你添加的导入：errors、strings、shortener/internal/shortcode。
//
// 验收方式：见 TASKS.md 第 4 组的验收表。
func (s *Server) handleCreateLink(c *gin.Context) {
	s.writeError(c, http.StatusNotImplemented, todo.Message("httpapi.Server.handleCreateLink", 4))
}

// handleListLinks 分页列出短链接，对应接口契约中的 GET /api/links。
//
// TODO(第 4 组)：由你实现本函数。
//
// 实现要求：
//  1. limit 参数的默认值是 20，上限是 100；offset 参数的默认值是 0。
//     读取查询参数可以使用 gin 提供的 c.DefaultQuery("limit", "20")，
//     它会在参数缺失时返回第二个参数作为默认值，然后自己用 strconv.Atoi 转换并判断范围。
//     参数不是合法的整数时返回 400；limit 小于 1 或者大于 100 时返回 400；
//     offset 小于 0 时返回 400。
//     上限取值的意义是防止一次请求把整张表读出来，这在数据量增长之后会拖慢数据库。
//  2. 先调用 s.store.ListLinks 取得当前页的记录，再调用 s.store.CountLinks 取得总数。
//  3. 返回 200 与 linkListResponse，其中 items 必须是数组而不是 null
//     （转换函数 toLinkResponses 已经保证这一点）。
//  4. 任意一步出错时返回 503 与 {"error":"依赖服务不可用"}。
//
// 需要你添加的导入：net/http 已经导入，还需要添加 strconv。
//
// 验收方式：见 TASKS.md 第 4 组的验收表。
func (s *Server) handleListLinks(c *gin.Context) {
	s.writeError(c, http.StatusNotImplemented, todo.Message("httpapi.Server.handleListLinks", 4))
}

// handleDeleteLink 删除短链接，对应接口契约中的 DELETE /api/links/{code}。
//
// TODO(第 6 组)：由你实现本函数。
//
// 实现要求：
//  1. 短码从路径参数中读取，写法是 c.Param("code")，
//     其中 "code" 与路由模式 "/links/:code" 中的参数名一致。
//  2. 先删除数据库记录。数据库返回 ErrNotFound 时返回 404 与 {"error":"短码不存在"}
//     （判断方式是 errors.Is(err, store.ErrNotFound)）。
//  3. 数据库删除成功之后再调用 s.cache.DeleteLink 删除 Redis 中的两个键。
//     顺序必须是先数据库后缓存，原因是：如果先删缓存再删数据库，而数据库删除失败，
//     那么缓存里的记录已经消失，后续请求会回落到数据库并且把这条记录重新填充进缓存，
//     表现为「删除操作看起来没有生效」。
//  4. 缓存删除失败时不要返回错误。缓存中的键带有生存时间，即使删除失败也会自动过期，
//     而数据库中的记录已经删除，因此数据最终是一致的；
//     此时应当记录一条警告日志而不是让整个请求失败。
//  5. 删除成功时返回 204 且响应体为空。gin 中写法是 c.Status(http.StatusNoContent)。
//     注意不要写成 c.JSON(http.StatusNoContent, nil)，那样会写出一个内容为 null 的响应体，
//     与契约中「响应体为空」的要求不一致。
//
// 需要你添加的导入：errors。
//
// 验收方式：见 TASKS.md 第 6 组的验收表。
func (s *Server) handleDeleteLink(c *gin.Context) {
	s.writeError(c, http.StatusNotImplemented, todo.Message("httpapi.Server.handleDeleteLink", 6))
}

// handleStats 返回汇总统计，对应接口契约中的 GET /api/stats。
//
// TODO(第 6 组)：由你实现本函数。
//
// 实现要求：
//  1. 调用 s.store.CountLinks 取得短链接总数，调用 s.store.SumClicks 取得累计点击数。
//  2. 调用 s.cache.CacheStats 取得缓存命中与未命中的累计次数。
//  3. 计算命中率：cacheHitRate 的取值是命中次数除以命中次数与未命中次数之和；
//     分母为 0 时返回 0。分母为 0 的情况出现在 Redis 刚启动、还没有发生过任何一次查询时，
//     此时直接做除法会得到 NaN，而 NaN 不是合法的 JSON，
//     序列化会失败并导致响应写出出错，因此必须先判断分母。
//  4. 返回 200 与 statsResponse。
//  5. 任意一步出错时返回 503 与 {"error":"依赖服务不可用"}。
//
// 验收方式：见 TASKS.md 第 6 组的验收表。
func (s *Server) handleStats(c *gin.Context) {
	s.writeError(c, http.StatusNotImplemented, todo.Message("httpapi.Server.handleStats", 6))
}

// handleRedirect 执行短码跳转，对应接口契约中的 GET /{code}。
//
// TODO(第 5 组)：由你实现本函数。
//
// 实现要求：
//  1. 从路径参数读取短码，写法是 c.Param("code")；
//     校验它恰好是 6 个字符并且每个字符都出现在 shortcode.Alphabet 中。
//     不满足时返回 404。需要这一步的原因是直接访问后端端口时，
//     像 /favicon.ico 与 /api 这样的单片段路径也会被 "/:code" 这条路由匹配到。
//  2. 先查缓存：调用 s.cache.GetURL。命中时记录一次命中统计，直接把长网址用于跳转。
//  3. 缓存未命中时记录一次未命中统计，然后调用 s.store.GetLink 查询数据库。
//     数据库返回 ErrNotFound 时返回 404 与一段 HTML 响应体（见第 6 条）。
//  4. 数据库返回记录时，把长网址写入缓存，生存时间使用 s.cfg.CacheTTL，
//     然后用于跳转。写入缓存这一步是「读时回填」，
//     也就是 README 第 5 节中 link:{code} 键的两种写入时机之一。
//  5. 跳转使用状态码 302 与 Location 响应头。gin 中的写法是
//     c.Redirect(http.StatusFound, 长网址)，框架会同时写出状态码与响应头。
//     使用 302 而不是 301 的原因是 301 会被浏览器长期缓存，
//     之后即使数据库中的记录被删除，浏览器仍然会直接跳转而不再访问本服务。
//  6. 短码不存在时返回 404，响应体的 Content-Type 是 text/html，内容是一段说明短码不存在的 HTML。
//     gin 中的写法是 c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(html))。
//     与其余接口返回 JSON 不同，这个端点面向浏览器，返回 HTML 更适合直接展示。
//  7. 跳转成功之后调用 s.cache.IncrClick 累加点击增量。
//     这一步失败时不要影响跳转：用户已经拿到了目标地址，
//     统计丢一次比让用户看到错误页面更符合预期，因此失败时只记录警告日志。
//  8. 访问 Redis 或者数据库出错（既不是未命中也不是不存在）时返回 503 与
//     {"error":"依赖服务不可用"}。
//
// 需要你添加的导入：errors、strings、shortener/internal/shortcode。
//
// 验收方式：见 TASKS.md 第 5 组的验收表。
func (s *Server) handleRedirect(c *gin.Context) {
	s.writeError(c, http.StatusNotImplemented, todo.Message("httpapi.Server.handleRedirect", 5))
}
