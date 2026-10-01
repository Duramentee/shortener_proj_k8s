# api · 后端任务清单

> 本文件是阶段 2 的分步指引。目标是：按第 1 组到第 8 组的顺序填写代码，
> 每完成一组就用本组给出的命令验收，全部完成之后在宿主机上运行后端，
> 用 `curl` 依次验证 `README.md` 第 4 节定义的全部接口。

---

## 0. 这份清单怎么用

### 0.1 已经写好与由你实现的分工

骨架已经创建完毕，并且经过实际验证：依赖整理成功、整体编译成功、静态检查成功、路由测试全部通过、
服务可以正常启动、数据表可以自动创建、`GET /api/healthz` 返回 200、未实现的端点返回 501。

| 分类 | 内容 | 状态 |
|---|---|---|
| HTTP 框架（已选定） | HTTP 层使用 `github.com/gin-gonic/gin` v1.12.0。路由注册、路由分组、路径参数取值、JSON 响应写出、中间件链这五件事由框架承担 | 已经接入并且在 `go.mod` 中声明依赖，写法见 `GO-CHEATSHEET.md` 第 11 节与第 13 节 |
| 基础设施（已写好） | 环境变量解析（`internal/config`）、PostgreSQL 连接池创建与建表（`internal/store`）、Redis 客户端创建与键名函数（`internal/cache`）、基于 gin 的路由注册与中间件挂载（`internal/httpapi/router.go`）、请求日志中间件与 panic 恢复中间件（`internal/httpapi/middleware.go`）、请求体解析与错误响应写出辅助函数（`internal/httpapi/handlers.go`）、`main` 的函数装配与优雅退出、`GET /api/healthz`、全部接口契约的响应结构体与转换函数 | 已写好，你不需要修改 |
| 业务逻辑（由你实现） | 短码生成 1 个函数、存储层 7 个方法、缓存层 8 个方法、处理函数 6 个、后台写回 2 个方法，合计 24 个函数 | 已全部填写完成（第 4 组至第 8 组于 2026-09-30 完成），并且通过第 10 节的全量验收 |
| 分层验证（已写好） | `internal/shortcode/shortcode_test.go`、`internal/store/postgres_test.go`、`internal/cache/redis_test.go`、`internal/httpapi/router_test.go` | 已写好，直接运行即可。前三个需要依赖服务或者环境变量，路由测试不需要任何依赖 |

### 0.2 未填写的函数在被调用时会怎样

未填写的函数会返回一条形如 `尚未实现：store.Postgres.CreateLink，请完成任务清单的第 2 组任务` 的错误，
对应的处理函数会返回 HTTP 状态码 501。也就是说，这个工程从第一步开始就是可编译、可启动、可观察的，
你不需要等到全部写完才能看到结果。

### 0.3 任务分组与代码位置

| 组号 | 主题 | 代码位置 | 函数数量 |
|---|---|---|---|
| 1 | 短码生成 | `internal/shortcode/shortcode.go` | 1 |
| 2 | 存储层查询 | `internal/store/postgres.go` | 7 |
| 3 | 缓存层读写 | `internal/cache/redis.go` | 8 |
| 4 | 创建与列表接口 | `internal/httpapi/handlers.go` | 2 |
| 5 | 跳转与点击计数 | `internal/httpapi/handlers.go` | 1 |
| 6 | 统计与删除接口 | `internal/httpapi/handlers.go` | 2 |
| 7 | 就绪探针 | `internal/httpapi/handlers.go` | 1 |
| 8 | 后台点击写回 | `internal/flusher/flusher.go` | 2 |

建议的完成顺序就是 1 到 8。第 1 组与第 2 组可以独立验收，第 3 组之后的验收依赖前面几组，
因此不要跳过。

### 0.4 两份配套文档

动手之前先读下面两份文档，它们的作用是把「系统怎么运转」与「Go 语法怎么写」这两层认知补上，
避免在填写函数时同时面对两种未知。

| 文档 | 内容 | 什么时候查阅 |
|---|---|---|
| `BACKEND-OVERVIEW.md` | 六个包的职责边界与依赖方向、四条主路径的完整流转、接口契约与代码位置的对应关系、建议的代码阅读顺序 | 动手之前通读一遍；填写某个函数不确定它在系统里的位置时回查第 4 节 |
| `GO-CHEATSHEET.md` | 本工程用到的全部 Go 语法与标准库功能，按包与导入、结构体与方法、错误处理、并发、测试等十六个主题编排，其中第 11 节讲 gin 的路由注册，第 13 节是 gin 与标准库 `net/http` 的逐项对照，第 14 节讲 `crypto/rand` 与 `math/big` 这两个包，第 15 节讲 pgx 与 PostgreSQL 的 SQL 语句与特性，第 16 节讲 Redis 与 go-redis | 写代码时遇到「这个写法是什么意思」或者「这个包怎么用」时按主题查阅 |
| `GIN-WALKTHROUGH.md` | gin 处理一次请求的完整流转、一个可以直接运行的完整示例程序、按用途分组的 gin API 清单、四种响应写出方式的对照、本项目七个处理函数与 gin API 的对应关系、`handleCreateLink` 的调用序列、易错点清单与练习 | 第 4 组到第 7 组填写 HTTP 层代码之前通读一遍；忘记 `c.Param`、`c.DefaultQuery`、`c.Redirect`、`c.Status`、`c.Next`、`c.Abort` 这些方法的用法时按第 3 节的清单查阅 |

### 0.5 常用命令的快捷入口（Makefile）

`api/` 目录下有一个 `Makefile`，它把常用的命令包装成短目标，减少手工输入长命令时出现差错的可能。
用法是在 `api/` 目录执行 `make 目标名`；不带参数执行 `make` 会打印全部可用目标。

| 目标 | 等价于的命令 | 什么时候使用 |
|---|---|---|
| `make help` | 无，由 Makefile 自己实现 | 忘记目标名称时 |
| `make build` | `go build ./...` | 每次改完代码之后的第一道检查 |
| `make vet` | `go vet ./...` | 编译通过之后 |
| `make fmt` | `gofmt -w .` | 一次修改了很多文件之后 |
| `make fmt-check` | `gofmt -l .` | 提交之前确认格式 |
| `make test` | `go test ./...` | 一组任务完成之后的整体确认 |
| `make test-unit` | `go test ./internal/shortcode/... ./internal/httpapi/... -v` | 第 1 组的验收，不需要任何外部依赖 |
| `make test-store` | 带上 `TEST_DATABASE_URL` 运行存储层测试 | 第 2 组的验收 |
| `make test-cache` | 带上 `TEST_REDIS_ADDR` 运行缓存层测试 | 第 3 组的验收 |
| `make test-all` | 带上两个环境变量运行全部测试 | 八组全部完成之后的总体验收 |
| `make run` | `go run ./cmd/api` | 启动服务；终端会被占用，按 `Ctrl+C` 停止 |
| `make tidy` | `go mod tidy` | 新增了导入之后 |
| `make clean` | 删除编译产物并清空测试缓存 | 测试结果一直显示 `cached` 时 |
| `make compose-ps` | 在上一级目录执行 `docker compose ps` | 每次开始工作之前确认依赖服务在运行 |
| `make compose-up` | 在上一级目录执行 `docker compose up -d` | 依赖服务没有运行时 |
| `make compose-down` | 在上一级目录执行 `docker compose down` | 需要彻底释放端口时 |
| `make verify` | 依次执行 build、vet、fmt-check、test-unit | 一组任务完成之后一次跑完全部检查 |

关于这份 Makefile 有三点需要知道。

| 事项 | 说明 |
|---|---|
| 配方行必须使用制表符 | Makefile 的语法要求命令那一行以制表符开头，使用空格会报 `missing separator` 并且拒绝执行。修改这个文件时必须保持制表符缩进 |
| 变量可以在命令行覆盖 | 例如 `make run HTTP_ADDR=:9090` 会用 9090 端口启动服务；`make test-store TEST_DATABASE_URL=另一个连接串` 会覆盖默认的连接串 |
| `GOPROXY` 已经在文件内部设置 | Makefile 把它导出给每一条命令，因此使用 `make` 时不需要手动执行 `export GOPROXY`；但直接使用 `go` 命令时仍然需要设置它 |
| 默认会先把实际执行的命令打印出来 | 这是 make 的默认行为，作用是让人看到这个目标到底执行了什么。执行 `make build` 时会先看到一行 `go build ./...` 再看到结果，两者之间的关系一目了然 |

---

## 1. 第 0 组 准备与首次启动

### 1.1 命令与预期结果

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 进入后端目录 | `cd /home/drow/k8s_proj/code/shortener/api` | 当前目录切换成功 |
| 设置模块代理 | `export GOPROXY=https://goproxy.cn,direct` | 无输出。这一步是必须的，因为 `proxy.golang.org` 在这台机器上不可达 |
| 整理依赖 | `go mod tidy` | 无输出。执行之后 `go.mod` 中出现 `require github.com/jackc/pgx/v5 v5.11.0` 与 `require github.com/redis/go-redis/v9 v9.22.0`，同时生成 `go.sum` |
| 编译 | `go build ./...` | 无输出。有输出就说明编译失败，先按输出的行号修正 |
| 静态检查 | `go vet ./...` | 无输出 |
| 启动服务 | `go run ./cmd/api` | 终端输出本工程的六行启动日志；`GIN_MODE` 取值为 debug 时中间还会插入一段 gin 的路由清单；最后一行是 `HTTP 服务开始监听`，然后进程保持运行 |

启动日志的实际形态如下（这是本机实测的输出，字段取值与你的环境一致）。

上面这些命令都有对应的 `make` 短目标，对照表见 0.5 节。使用 `make` 执行时不需要手动导出 `GOPROXY`，因为 Makefile 内部已经把它导出了。

```text
level=INFO msg=配置读取完成 摘要="监听地址=:8080 Redis地址=localhost:6379 Redis库编号=0 缓存生存时间=5m0s 点击写回周期=5s 优雅退出时限=10s"
level=INFO msg="已连接 PostgreSQL" 主机=localhost 数据库=shortener
level=INFO msg=数据表已就绪
level=INFO msg="已连接 Redis" 地址=localhost:6379
level=WARN msg=后台点击写回协程尚未实现 提示="尚未实现：flusher.Flusher.Run，请完成任务清单的第 8 组任务"
level=INFO msg="HTTP 服务开始监听" 地址=:8080
```

### 1.2 环境变量

不设置任何环境变量也可以启动，因为 `.env.example` 中记录的默认值与 `compose.yaml` 中的
PostgreSQL 与 Redis 完全一致。需要覆盖默认值时，先执行 `cp .env.example .env`，
编辑 `.env`，再用 `set -a; source .env; set +a` 把取值导出到当前 shell。

| 变量 | 默认值 | 作用 | 什么时候需要修改 |
|---|---|---|---|
| `HTTP_ADDR` | `:8080` | HTTP 服务的监听地址 | 8080 端口被占用时 |
| `DATABASE_URL` | `postgres://shortener:shortener_dev_password@localhost:5432/shortener?sslmode=disable` | PostgreSQL 连接串 | 数据库不在本机，或者用户名与口令不同时 |
| `REDIS_ADDR` | `localhost:6379` | Redis 地址 | Redis 不在本机时 |
| `REDIS_PASSWORD` | 空字符串 | Redis 口令 | Redis 开启了 `requirepass` 时 |
| `REDIS_DB` | `0` | Redis 逻辑数据库编号 | 同一个 Redis 实例被多个项目共用时 |
| `CACHE_TTL` | `300s` | 缓存键的生存时间 | 演示缓存过期与未命中回落时，改成 `10s` 可以让现象出现得更快 |
| `CLICK_FLUSH_INTERVAL` | `5s` | 点击增量写回周期 | 演示写合并时，改成 `30s` 可以让 `clicks:{code}` 键中的增量累积得更明显 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅退出的时限 | 一般不需要修改 |

### 1.3 首次启动之后的验证

服务保持运行，另开一个终端执行下表的命令。下表的返回码是本机实测结果。

| 请求 | 命令 | 当前预期返回码 | 全部完成之后的预期返回码 |
|---|---|---|---|
| 存活检查 | `curl -i http://localhost:8080/api/healthz` | 200，响应体是 `{"status":"ok"}` | 200，不变 |
| 就绪检查 | `curl -o /dev/null -w '%{http_code}\n' http://localhost:8080/api/readyz` | 501 | 200 或者 503 |
| 列表接口 | `curl -o /dev/null -w '%{http_code}\n' http://localhost:8080/api/links` | 501 | 200 |
| 未注册路径 | `curl -o /dev/null -w '%{http_code}\n' http://localhost:8080/api/links/xyz` | 405 | 405，不变 |
| 短码跳转 | `curl -o /dev/null -w '%{http_code}\n' http://localhost:8080/a1B2c3` | 501 | 302 或者 404 |
| 统计接口 | `curl -o /dev/null -w '%{http_code}\n' http://localhost:8080/api/stats` | 501 | 200 |

> 关于 `/api/links/xyz` 返回 405 这件事：这个路径被 `DELETE /api/links/:code` 这条路由匹配，
> 但是请求方法不是 `DELETE`，因此返回 405（方法不被允许）而不是 404（路径不存在）。
> 这个行为由 `Routes()` 中打开的 `engine.HandleMethodNotAllowed` 决定，
> 该项的默认取值是 false，此时 gin 返回 404；本工程打开它，是为了让行为符合 HTTP 语义。
> 需要留意的是这个响应由 gin 直接写出，响应体是 `405 method not allowed`，
> 不经过本工程的 `writeError` 辅助函数，因此它的结构与契约中的 `{"error":"..."}` 不同，
> 这是框架层产生的响应与业务层产生的响应之间的一处差异。
>
> 关于本机终端的一处现象：这台机器的终端输出偶尔出现串扰，表现为同一批命令的运行结果混排，
> 或者服务日志中的请求路径末尾多出一个分号。遇到无法解释的状态码时，
> 先重新执行一次同一条命令再判断，并且把结果用 `>> 文件名` 重定向到文件之后再读取，
> 这样可以排除终端渲染造成的影响。

### 1.4 停止服务

在服务所在的终端按 `Ctrl+C`。预期看到 `收到停止信号，开始优雅退出` 与 `服务已停止` 两行日志。
这两行日志证明优雅退出流程生效，第 8 组完成之后这里还会多出一次收尾的写回动作。

---

## 2. 第 1 组 短码生成

### 2.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的函数 | `shortcode.Generate` |
| 代码位置 | `internal/shortcode/shortcode.go` |
| 依赖的外部数据 | 无 |
| 它在系统中的位置 | 处理函数 `handleCreateLink` 在写入数据库之前调用它，为每一条新记录生成主键 |

### 2.2 实现要求

要求已经写在函数的注释里，核心有三条：长度必须是 6、每个字符都必须来自 `Alphabet`、
随机源必须使用 `crypto/rand` 而不是 `math/rand`。关于第三条的两层原因，注释中有完整说明。

需要你自行添加的导入是 `crypto/rand` 与 `math/big`。

### 2.3 验收

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 运行单元测试 | `go test ./internal/shortcode/... -v` | 三个测试全部通过，输出以 `ok  shortener/internal/shortcode` 结尾 |
| 编译整个工程 | `go build ./...` | 无输出 |

测试文件里已经包含三条检查：长度与字符集、200 次生成不出现重复、500 次生成之后字符集被完整覆盖。

### 2.4 需要写进笔记的现象

完成本组之后，请把下面两个问题的答案写进当天笔记，它们是面试中常见的追问。

| 问题 | 需要说清的机制 |
|---|---|
| 为什么不用 `math/rand` | `math/rand` 的默认随机源在未设置种子时每次启动产生相同序列，因此重建 Pod 之后短码会重复；并且它的序列可以被推导，公开的短码因此可以被批量预测与占位 |
| 为什么不能把随机数与取模运算组合使用 | 当随机数的取值范围不能被 62 整除时，取模之后的分布不均匀，位置靠前的字符被选中的概率偏高，这就是取模偏差 |

---

## 3. 第 2 组 存储层查询

### 3.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的方法 | `CreateLink`、`GetLink`、`ListLinks`、`CountLinks`、`SumClicks`、`AddClicks`、`DeleteLink` |
| 代码位置 | `internal/store/postgres.go` |
| 依赖的外部数据 | PostgreSQL 中的 `links` 表，由已经写好的 `EnsureSchema` 在启动时创建 |
| 它在系统中的位置 | 全部持久化数据的唯一出入口。缓存层的未命中回落、点击增量的写回、统计接口的计数都经过这里 |

### 3.2 七个方法各自在系统中的位置

| 方法 | 被谁调用 | 它的职责边界 |
|---|---|---|
| `CreateLink` | 第 4 组的 `handleCreateLink` | 只插入记录；短码的生成与合法性校验不属于本方法 |
| `GetLink` | 第 5 组的 `handleRedirect` | 只读取记录；是否要写回缓存由调用方决定 |
| `ListLinks` | 第 4 组的 `handleListLinks` | 只负责分页与排序；`limit` 的取值范围校验由调用方完成 |
| `CountLinks` | 第 6 组的 `handleStats` | 只返回记录总数 |
| `SumClicks` | 第 6 组的 `handleStats` | 只返回 `clicks` 列的合计 |
| `AddClicks` | 第 8 组的写回协程 | 只做累加；不做增量是否合理的判断 |
| `DeleteLink` | 第 6 组的 `handleDeleteLink` | 只删除数据库记录；缓存键的删除由调用方负责 |

### 3.3 验收

本层没有 HTTP 接口可以调用，因此验收通过集成测试完成。测试文件已经写好。

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 确认数据表已创建 | `docker exec shortener-postgres psql -U shortener -d shortener -c '\d links'` | 输出 `links` 表的列定义与两个索引，其中包括主键 `links_pkey` 与 `links_created_at_idx` |
| 运行存储层测试 | `TEST_DATABASE_URL='postgres://shortener:shortener_dev_password@localhost:5432/shortener?sslmode=disable' go test ./internal/store/... -v` | 九个测试全部通过 |
| 查看测试写入的记录是否被清理 | `docker exec shortener-postgres psql -U shortener -d shortener -c "select count(*) from links where code like 'zzstore%'"` | `count` 列的取值是 `0` |

测试覆盖的九个场景是：插入之后按短码读出、默认值 `clicks=0` 与 `created_at` 由数据库写入、
查询不存在的短码返回 `ErrNotFound`、重复插入返回 SQLSTATE `23505`、列表按创建时间倒序、
逐条分页的结果与整体顺序一致、空结果返回长度为零的切片而不是 `nil`、
总数与点击总数的增量正确、累加是叠加的、删除之后再次删除返回 `ErrNotFound`。

### 3.4 需要写进笔记的机制

| 问题 | 需要说清的机制 |
|---|---|
| 为什么列表排序要追加 `code` 作为次级条件 | 若干条记录的 `created_at` 完全相同时，仅按 `created_at` 排序的顺序不确定，相邻两页可能出现同一条记录或者漏掉一条记录 |
| 为什么空结果必须返回长度为零的切片 | `nil` 切片会被 `encoding/json` 序列化成 `null`，而接口契约规定 `items` 字段始终是数组，前端直接依赖这一点 |
| 为什么点击累加交给数据库执行 | 在 Go 里执行「读取、相加、写回」三步之间存在时间窗口，并发写回会互相覆盖，从而丢失增量 |

---

## 4. 第 3 组 缓存层读写

### 4.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的方法 | `GetURL`、`SetURL`、`DeleteLink`、`IncrClick`、`RecordCacheResult`、`CacheStats`、`CollectClicks`、`SubtractClicks` |
| 代码位置 | `internal/cache/redis.go` |
| 依赖的外部数据 | Redis 中的四类键，定义见 `README.md` 第 5 节 |
| 它在系统中的位置 | 两个互不相干的职责共用同一个客户端：读缓存（降低重定向对数据库的读压力）与写缓冲（把高频自增合并成低频更新） |

### 4.2 八个方法各自在系统中的位置

| 方法 | 被谁调用 | 它使用或影响的键 |
|---|---|---|
| `GetURL` | 第 5 组的 `handleRedirect` | 读 `link:{code}` |
| `SetURL` | 第 5 组的 `handleRedirect`，用于读时回填 | 写 `link:{code}` |
| `DeleteLink` | 第 6 组的 `handleDeleteLink` | 删除 `link:{code}` 与 `clicks:{code}` |
| `IncrClick` | 第 5 组的 `handleRedirect` | 写 `clicks:{code}` |
| `RecordCacheResult` | 第 5 组的 `handleRedirect` | 写 `stats:cache_hits` 或者 `stats:cache_misses` |
| `CacheStats` | 第 6 组的 `handleStats` | 读两个统计键 |
| `CollectClicks` | 第 8 组的写回协程 | 扫描并读取 `clicks:*` |
| `SubtractClicks` | 第 8 组的写回协程 | 写 `clicks:{code}` |

### 4.3 验收

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 运行缓存层测试 | `TEST_REDIS_ADDR=localhost:6379 go test ./internal/cache/... -v` | 七个测试全部通过 |
| 确认测试使用的键已被清理 | `docker exec shortener-redis redis-cli KEYS 'zzcache*'` | 输出为空 |
| 观察键的生存时间 | 先执行 `docker exec shortener-redis redis-cli SET link:testttl https://example.com EX 300`，再执行 `docker exec shortener-redis redis-cli TTL link:testttl` | `TTL` 的输出是 300 上下的整数，说明生存时间生效 |
| 清理上一步手工写入的键 | `docker exec shortener-redis redis-cli DEL link:testttl` | 输出 `1` |

> 关于统计键的两个测试：它们会先清空 `stats:cache_hits` 与 `stats:cache_misses`，
> 因此运行之前请确认后端进程没有在同时处理请求，否则并发写入会让断言失败。

### 4.4 需要写进笔记的机制

| 问题 | 需要说清的机制 |
|---|---|
| 为什么枚举键必须使用 `SCAN` 而不是 `KEYS` | `KEYS` 在一次调用中遍历整个键空间，遍历期间会阻塞其他命令的执行；键的数量较多时 Redis 会停止响应，而 `SCAN` 采用游标分批返回，单次调用只遍历一部分键 |
| 为什么写回之后使用减法而不是删除键 | 扣除之前键里可能已经累积了新的增量，这些增量来自扣除动作执行期间到达的请求，直接删除会把它们一起丢掉 |
| 为什么统计键不设置生存时间 | 命中率需要长期累计才有意义；代价是 Redis 重启（并且没有开启持久化）之后这两个键会被清零，命中率从零重新累计 |

---

## 5. 第 4 组 创建与列表接口

### 5.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的函数 | `handleCreateLink`、`handleListLinks` |
| 代码位置 | `internal/httpapi/handlers.go` |
| 依赖的函数 | 第 1 组的 `shortcode.Generate`、第 2 组的 `CreateLink`、`ListLinks`、`CountLinks` |
| 它在系统中的位置 | 前端页面的两个主要动作：提交长网址与查看列表 |

### 5.2 错误分支与状态码

本组涉及的错误分支最多，下表把它们集中列出，便于逐条核对。

| 触发条件 | 状态码 | 响应体 |
|---|---|---|
| 请求体不是合法的 JSON | 400 | `{"error":"请求体不是合法的 JSON"}` |
| `url` 缺失或者为空字符串 | 400 | `{"error":"url 字段不能为空"}` |
| `url` 不以 `http://` 或 `https://` 开头 | 400 | `{"error":"url 字段必须以 http:// 或 https:// 开头"}` |
| `url` 的字节长度超过 2048 | 400 | `{"error":"url 字段长度超过 2048"}` |
| 连续 5 次生成的短码都与已有记录冲突 | 500 | `{"error":"服务器内部错误"}` |
| 数据库访问出错 | 503 | `{"error":"依赖服务不可用"}` |
| 列表接口的 `limit` 不是合法整数，或者小于 1，或者大于 100 | 400 | 自定义的错误文本 |
| 列表接口的 `offset` 不是合法整数，或者小于 0 | 400 | 自定义的错误文本 |

### 5.3 验收

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 创建一条记录 | `curl -i -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -d '{"url":"https://example.com/very/long/path"}'` | 201，响应体中包含 `code`、`url`、`clicks`（取值 0）、`createdAt` 四个字段 |
| 请求体不是合法 JSON | `curl -i -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -d 'not-json'` | 400，错误文本是「请求体不是合法的 JSON」 |
| `url` 字段缺失 | `curl -i -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -d '{}'` | 400，错误文本是「url 字段不能为空」 |
| 协议不符合要求 | `curl -i -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -d '{"url":"ftp://example.com"}'` | 400，错误文本是「url 字段必须以 http:// 或 https:// 开头」 |
| 列表接口默认分页 | `curl -s http://localhost:8080/api/links` | 200，响应体中的 `limit` 是 20、`offset` 是 0、`items` 是数组、`total` 是当前记录总数 |
| 列表接口指定分页 | `curl -s 'http://localhost:8080/api/links?limit=1&offset=0'` | 200，`items` 的长度是 1 |
| `limit` 超出上限 | `curl -i 'http://localhost:8080/api/links?limit=101'` | 400 |
| 数据落库确认 | `docker exec shortener-postgres psql -U shortener -d shortener -c 'select code, url, clicks from links order by created_at desc limit 3'` | 输出中可以看到刚创建的三条记录，`clicks` 列的取值都是 0 |

### 5.4 需要写进笔记的机制

| 问题 | 需要说清的机制 |
|---|---|
| 为什么要限制 `limit` 的上限 | 不限制时一次请求可以把整张表读出来，数据量增长之后这一次查询会占用大量内存与数据库时间，并且把结果压进一次响应，属于典型的资源耗尽入口 |
| 为什么短码冲突要重试而不是直接报错 | 短码由随机数生成，冲突的概率等于记录数除以 62 的 6 次方，正常情况下极小；重试机制处理的是概率事件，超出重试次数才说明存在更严重的问题 |
| 为什么长度校验的单位是字节而不是字符 | Go 中 `len` 返回字节数，含中文的网址每个汉字占 3 个字节，因此同一条网址按字符计算没有超限、按字节计算可能超限。这是刻意保留在契约里的表述，你需要在笔记中记录这个差异 |

---

## 6. 第 5 组 跳转与点击计数

### 6.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的函数 | `handleRedirect` |
| 代码位置 | `internal/httpapi/handlers.go` |
| 依赖的函数 | 第 3 组的 `GetURL`、`SetURL`、`IncrClick`、`RecordCacheResult`；第 2 组的 `GetLink` |
| 它在系统中的位置 | 整个系统的核心路径：浏览器访问短网址到跳转到原网址的完整过程，同时也是缓存与点击计数两个机制唯一被触发的位置 |

### 6.2 本组的执行顺序

执行顺序决定了故障发生时系统的表现，因此顺序本身就是实现要求的一部分。

| 序号 | 动作 | 输入 | 产生的状态变化 | 失败时的处理 |
|---|---|---|---|---|
| 1 | 校验短码格式 | 路径参数 `code` | 无 | 返回 404，不访问任何依赖 |
| 2 | 查询缓存 | `code` | `stats:cache_hits` 加一（命中时） | 缓存不可用时返回 503 |
| 3 | 记录未命中 | `code` | `stats:cache_misses` 加一（未命中时） | 同上 |
| 4 | 回落数据库查询 | `code` | 无 | 记录不存在返回 404；依赖出错返回 503 |
| 5 | 回填缓存 | `code` 与查到的长网址 | 新增键 `link:{code}`，生存时间 300 秒 | 失败时只记录日志，不影响跳转 |
| 6 | 写出跳转响应 | 长网址 | 客户端收到 302 与 `Location` 响应头 | 无 |
| 7 | 累加点击增量 | `code` | `clicks:{code}` 加一 | 失败时只记录日志，不影响跳转 |

第 5 步与第 7 步的失败之所以不影响跳转，是因为用户此时已经拿到了目标地址，
统计数据的丢失比让用户看到错误页面更容易接受。这个取舍需要在笔记中说明。

### 6.3 验收

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 取一个真实短码 | 用第 4 组创建接口返回的 `code` 替换下面命令中的 `a1B2c3` | 无 |
| 第一次跳转（缓存未命中） | `curl -i http://localhost:8080/a1B2c3` | 302，`Location` 响应头是原始长网址 |
| 确认缓存已经回填 | `docker exec shortener-redis redis-cli GET link:a1B2c3` | 输出原始长网址 |
| 确认生存时间已设置 | `docker exec shortener-redis redis-cli TTL link:a1B2c3` | 输出接近 300 的整数 |
| 第二次跳转（缓存命中） | `curl -i http://localhost:8080/a1B2c3` | 302，仍然是同一条长网址 |
| 确认点击增量已记录 | `docker exec shortener-redis redis-cli GET clicks:a1B2c3` | 输出 `2`（两次跳转各加一） |
| 确认命中与未命中被分别统计 | `docker exec shortener-redis redis-cli MGET stats:cache_hits stats:cache_misses` | 两次跳转之后命中数明显多于未命中数 |
| 短码不存在 | `curl -i http://localhost:8080/zzzzzz` | 404，响应头的 `Content-Type` 是 `text/html`，响应体是一段说明短码不存在的 HTML |
| 刻意用浏览器验证 | 在浏览器地址栏直接打开 `http://localhost:8080/a1B2c3` | 浏览器跳转到原始长网址 |

### 6.4 需要写进笔记的机制

| 问题 | 需要说清的机制 |
|---|---|
| 为什么使用 302 而不是 301 | 301 表示永久重定向，会被浏览器长期缓存，之后即使数据库中的记录被删除，浏览器仍然会直接跳转而不再访问本服务，删除操作对用户就失效了 |
| 为什么回填缓存要做在跳转之前 | 回填的耗时在 1 毫秒上下，而它换来的收益是后续同一短码的查询都不再访问数据库；如果把回填放进后台任务，回填失败时不会有任何提示，缓存会长期不生效 |
| 为什么直接访问后端端口时 `favicon.ico` 会命中跳转处理函数 | 路由模式 `GET /{code}` 匹配任意单个路径片段，因此 `/favicon.ico` 与 `/api` 都会被它匹配；生产形态下 nginx 只转发六位短码路径，所以这条校验的作用是在直接访问后端时保持行为正确 |

---

## 7. 第 6 组 统计与删除接口

### 7.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的函数 | `handleStats`、`handleDeleteLink` |
| 代码位置 | `internal/httpapi/handlers.go` |
| 依赖的函数 | 第 2 组的 `CountLinks`、`SumClicks`、`DeleteLink`；第 3 组的 `CacheStats`、`DeleteLink` |
| 它在系统中的位置 | 前端统计条与删除按钮所对应的两个接口 |

### 7.2 验收

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 查看统计 | `curl -s http://localhost:8080/api/stats` | 200，响应体中包含 `links`、`clicks`、`cacheHits`、`cacheMisses`、`cacheHitRate` 五个字段 |
| 确认命中率的计算 | 把上一步的 `cacheHits` 除以 `cacheHits` 与 `cacheMisses` 之和，与 `cacheHitRate` 比较 | 两者一致；两个统计键都被清空时 `cacheHitRate` 是 0 而不是 `NaN` |
| 删除一条记录 | `curl -i -X DELETE http://localhost:8080/api/links/a1B2c3` | 204，响应体为空 |
| 确认缓存键已删除 | `docker exec shortener-redis redis-cli EXISTS link:a1B2c3 clicks:a1B2c3` | 输出 `0`，表示两个键都不存在 |
| 删除之后再次跳转 | `curl -i http://localhost:8080/a1B2c3` | 404，说明缓存与数据库都已删除 |
| 删除不存在的短码 | `curl -i -X DELETE http://localhost:8080/api/links/zzzzzz` | 404，错误文本是「短码不存在」 |
| 确认记录数减少 | `docker exec shortener-postgres psql -U shortener -d shortener -c 'select count(*) from links'` | 计数比删除之前少 1 |

### 7.3 需要写进笔记的机制

| 问题 | 需要说清的机制 |
|---|---|
| 为什么命中率的分母为 0 时必须单独判断 | 分母为 0 时除法结果是 `NaN`，`encoding/json` 无法把 `NaN` 序列化成合法的 JSON，响应写出会失败，表现为接口返回错误而不是返回 0 |
| 为什么删除的顺序是先数据库后缓存 | 反过来执行时，如果数据库删除失败，缓存中的记录已经消失，后续请求会回落到数据库并把这条记录重新填充进缓存，表现为删除操作看起来没有生效 |
| 为什么缓存删除失败不影响响应状态码 | 缓存键带有生存时间，删除失败也会自动过期，而数据库中的记录已经删除，数据最终是一致的；此时记录警告日志比让请求失败更合适 |

---

## 8. 第 7 组 就绪探针

### 8.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的函数 | `handleReadyz` |
| 代码位置 | `internal/httpapi/handlers.go` |
| 依赖的函数 | 第 2 组的 `Ping`（已写好）、第 3 组的 `Ping`（已写好） |
| 它在系统中的位置 | 第 4 周 Kubernetes 就绪探针的检查端点。就绪探针的作用是决定 Pod 是否接收流量，因此它检查的必须是「依赖是否可用」，而不是「进程是否存活」 |

本组是第 3 周与第 4 周之间的连接点：存活探针与就绪探针的职责划分在这里被实现出来，
第 4 周写 Deployment 清单时，两个探针分别指向 `/api/healthz` 与 `/api/readyz`。

### 8.2 验收

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 依赖都正常时 | `curl -i http://localhost:8080/api/readyz` | 200，响应体是 `{"status":"ready","checks":{"postgres":"ok","redis":"ok"}}` |
| 停止 Redis | `docker compose stop redis` | `docker compose ps` 中 redis 服务不再运行 |
| 依赖缺失时 | `curl -i http://localhost:8080/api/readyz` | 503，`status` 字段是 `not ready`，`checks` 中 `redis` 的取值是具体的错误原因，而 `postgres` 仍然是 `ok` |
| 确认响应时间受控 | `curl -o /dev/null -w '%{time_total}\n' http://localhost:8080/api/readyz` | 输出小于 3 秒，说明检查使用的超时时间生效 |
| 恢复 Redis | `docker compose start redis` | 等待数秒之后再次请求，`/api/readyz` 恢复为 200 |
| 确认存活探针不受影响 | `docker compose stop redis` 之后执行 `curl -i http://localhost:8080/api/healthz` | 200，响应体是 `{"status":"ok"}`。这一步证明两个探针的职责划分正确 |

### 8.3 需要写进笔记的机制

| 问题 | 需要说清的机制 |
|---|---|
| 为什么就绪检查必须带超时 | 依赖处于「连接可以建立但是不响应」的状态时，不带超时的检查会一直等待，就绪探针每次都要等到 kubelet 侧超时才有结果，kubelet 因此会重启容器，反而制造出额外故障 |
| 为什么存活探针不检查依赖 | 存活探针失败会导致容器被重启，而重启不能修复依赖故障，只会让「依赖故障」这个原因被「容器不断重启」这个表象掩盖，排障时更难定位 |
| 为什么 PostgreSQL 检查失败时仍然要检查 Redis | `checks` 字段同时返回两个依赖的状态，排障时一次就能看清全部原因，而不是修好一个之后才发现另一个也有问题 |

---

## 9. 第 8 组 后台点击写回

### 9.1 目标与位置

| 项目 | 内容 |
|---|---|
| 需要实现的方法 | `Flusher.Run`、`Flusher.FlushOnce` |
| 代码位置 | `internal/flusher/flusher.go` |
| 依赖的函数 | 第 3 组的 `CollectClicks`、`SubtractClicks`；第 2 组的 `AddClicks` |
| 它在系统中的位置 | 写合并机制的实现部分：把重定向路径上的高频自增合并成数据库上的低频更新 |

### 9.2 验收

本组的验收需要在两个终端之间交替进行，一个终端运行服务，另一个终端执行命令。

| 动作 | 命令 | 预期结果 |
|---|---|---|
| 创建一条记录 | `curl -s -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -d '{"url":"https://example.com/flush/test"}'` | 201，记下响应体中的 `code`，下面用 `a1B2c3` 表示 |
| 连续跳转 10 次 | `for i in $(seq 1 10); do curl -s -o /dev/null "http://localhost:8080/a1B2c3"; done` | 无输出 |
| 立刻查看数据库中的点击数 | `docker exec shortener-postgres psql -U shortener -d shortener -c "select code, clicks from links where code = 'a1B2c3'"` | `clicks` 列的取值是 0 或者明显小于 10，说明写入被缓冲在 Redis 中 |
| 立刻查看 Redis 中的增量 | `docker exec shortener-redis redis-cli GET clicks:a1B2c3` | 输出接近 10 的整数 |
| 等待一个写回周期之后查看数据库 | 等待 5 秒以上再执行上一条查询数据库的命令 | `clicks` 列的取值变成 10，说明写回生效 |
| 确认增量已被扣除 | `docker exec shortener-redis redis-cli GET clicks:a1B2c3` | 输出 0 或者该键已经不存在 |
| 确认计数没有丢失 | 把跳转次数改成 100 次并重复上面三个步骤 | 数据库中的 `clicks` 最终等于 100，Redis 中的增量归零 |
| 验证优雅退出时的收尾写回 | 跳转 5 次之后立刻在服务终端按 `Ctrl+C` | 日志中出现写回动作；重新启动服务之后查询数据库，`clicks` 已包含那 5 次 |

### 9.3 需要写进笔记的机制

| 问题 | 需要说清的机制 |
|---|---|
| 为什么一轮写回失败之后要继续循环而不是退出 | PostgreSQL 短暂不可用是很常见的情况，退出循环会让点击计数从此永久停止写回；继续循环可以在依赖恢复之后自动接上，代价是失败期间的增量暂时留在 Redis 中 |
| 为什么必须先写数据库再扣除增量 | 顺序颠倒时，数据库更新失败会导致已经扣除的增量永久丢失，而本次讨论的边界条件「后端在扣除增量之前被杀死」只会造成重复写回，两者的后果不同 |
| 写合并的代价是什么 | 后端在扣除增量之前被强制杀死时，这一轮的增量会留在 Redis 中，下次启动之后被重新写回，因此数据库中可能出现偏高一次的计数；这个代价换取的是重定向的响应时间不再受数据库写入延迟影响 |

---

## 10. 全量验收（八组全部完成之后执行）

依次执行下表命令，全部符合预期即代表阶段 2 完成。下表是阶段 2 的完成标志。

| 序号 | 动作 | 命令 | 预期结果 |
|---|---|---|---|
| 1 | 编译与静态检查 | `go build ./... && go vet ./...` | 无输出 |
| 2 | 全部单元与集成测试 | `TEST_DATABASE_URL='postgres://shortener:shortener_dev_password@localhost:5432/shortener?sslmode=disable' TEST_REDIS_ADDR=localhost:6379 go test ./...` | 全部包输出 `ok` |
| 3 | 启动服务 | `go run ./cmd/api` | 输出本工程的六行启动日志与 gin 的路由清单，没有 `WARN` 级别的「尚未实现」提示 |
| 4 | 存活检查 | `curl -i http://localhost:8080/api/healthz` | 200 与 `{"status":"ok"}` |
| 5 | 就绪检查 | `curl -i http://localhost:8080/api/readyz` | 200 与 `{"status":"ready","checks":{"postgres":"ok","redis":"ok"}}` |
| 6 | 创建 | `curl -i -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -d '{"url":"https://example.com/final/check"}'` | 201，四个字段齐全 |
| 7 | 跳转 | `curl -i http://localhost:8080/<第 6 步返回的 code>` | 302 与正确的 `Location` |
| 8 | 列表 | `curl -s 'http://localhost:8080/api/links?limit=5&offset=0'` | 200，`items` 中第一条是第 6 步创建的记录 |
| 9 | 统计 | `curl -s http://localhost:8080/api/stats` | 200，`links` 与第 8 步的 `total` 一致，`cacheHitRate` 在 0 与 1 之间 |
| 10 | 删除 | `curl -i -X DELETE http://localhost:8080/api/links/<第 6 步返回的 code>` | 204 |
| 11 | 删除之后跳转 | `curl -i http://localhost:8080/<第 6 步返回的 code>` | 404 |
| 12 | 前端联调 | 在 `web/` 目录执行 `npm install`，再执行 `npm run dev`，浏览器打开 `http://localhost:5173` | 页面顶部显示「后端就绪」，可以创建、跳转、删除，统计条随之变化 |

第 12 步是阶段 2 与阶段 3 之间的衔接：前端在开发模式下通过 Vite 的转发规则访问后端，
因此这一条通过之后，阶段 3 只需要处理容器化，不需要再修改任何业务代码。

---

## 11. 常见错误对照表

| 现象 | 原因 | 定位方式 |
|---|---|---|
| `no required module provides package` | 没有执行 `go mod tidy`，或者 `GOPROXY` 没有设置 | 执行 `go mod tidy`，确认已经执行过 `export GOPROXY=https://goproxy.cn,direct` |
| `go: module lookup disabled` 或者下载超时 | `GOPROXY` 指向默认地址，`proxy.golang.org` 在这台机器上不可达 | 执行 `go env GOPROXY` 查看当前取值，重新导出 `GOPROXY=https://goproxy.cn,direct` |
| 接口返回 501 并且响应体是「尚未实现」 | 对应的任务分组还没有完成 | 响应体里写明了组号，回到 `handlers.go` 找到那个函数 |
| `连接数据库失败` 并且错误里包含 `connection refused` | PostgreSQL 容器没有运行，或者 `DATABASE_URL` 的主机与端口不正确 | 执行 `docker compose ps` 确认服务状态；执行 `docker compose logs postgres` 查看数据库日志 |
| `连接 Redis 失败` | Redis 容器没有运行，或者 `REDIS_ADDR` 不正确 | 执行 `docker compose ps`，再执行 `docker exec shortener-redis redis-cli ping`，预期输出 `PONG` |
| `address already in use` | 8080 端口已经被占用，可能是上一次的服务进程没有退出 | 执行 `ss -ltnp \| grep 8080` 找到占用端口的进程；或者用 `HTTP_ADDR=:8081 go run ./cmd/api` 换端口启动 |
| 创建接口返回 500 并且日志中出现 `23505` | 短码冲突的重试逻辑没有实现，或者重试次数用完之后仍然冲突 | 检查 `handleCreateLink` 中是否对 `23505` 做了重新生成 |
| 列表接口的 `items` 是 `null` | `ListLinks` 在结果为空时返回了 `nil` 切片 | 检查 `ListLinks` 中是否用 `make([]Link, 0, limit)` 初始化了切片 |
| 跳转第一次成功、第二次返回 503 | 缓存回填时写入了空字符串，或者生存时间设置为 0 导致键立刻过期 | 执行 `docker exec shortener-redis redis-cli TTL link:<code>` 查看生存时间 |
| 统计接口返回错误而不是 0 | 命中率的分母为 0 时没有做判断，`NaN` 无法序列化成 JSON | 在 `handleStats` 中先判断分母是否为 0 |
| 数据库中的 `clicks` 一直是 0 | 后台写回没有运行，或者写回周期还没到 | 查看启动日志中是否出现「后台点击写回协程尚未实现」；用 `CLICK_FLUSH_INTERVAL=5s` 启动 |
| 前端页面显示「无法连接后端」 | 后端没有运行，或者 `vite.config.js` 中转发目标不是 8080 | 执行 `curl -i http://localhost:8080/api/healthz` 确认后端可达 |
| 启动时报 `panic: '...' in new path '...' conflicts with existing wildcard` | 两条路由的模式在同一层级上发生冲突，例如同时注册 `/links/:code` 与 `/links/new` | gin 在注册阶段就会 panic，panic 信息里写明了两条冲突的模式；调整其中一条的路径结构即可 |
| 日志中出现 `[GIN-debug] [WARNING] Running in "debug" mode` | 这是 gin 在 debug 模式下的启动提示，不是错误 | 本地联调可以保留，这一段输出正好用来核对路由注册结果；容器环境中设置 `GIN_MODE=release` 关闭，`api/Dockerfile` 中已经设置 |
| 某个处理函数在一次请求中被执行了两次 | panic 恢复中间件捕获 panic 之后没有终止后续处理，外层中间件的循环因此继续执行了后续的处理函数 | 检查 `recoveryMiddleware` 的收尾分支：两个分支都必须调用 `c.Abort()` 或者 `c.AbortWithStatusJSON`，后者内部已经包含终止动作 |
| 日志中出现 `headers were already written`，或者响应体被拼成非法 JSON | 处理函数在已经写出响应之后又写了一次 | 在再次写出之前先判断 `c.Writer.Written()` |
| 就绪检查与跳转处理函数拿到的短码不是预期取值 | 路径参数的名字写错了，例如注册的是 `:code` 而读取时写的是 `c.Param("shortCode")` | 检查 `c.Param` 的参数与 `Routes()` 中注册的参数名是否完全一致 |

---

## 12. 八组全部完成之后的收尾

| 序号 | 动作 | 说明 |
|---|---|---|
| 1 | 删除 `internal/todo` 包 | 这个包只用于生成「尚未实现」的提示。删除之前先执行 `grep -rn 'internal/todo' --include='*.go' .` 找出全部引用位置，逐个删除引用之后，再删除 `internal/todo` 目录与 `internal/shortcode/todo.go` |
| 2 | 确认编译仍然通过 | 执行 `go build ./... && go vet ./...`，两者都应无输出 |
| 3 | 重新执行全量验收 | 执行第 10 节的全部命令，确认删除辅助包之后行为没有变化 |
| 4 | 记录本阶段的结论 | 把第 2.4、3.4、4.4、5.4、6.4、7.3、9.3 七张小节的表格内容合并进当天笔记，作为第 3 周「存储与资源」主题的结论部分 |
| 5 | 进入阶段 3 | 阶段 3 需要把四个组件一起放进 Compose 运行。`api/Dockerfile` 已经写好，你需要在 `compose.yaml` 中增加 `api` 与 `web` 两个服务，其中 `web` 的端口映射写成 `8080:80`，`api` 的服务名必须是 `api` 并且暴露 8080 端口，原因是 `web/nginx.conf` 中的上游地址写的正是 `api:8080` |

---

## 13. 本次完成的记录（2026-09-30）

第 4 组至第 8 组与收尾动作已经全部执行完毕，下表记录实际执行的动作与实测结果，
其中「实测结果」一列的内容来自本机运行的真实输出，可以直接作为笔记中的结论引用。

| 序号 | 动作 | 实测结果 |
|---|---|---|
| 1 | 填写第 4 组至第 7 组的六个处理函数 | 代码位于 `internal/httpapi/handlers.go`，共用一个常量块（`maxURLLength`、`maxCreateAttempts`、`defaultListLimit`、`maxListLimit`、`readinessCheckTimeout`）与三个辅助函数（`isShortCodeFormat`、`checkDependency`、`cacheHitRate`） |
| 2 | 填写第 8 组的两个方法 | `flusher.Run` 用 `time.NewTicker` 配合 `select` 同时等待定时器与 `ctx.Done()`，`FlushOnce` 逐个短码执行「先写数据库、后扣除缓存增量」 |
| 3 | 删除 `internal/todo` 包 | 包内已经没有引用方，删除之后 `go build ./...` 与 `go vet ./...` 都没有输出，`gofmt -l .` 也为空 |
| 4 | 更新 `internal/httpapi/router_test.go` | 期望取值同步为完成之后的行为；新增「请求体非法返回 400」「四个 `url` 校验分支」「五个分页校验分支」「短码格式不符返回 HTML 形态的 404」四组用例，`go test ./...` 全部通过 |
| 5 | 修复就绪检查的超时上界 | 在 `internal/cache/redis.go` 的 `New` 中设置 `ContextTimeoutEnabled: true`、`DialTimeout`、`ReadTimeout`、`WriteTimeout` 四项。修复之前，Redis 容器正在停止的过程中，就绪检查要 5.005 秒才有结论；修复之后同一窗口下的最大值是 2.004 秒 |
| 6 | 全量验收第 4 组至第 7 组 | 创建返回 201 且四个字段齐全；四个校验分支的文本与契约一致；跳转返回 302 与正确的 `Location`；`link:{code}` 的 `TTL` 是 300；删除返回 204 且两个键的 `EXISTS` 是 0；删除之后跳转返回 404；列表与统计的字段全部正确 |
| 7 | 验收第 8 组 | 写回周期设置为 60 秒时，跳转十次之后 Redis 中的增量是 10 而数据库中的点击数仍然是 0；向进程发送 SIGTERM 触发优雅退出之后，数据库中的点击数变成 10，Redis 中的增量变成 0 |
| 8 | 阶段 3 的 Compose 改造 | `compose.yaml` 增加 `api` 与 `web` 两个服务，`api` 不发布宿主机端口（与 Kubernetes 中只提供 ClusterIP 的安排一致），`web` 映射 `8080:80`；`api/Dockerfile` 增加 `GOPROXY` 构建参数，默认取值是 `https://goproxy.cn,direct` |

需要写进笔记的机制共三条：第一，命中率的分母为 0 时必须返回 0，因为 `NaN` 不是合法的 JSON；
第二，删除操作必须先动数据库再动缓存，反过来会让「数据库删除失败」表现为删除没有生效；
第三，写回的扣除顺序决定了故障后果，先写数据库只会造成重复写回，先扣缓存会永久丢失计数。

