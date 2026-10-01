# api · Go 语法速查（读这个工程需要的最小集合）

> 建立日期：2026-09-27（第 3 周）。
> 用途：这个工程用到的 Go 语法与标准库功能都在本文里，按主题分成十六节。
> 每一节的结构都是「概念 → 写法 → 本工程中的实例 → 这么写的原因」。
> 只需要掌握本文列出的内容，就能完整读懂并填写 `api/` 下面的全部代码。
>
> 与本文配套的另一份文档是 `BACKEND-OVERVIEW.md`，那一份讲的是系统怎么运转。

---

## 1. 包、导入与导出规则

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 包声明 | 每个 `.go` 文件的第一条有效语句是 `package 名字` | `internal/store/postgres.go` 的第一条有效语句是 `package store` | 同一个目录下的所有文件必须使用同一个包名，包名通常与目录名一致 |
| 导入标准库 | `import "包路径"` | `import "net/http"` | 标准库的路径不包含域名 |
| 导入第三方包 | `import "域名/路径"` | `import "github.com/redis/go-redis/v9"` | 路径的第一段是域名 |
| 导入本工程内的包 | `import "模块名/目录路径"` | `import "shortener/internal/config"` | 第一段 `shortener` 来自 `go.mod` 中的 `module shortener`，后面是相对于模块根目录的目录路径 |
| 引用第三方包时的名字 | 使用路径的最后一段 | 导入路径 `github.com/jackc/pgx/v5/pgxpool` 在代码中写作 `pgxpool.NewWithConfig` | 主版本后缀 `/v5` 属于导入路径的一部分，不计入包名 |
| 分组导入 | 用圆括号按行列出，常用空行分隔标准库与第三方包 | `cmd/api/main.go` 的导入块 | 这只是 `gofmt` 的排版惯例，不是语法要求 |
| 导出规则 | 首字母大写的标识符可以被其他包访问，首字母小写的只能在包内部访问 | `Postgres`、`CreateLink`、`ErrNotFound`、`Generate` 是导出的；`pool`、`linkKeyPrefix`、`notImplemented` 是包私有的 | Go 没有 `public` 与 `private` 关键字，大小写本身就是权限控制 |
| 未使用的导入 | 会导致编译失败 | 实现 `handleListLinks` 时如果没有用到 `strconv`，就必须删除这一行导入 | 这是刻意的设计，用来防止依赖无限增长 |
| 未使用的局部变量 | 会导致编译失败 | — | 但未使用的函数参数不会报错 |
| `internal` 目录 | 位于 `internal/` 下面的包只能被本模块内部的代码导入 | `internal/store` 无法被 `shortener` 之外的模块导入 | 由编译器强制，用来表达「这些包属于实现细节」 |
| 包注释 | 写在 `package` 关键字上方的连续注释 | `internal/shortcode/shortcode.go` 开头以 `Package shortcode` 起始的注释块 | 约定格式是 `Package 包名 说明文字`，`go doc` 与编辑器提示都会显示它 |

---

## 2. 变量、常量与复合字面量

| 语法 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 变量声明 | `var 名字 类型` | `var link Link` | 声明之后得到该类型的零值，数值类型是 0，字符串是空串，指针与切片是 `nil` |
| 声明并初始化 | `var 名字 类型 = 表达式` | `var ErrMiss = errors.New("缓存未命中")` | 类型可以由表达式推导时也可以省略类型 |
| 短变量声明 | `名字 := 表达式` | `pool, err := pgxpool.NewWithConfig(ctx, poolConfig)` | 只能用在函数内部；左侧至少要出现一个此前未声明的变量 |
| 常量 | `const 名字 = 表达式` | `const Length = 6`、`const maxRequestBodyBytes = 64 * 1024` | 常量在编译期求值，声明之后不能修改 |
| 常量组 | `const` 后面跟一对圆括号，按行声明 | `internal/cache/redis.go` 中的键名前缀常量组 | 用于把一组相关的常量集中在一起 |
| 多返回值 | 在返回类型位置并列写出多个类型 | `func New(ctx context.Context, addr string, password string, db int) (*Redis, error)` | Go 没有异常机制，失败信息通过最后一个返回值传递 |
| 命名返回值 | 在返回类型位置给返回值起名字 | `func (r *Redis) CacheStats(ctx context.Context) (hits int64, misses int64, err error)` | 起名之后函数内部可以直接给它们赋值；本工程仍然显式写出 `return`，可读性更好 |
| 复合字面量 | `类型{字段: 取值}` | `&Flusher{cache: rdb, store: pg, interval: interval, logger: logger}` | 字段名与取值用冒号分隔，多个字段用逗号分隔 |
| 取结构体地址 | 在复合字面量前加 `&` | `return &Postgres{pool: pool}, nil` | 返回指针而不是值，避免每次调用都复制整个结构体 |
| 空标识符 | `_` | `_ = pg.DeleteLink(ctx, code)`、`for _, link := range links` | 表示「这个返回值不使用」；用它显式丢弃不能忽略的返回值 |
| 类型转换 | `目标类型(表达式)` | `string(body)`、`int64(value)`、`time.Duration(seconds)` | 与类型断言不同，转换要求两种类型之间可以互相表示 |
| 类型断言 | `表达式.(类型)` | 第 3 组处理 `MGet` 返回值时使用 | 断言失败会 panic，安全写法是 `v, ok := x.(T)` |
| 无类型常量 | 不写类型的常量 | `const maxRequestBodyBytes = 64 * 1024` | 赋值给不同类型的变量时自动适配，因此在 `http.MaxBytesReader` 与 `int64` 两种用途下都能直接使用 |

### 2.1 for 循环的四种形式

Go 只有 `for` 一个循环关键字，没有 `while`，也没有 `do...while`。全部形式只有下面四种。

| 形式 | 写法 | 何时使用 | 本工程中的位置 |
|---|---|---|---|
| 三段式 | `for 初始化语句; 条件; 后置语句 { }` | 需要自己维护下标或者计数器时 | 短码生成的早期版本用过，现在改成了下面的整数遍历形式 |
| 只有条件 | `for 条件 { }` | 相当于其他语言里的 `while` | `ListLinks` 中读取结果集的循环 |
| 无限循环 | `for { }` | 退出条件写在循环体内部的 `break` 或者 `return` 上 | 第 8 组的写回协程 |
| 遍历 | `for 键, 值 := range 被遍历对象 { }` | 遍历切片、数组、map、字符串、通道，或者从 Go 1.22 起遍历一个整数 | `ListLinks` 之外的其他循环，例如测试文件中的 `for _, link := range links` |

三段式有四条语法规则需要记住。

| 规则 | 说明 |
|---|---|
| 三条语句都不能加括号 | 写成 `for (i := 0; i < 10; i++)` 会编译失败，这是与 C 系语言最明显的差别 |
| 初始化语句与后置语句都可以省略 | 两条都省略时只剩下条件，也就是上表的第二种形式 |
| 后置语句只能使用赋值 | 可以是 `i = i + 1` 或者 `i++`，不能使用短变量声明 `:=` |
| 循环变量在循环之外不可见 | 它属于循环自身的隐式作用域，循环结束后访问它会编译失败 |

`range` 形式有四条细节需要记住。

| 细节 | 说明 |
|---|---|
| 不需要值的时候用空标识符占位 | 写法是 `for _, link := range links`。如果写成 `for link := range links`，得到的 `link` 是下标而不是元素，这是与 Python 一类语言最容易混淆的一点 |
| 只需要下标时省略第二个变量 | 写法是 `for i := range links` |
| 遍历 map 的顺序是随机的 | 需要固定顺序时必须先把键取出来排序 |
| 从 Go 1.22 起可以直接遍历整数 | `for range Length` 表示循环 `Length` 次并且不使用循环变量；写成 `for i := range Length` 时 `i` 依次取 0 到 `Length-1` |

关于两个控制关键字：`continue` 跳过本轮剩余的语句并直接进入下一轮；`break` 立即结束整个循环。带标签的 `break 标签` 用于跳出多层嵌套，本工程没有多层嵌套的循环，因此不使用标签。

关于「读取查询结果集」这个场景为什么只能用第二种形式：`rows.Next()` 这个调用同时承担两个职责，推进游标与报告是否还有下一行，因为推进动作是它自己完成的，所以它可以直接写在 `for` 的条件位置；而第四种形式需要一个可以被逐个取出的对象，一次性使用的 `pgx.Rows` 不满足这个条件。

---

## 3. 结构体、方法与接收者

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 结构体定义 | `type 名字 struct { 字段名 类型 }` | `type Postgres struct { pool *pgxpool.Pool }` | 结构体是值类型，赋值与传参时默认复制 |
| 方法定义 | `func (接收者 类型) 名字(参数) 返回值` | `func (p *Postgres) Close()` | 方法与普通函数的唯一区别是多了一个接收者，表示这个方法属于哪个类型 |
| 指针接收者 | 接收者写在 `*类型` 上 | `store`、`cache`、`httpapi`、`flusher` 四个包的全部方法 | 需要修改结构体字段、结构体较大需要避免复制、或者方法内部需要判断接收者是否为 `nil` 时必须使用指针接收者 |
| 值接收者 | 接收者写在 `类型` 上 | `func (c Config) Summary() string`、`func (c Config) Load()` 之外的辅助函数 | `Config` 只读不写、复制成本低，使用值接收者语义更清晰 |
| 自动解引用 | `p.pool` | 等价于 `(*p).pool` | 通过指针访问字段或调用方法时不需要手动写 `*` |
| 结构体标签 | 字段后面的反引号内容 | `Code` 字段的标签是 `json:"code"` | 标签是给 `encoding/json` 这类库读取的元数据，Go 语言本身不解释它的内容 |
| 接口嵌入接口 | 接口里写另一个接口的名字 | gin 的 `gin.ResponseWriter` 内嵌了标准库的 `http.ResponseWriter`，因此它同时包含标准库接口的全部方法 | 嵌入之后外层接口包含被嵌入接口的全部方法，`c.Writer` 的类型就是它 |
| 结构体嵌入 | 结构体里只写类型名、不写字段名 | 本工程未使用 | 改用 gin 之前，日志中间件用这种写法包装响应写出器，以便把状态码读回来；现在直接调用 `c.Writer.Status()` 与 `c.Writer.Written()`，这一层包装不再需要 |
| 构造函数惯例 | 名字以 `New` 开头，返回指针与 `error` | `store.NewPostgres`、`cache.New`、`httpapi.New`、`flusher.New` | Go 没有构造函数关键字，用普通函数承担这个角色；需要建立连接或校验参数时把 `error` 作为第二个返回值 |

---

## 4. 错误处理（最需要先看懂的一节）

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 错误是返回值 | `if err != nil { return err }` | `store.NewPostgres` 中每一步之后都是这个模式 | 没有 `try` 与 `catch`，错误必须显式判断 |
| 包装并保留原因 | `fmt.Errorf("说明：%w", err)` | `fmt.Errorf("创建数据库连接池失败：%w", err)` | `%w` 把原错误包进新错误里形成链，`errors.Is` 可以穿透整条链 |
| 只拼文本、丢弃原因 | `fmt.Errorf("说明：%v", err)` | 本工程未使用这种写法 | 使用 `%v` 之后原错误变成普通文本，`errors.Is` 再也找不到它，这是最常见的失误 |
| 创建新错误 | `errors.New("文本")` | `var ErrNotFound = errors.New("短码不存在")` | 用于创建哨兵错误 |
| 哨兵错误 | 声明为包级变量的错误，供调用方比较 | `store.ErrNotFound` 与 `cache.ErrMiss` | 用来表达一种可以预期的业务状况，而不是故障 |
| 判断错误类别 | `errors.Is(err, 目标错误)` | `if errors.Is(err, store.ErrNotFound) { 返回 404 }` | 只能用于哨兵错误；与 `==` 的区别是它能穿透 `%w` 包装的多层链 |
| 取出具体错误类型 | `var 目标 *具体类型` 加 `errors.As(err, &目标)` | `var pgErr *pgconn.PgError` 加 `errors.As(err, &pgErr)`，再读取 `pgErr.Code` | 用于需要读取错误对象内部字段的场景，例如读取 PostgreSQL 的 SQLSTATE |
| 不能使用字符串比较 | 不要写 `err.Error() == "..."` | — | 错误文本会随包装层与版本变化，包装之后再也匹配不上 |
| 判断错误是否来自上游库 | `errors.Is(err, pgx.ErrNoRows)`、`errors.Is(err, redis.Nil)` | 第 2 组的 `GetLink` 与第 3 组的 `GetURL` | 第三方库也使用哨兵错误表达「没有数据」这种正常情况 |
| 忽略明确的无关错误 | 用空标识符显式丢弃 | `_ = pg.DeleteLink(ctx, code)` 出现在测试的清理逻辑中 | 写成 `_ =` 而不是完全不接返回值，是为了让读代码的人知道这个忽略是刻意的 |
| 自定义错误的实现 | 只要类型拥有 `Error() string` 方法就满足 `error` 接口 | `internal/store` 包借助 `errors.New` 生成 `ErrNotFound` 与 `ErrConflict` 两个哨兵错误 | 本工程没有自定义错误类型，全部使用 `errors.New` 与 `fmt.Errorf` |

为什么 `store` 必须区分「记录不存在」与「数据库出错」，并且用两个不同的返回值表达：
前者是正常的业务结果，处理函数应当返回 404；后者是系统故障，处理函数应当返回 503。
两种情况在错误文本里可能都包含 `no rows` 之类的词，靠文本判断在包装之后必然失效，
因此必须由存储层返回一个可以用 `errors.Is` 判断的哨兵错误。

---

## 5. defer

| 概念 | 说明 | 本工程中的实例 |
|---|---|---|
| 执行时机 | `defer` 注册的调用在函数返回之前执行，函数体正常返回与发生 panic 两种情况都会执行 | `main.go` 的 `defer pg.Close()`、`defer rdb.Close()`、`defer cancelStartup()` |
| 执行顺序 | 同一个函数内注册多个 `defer` 时，按后进先出的顺序执行 | `run` 函数中先注册 `defer stop()`，之后注册 `defer cancelStartup()`，因此先执行 `cancelStartup` 再执行 `stop` |
| 参数求值时机 | `defer` 语句的参数在注册的那一刻求值，函数体在返回时才执行 | `defer cancelShutdown()` 在注册时就把 `cancelShutdown` 这个变量的值固定下来 |
| 常用场景 | 释放连接、关闭客户端、停止定时器、解锁、写收尾日志 | 第 8 组中要写的 `defer ticker.Stop()` |
| 为什么收尾动作必须用 defer | 处理函数发生 panic 时控制流会直接跳出函数，只有 `defer` 能保证收尾动作仍然被执行 | `internal/httpapi/middleware.go` 中 `recoveryMiddleware` 的 `defer func() { 恢复 panic 并写出 500 响应 }()`；作为对比，同一个文件里的 `loggingMiddleware` 不使用 `defer`，因为它的日志写在 `c.Next()` 之后，它需要的是「处理完成之后」而不是「函数退出之前」，两者是不同的时机 |
| cancel 必须被调用 | `context.WithTimeout` 返回的取消函数不调用会造成 context 泄漏 | 本工程中每一个 `context.WithTimeout` 后面都紧跟着 `defer cancel()` |

---

## 6. 切片、map 与 nil 的区别

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 切片类型 | `[]元素类型` | `[]Link`、`[]linkResponse`、`[]byte` | 切片是指向底层数组的视图，包含长度与容量两项信息 |
| 预分配容量 | `make([]T, 0, 容量)` | `items := make([]linkResponse, 0, len(links))` | 预先给出容量可以避免 `append` 过程中反复扩容并复制数据 |
| 追加元素 | `s = append(s, 元素)` | `items = append(items, toLinkResponse(link))` | `append` 返回的是新切片，必须重新赋值，否则新增的元素会丢失 |
| nil 切片与空切片的区别 | `var s []T` 是 `nil`；`make([]T, 0)` 不是 `nil` | `toLinkResponses` 刻意使用 `make` 而不是 `var` | 两者长度都是 0，但是序列化成 JSON 之后分别是 `null` 与 `[]`，而接口契约规定 `items` 字段始终是数组，因此空结果必须返回 `make` 出来的切片 |
| 判断切片是否为空 | 使用 `len(s) == 0` | 测试文件中的断言 | 不要用 `s == nil` 判断，因为空切片与 `nil` 切片的长度都是 0，而前者不是 `nil` |
| map 类型 | `map[键类型]值类型` | `map[string]int64`（`CollectClicks` 的返回值）、`map[string]string`（错误响应体） | map 是引用类型，读取不存在的键返回该值类型的零值，不会报错 |
| 判断键是否存在 | `v, ok := m[k]` | 第 7 组读取 `checks` 映射时需要使用 | `ok` 为 `false` 表示键不存在，此时 `v` 是零值 |
| 写入键值 | `m[k] = v` | `counts[ch]++` 出现在测试文件里 | 对不存在的键直接做加法也成立，因为读取到的是零值 |
| 遍历 map | `for 键, 值 := range m` | 第 3 组处理 `CollectClicks` 的返回值时使用 | 遍历顺序是随机的，需要固定顺序时必须先把键排序 |
| 删除键 | `delete(m, k)` | 本工程未使用 | 本工程的删除操作都由 Redis 命令完成 |

---

## 7. 接口与隐式实现

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 接口定义 | `type 名字 interface { 方法签名 }` | `error` 接口只有一个方法 `Error() string` | 接口只是方法集合，不包含数据 |
| 隐式实现 | 一个类型只要拥有接口要求的全部方法，就自动满足该接口 | `*gin.Engine` 拥有 `ServeHTTP(w http.ResponseWriter, r *http.Request)` 方法，因此它自动满足标准库的 `http.Handler`，可以直接赋值给 `http.Server` 的 `Handler` 字段 | **不需要写 `implements` 之类的关键字**，这是 Go 与 Java 一类语言最大的差异 |
| 标准库接口 | `http.Handler` 只有一个方法 `ServeHTTP(w http.ResponseWriter, r *http.Request)` | `cmd/api/main.go` 中的 `srv := &http.Server{...}`，其中 `Handler` 字段接收的是 gin 引擎 | 只要一个取值满足这个接口，它就可以承担 HTTP 服务的处理入口，与它内部用什么框架实现无关 |
| 框架定义的函数类型 | 一个具名函数类型，并且拥有自己的方法 | `gin.HandlerFunc` 的底层类型是 `func(c *gin.Context)`；`engine.Use` 与 `engine.GET` 接收的都是它 | `middleware.go` 的两个方法返回 `gin.HandlerFunc`；`handleHealthz` 这类方法的类型与之相同，所以可以直接传给 `api.GET`，不需要显式转换 |
| 接口嵌入接口 | 接口里写另一个接口的名字 | `gin.ResponseWriter` 内嵌了 `http.ResponseWriter` 与 `http.Hijacker` 等接口 | 表示该接口同时包含被嵌入接口的全部方法 |
| 空接口 | `any` 是 `interface{}` 的别名，表示任何类型 | `func decodeJSON(c *gin.Context, dst any) error` | 需要接收任意类型时使用，代价是运行时才能确定具体类型 |
| 接口值为 nil 的陷阱 | 接口变量只有在类型与取值都为空时才是 `nil` | 本工程通过显式判断规避 | 一个持有 `nil` 指针的非空接口，用 `== nil` 判断的结果是 `false` |
| 为什么处理函数使用框架的上下文而不是标准库的两个参数 | 框架把请求与响应合并成一个上下文对象，取值与写出都由它提供 | `func(c *gin.Context)` 既可以读 `c.Param` 与 `c.Query`，也可以写 `c.JSON`、`c.Data` 与 `c.Redirect` | 减少参数个数，并且让「读取响应已经写出的状态」这类操作有统一入口 |

标准库的 `http.ResponseWriter` 只能写状态码、不能把状态码读回来，
因此在改用 gin 之前，日志中间件必须用结构体嵌入的方式自己包装一层才能记录状态码。
改用 gin 之后 `c.Writer` 本身提供 `Status()` 与 `Written()` 两个方法，这一层包装不再需要。
这个变化说明一件事：**框架是否提供某项能力，直接决定了业务代码需要额外写多少辅助结构**。

---

## 8. context 与取消信号

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 根 context | `context.Background()` | `main.go` 的 `run` 函数与全部测试文件 | 所有 context 的起点，本身永远不会被取消 |
| 带超时的子 context | `context.WithTimeout(父, 时长)` | `context.WithTimeout(ctx, startupTimeout)` | 到达时限时自动取消，同时返回一个取消函数 |
| 手动取消 | `cancel()` | 每一个 `WithTimeout` 后面都跟着 `defer cancel()` | 即使时限还没到也必须调用，否则内部资源不会释放 |
| 读取取消信号 | `<-ctx.Done()` | `run` 函数的 `select`、第 8 组要写的 `Run` 循环 | `Done()` 返回一个通道，context 被取消时该通道被关闭 |
| 取消原因 | `ctx.Err()` | 排查时使用 | 返回 `context.Canceled`（主动取消）或者 `context.DeadlineExceeded`（超时） |
| 把操作系统信号转成取消 | `signal.NotifyContext(父, 信号列表)` | `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` | 收到 Ctrl+C 或者容器停止信号时，派生出的 context 被取消，一个通道通知全部下游 |
| 向下游传递 | 把 context 作为函数的第一个参数 | 全部需要访问外部资源的函数签名都是 `func (ctx context.Context, ...)` | 这是 Go 的强制惯例，第一个参数固定是 context |
| 传递请求范围的数据 | `context.WithValue` | 本工程未使用 | 本工程只用 context 传递取消与超时，不用它传递业务参数 |
| 超时时间的来源 | 从配置读取，不写死在代码里 | `cfg.CacheTTL`、`cfg.ShutdownTimeout`、`startupTimeout` 常量 | 便于在不同环境调整，也便于演示 |

`run` 函数中收尾用的 `shutdownCtx` 必须从 `context.Background()` 派生，不能从已经取消的 `ctx` 派生。
原因是 `ctx` 在收到停止信号时已经处于取消状态，从它派生的子 context 会立刻也是取消状态，
`srv.Shutdown` 会直接返回而不会等待任何请求处理完成，优雅退出就完全失效了。

---

## 9. 并发三件套：goroutine、channel、select

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 启动协程 | `go 函数调用()` | `go fl.Run(ctx)`、`go func() { ... }()` | 协程是语言层面的轻量线程，启动开销很小，可以启动成千上万个 |
| 带缓冲的通道 | `make(chan 元素类型, 容量)` | `serverErr := make(chan error, 1)` | 容量为 1 时，发送方在没有人立刻接收的情况下也不会阻塞 |
| 发送与接收 | `ch <- 值` 与 `值 := <-ch` | `serverErr <- err` 与 `case err := <-serverErr:` | 箭头的方向表示数据的流向 |
| 多路等待 | `select { case ...: case ...: }` | `run` 函数同时等待监听失败与停止信号 | 哪个分支先就绪就先执行哪个；全部阻塞时 `select` 会一直等待 |
| 定时触发 | `time.NewTicker(周期)` | 第 8 组要写的写回循环 | 返回的 `ticker.C` 是一个通道，每个周期收到一个时间值 |
| 停止定时器 | `ticker.Stop()` | 第 8 组要写的 `defer ticker.Stop()` | 不停止时定时器会一直占用运行时资源 |
| 数据竞争 | 多个协程同时读写同一个变量且没有同步手段 | 本工程刻意避免 | 表现形式是结果不确定、难以复现 |
| 本工程如何避免竞争 | 把与状态相关的操作交给外部组件执行 | 点击计数交给 Redis 的原子自增命令，点击累加交给数据库的 `UPDATE links SET clicks = clicks + $2` | 这样应用进程内部不需要加锁，多个副本同时运行时结果也正确，这是副本数可以超过 1 的前提 |

---

## 10. JSON 序列化与结构体标签

| 概念 | 写法 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 序列化到响应 | `json.NewEncoder(w).Encode(v)` | `internal/httpapi/handlers.go` 中的 `writeError`（它内部调用 gin 的 `c.JSON`） | gin 的 `c.JSON` 默认使用标准库的 `encoding/json`，并且会同时设置 Content-Type 与状态码 |
| 序列化为字节切片 | `json.Marshal(v)` | 本工程未使用 | 需要把结果放进其他结构或者做字符串拼接时使用 |
| 反序列化请求体 | `json.NewDecoder(请求体).Decode(&目标)` | `internal/httpapi/handlers.go` 的 `decodeJSON`，它读取的是 `c.Request.Body` | 必须传指针，否则解析结果写不回原变量；gin 的 `c.ShouldBindJSON` 也能完成解析，本工程刻意不使用它，理由写在 `decodeJSON` 的注释里 |
| 字段名映射 | 结构体标签 `json:"名字"` | `Code` 字段的标签是 `json:"code"` | 不写标签时使用 Go 的字段名本身；本工程全部显式指定，与契约文档保持一致 |
| 忽略某个字段 | 结构体标签 `json:"-"` | 本工程未使用 | 表示该字段不参与序列化与反序列化 |
| 省略零值 | 结构体标签 `json:",omitempty"` | 本工程未使用 | 字段取值为零值时不出现在结果里 |
| 未导出字段 | 首字母小写的字段 | `Postgres.pool`、`cache.Redis.client`、`Server.logger` | `encoding/json` 无法读写未导出字段，因此内部状态不会泄漏到响应里 |
| 时间类型 | `time.Time` | `linkResponse.CreatedAt` | 默认序列化成 RFC 3339 格式，前端的 `new Date(...)` 可以直接解析 |
| 无法序列化的浮点取值 | `NaN` 与正负无穷 | `handleStats` 中的命中率 | 这两个取值不是合法的 JSON，序列化会失败；因此命中率的分母为 0 时必须先判断并返回 0 |
| 请求体的长度限制 | `http.MaxBytesReader(响应写出器, 请求体, 上限)` | `decodeJSON` 中的 `http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)` | 在读取过程中实际达到上限时报错，因此不受客户端伪造的 `Content-Length` 影响；gin 的响应写出器实现了标准库的 `http.ResponseWriter`，因此可以直接传给它 |

---

## 11. 路由注册与路径参数（gin 框架）

| 写法 | 含义 | 本工程中的实例 |
|---|---|---|
| `api := engine.Group("/api")` | 路由分组。组内注册只需要写前缀之后的部分 | `internal/httpapi/router.go` 的 `Routes()` |
| `api.GET("/healthz", 处理函数)` | 注册一条只接受 `GET` 方法的路由 | 存活检查 |
| `api.POST("/links", 处理函数)` | 同一个路径上可以按方法注册不同的处理函数 | 创建接口与列表接口共用 `"/links"` 这个路径 |
| `api.DELETE("/links/:code", 处理函数)` | `:code` 匹配单个路径片段 | 删除接口 |
| `engine.GET("/:code", 处理函数)` | 挂在引擎根上的单片段参数路由 | 短码跳转 |
| 读取路径参数 | `c.Param("code")` | 跳转与删除两个处理函数 |
| 读取查询参数 | `c.Query("limit")`，带默认值用 `c.DefaultQuery("limit", "20")` | 列表接口 |
| 静态片段优先于参数片段 | 同一层级上同时存在静态与参数子节点时，请求优先匹配静态的那一条 | 请求 `/api/links` 由 `api.GET("/links", ...)` 处理，不会被 `engine.GET("/:code", ...)` 抢走 |
| 路径不匹配 | 没有任何路由匹配时返回 404，响应体由 gin 生成 | 请求 `/nope/extra/path` 得到 404 与响应体 `404 page not found` |
| 方法不匹配 | 默认返回 404；把 `engine.HandleMethodNotAllowed` 设为 `true` 之后返回 405 | 用 `GET` 请求 `/api/links/xyz` 得到 405，因为该路径被 `DELETE /api/links/:code` 匹配 |
| 单一通配的副作用 | `/:code` 会匹配任意单个路径片段 | `/favicon.ico` 与 `/api` 都会命中跳转处理函数，因此该函数内部必须校验短码格式 |
| 不支持正则表达式 | gin 的路径参数不支持正则约束，只能在后端处理函数内部校验 | 六位短码的格式校验写在 `handleRedirect` 内部 |
| 路由冲突 | 同一层级上参数片段与静态片段发生冲突时，gin 在注册阶段直接 panic | 本工程刻意避免在同一层级上同时注册这两类路由，注册结果由 `internal/httpapi/router_test.go` 验证 |
| 路由清单 | debug 模式下 gin 在启动时打印全部路由与对应的处理函数 | 服务启动日志中的 `[GIN-debug]` 段，这一段可以用来核对路由注册结果 |

---

## 12. 测试与工具链命令

| 概念 | 写法或命令 | 本工程中的实例 | 说明 |
|---|---|---|---|
| 测试文件命名 | 以 `_test.go` 结尾 | `internal/store/postgres_test.go` | 构建正式产物时这些文件被忽略 |
| 测试函数命名 | `func Test开头名字(t *testing.T)` | `TestGenerateLengthAndAlphabet` | 函数名必须以 `Test` 开头，参数固定为 `*testing.T` |
| 立即终止本测试 | `t.Fatalf(格式, 参数...)` | 检查返回值出错时使用 | 后面的断言不再执行 |
| 记录并继续 | `t.Errorf(格式, 参数...)` | 检查字段取值时使用 | 一次运行可以看到多个失败点 |
| 记录日志 | `t.Logf(格式, 参数...)` | 清理失败时使用 | 只在 `-v` 模式下显示 |
| 跳过测试 | `t.Skip("原因")` | 未设置 `TEST_DATABASE_URL` 时跳过 | 用来让没有依赖服务的机器也能跑通 `go test ./...` |
| 注册收尾动作 | `t.Cleanup(func(){ ... })` | 删除测试写入的记录与键 | 测试结束时执行，即使测试失败也会执行 |
| 标记辅助函数 | `t.Helper()` | `newTestStore`、`insertTestLink` | 让失败信息指向调用处而不是辅助函数内部 |
| 子测试 | `t.Run("名字", func(t *testing.T){ ... })` | 本工程未使用 | 表驱动测试时使用 |
| 运行全部测试 | `go test ./...` | — | `...` 表示递归到所有子目录 |
| 显示详细输出 | `go test ./... -v` | — | 打印每个测试的名称与结果 |
| 只运行指定测试 | `go test -run 名字前缀 ./包路径` | 排查单个失败时使用 | 名字按子串匹配 |
| 编译检查 | `go build ./...` | — | 只检查语法与类型，不产生输出文件 |
| 静态检查 | `go vet ./...` | — | 检查格式化字符串参数不匹配这类问题 |
| 格式检查 | `gofmt -l .` | — | 列出需要重新格式化的文件 |
| 格式修正 | `gofmt -w .` | — | 直接改写文件 |
| 依赖整理 | `go mod tidy` | — | 根据源码中的导入自动维护 `go.mod` 与 `go.sum` |
| 查看依赖清单 | `cat go.mod` | — | 直接列出直接依赖与间接依赖 |
| 直接运行 | `go run ./cmd/api` | — | 先编译到临时目录再执行，适合开发阶段 |
| 设置模块代理 | `export GOPROXY=https://goproxy.cn,direct` | — | 本机必须设置，否则依赖无法下载 |
| 查看当前模块代理 | `go env GOPROXY` | — | 排查依赖下载失败时使用 |
| 查看依赖为什么被引入 | `go mod why 包路径` | — | 排查间接依赖的来源时使用 |

---

## 13. gin 与 net/http 的对照

本工程的 HTTP 层使用 gin 框架。下表把同一个需求在两种写法下的形式并列出来，便于对照阅读源码。

| 需求 | 标准库 `net/http` 的写法 | gin 的写法 | 本工程中的位置 |
|---|---|---|---|
| 注册一条路由 | `mux.HandleFunc("GET /api/healthz", 处理函数)` | `engine.GET("/api/healthz", 处理函数)` | `router.go` 的 `Routes()` |
| 路由分组 | 每一条模式都要写完整路径 | `engine.Group("/api")`，组内只写前缀之后的部分 | `router.go` 的 `Routes()` |
| 处理函数的签名 | `func(w http.ResponseWriter, r *http.Request)` | `func(c *gin.Context)` | `handlers.go` 的全部处理函数 |
| 读取路径参数 | `r.PathValue("code")` | `c.Param("code")` | 第 5 组与第 6 组要写 |
| 读取查询参数 | `r.URL.Query().Get("limit")` | `c.Query("limit")`，带默认值用 `c.DefaultQuery("limit", "20")` | 第 4 组要写 |
| 读取请求体 | `json.NewDecoder(r.Body).Decode(&目标)` | 本工程使用 `decodeJSON(c, &目标)`，内部同样是标准库的解码器 | `handlers.go` 的 `decodeJSON` |
| 写出 JSON | 先设置 Content-Type、再写状态码、再序列化响应体 | `c.JSON(状态码, 取值)` 一次完成 | `handlers.go` 的 `writeError` 与全部处理函数 |
| 写出 HTML 或者纯文本 | 先设置 Content-Type、再写状态码、再写出字节切片 | `c.Data(状态码, "text/html; charset=utf-8", []byte(内容))` | 第 5 组的 404 响应 |
| 写出只有状态码的响应 | `w.WriteHeader(http.StatusNoContent)` | `c.Status(http.StatusNoContent)` | 第 6 组的删除成功响应 |
| 重定向 | `http.Redirect(w, r, 地址, http.StatusFound)` | `c.Redirect(http.StatusFound, 地址)` | 第 5 组的跳转 |
| 读取已经写出的状态码 | 标准库不提供，需要自己包装响应写出器 | `c.Writer.Status()` | `middleware.go` 的 `loggingMiddleware` |
| 读取响应是否已经写出 | 标准库不提供，需要自己包装响应写出器 | `c.Writer.Written()` | `middleware.go` 的 `recoveryMiddleware` |
| 中间件的签名 | `func(next http.Handler) http.Handler` | `func(c *gin.Context)`，配合 `c.Next()` | `middleware.go` 的两个中间件 |
| 终止后续的处理 | 没有对应概念，处理函数直接 `return` 即可 | 必须显式调用 `c.Abort()`，否则后续的处理函数会继续执行 | `middleware.go` 的 `recoveryMiddleware` |
| 客户端地址 | `r.RemoteAddr` | `c.ClientIP()`，取值受 `SetTrustedProxies` 影响 | `middleware.go` 的日志字段 |
| 启动服务 | 自建 `http.Server` 或者 `http.ListenAndServe` | 与标准库相同：引擎实现了 `http.Handler`，仍然交给 `http.Server` | `cmd/api/main.go` |
| 方法不匹配时的状态码 | 标准库返回 405 | 默认返回 404，需要把 `engine.HandleMethodNotAllowed` 设为 `true` 才返回 405 | `router.go` 的 `Routes()` |
| 路径参数是否支持正则约束 | 不支持 | 不支持 | 六位短码的格式校验写在 `handleRedirect` 内部 |

其中最需要留意的是「终止后续的处理」这一行。gin 的中间件链用一个下标在同一个循环里依次调用处理函数，
当某一层捕获到 panic 之后如果没有调用 `c.Abort()`，外层循环会继续递增下标并再次执行后续的处理函数，
表现为同一个处理函数被调用两次。这条机制在 `middleware.go` 的注释中有完整说明。

---

## 14. crypto/rand 与 math/big

本工程只有第 1 组的短码生成用到这两个包，因此本节只覆盖实际用到的部分，不覆盖两个包的全部 API。

### 14.1 crypto/rand 包：密码学安全的随机源的入口

| 概念 | 说明 |
|---|---|
| 导入路径与包名 | 导入路径是 `crypto/rand`，包名是 `rand`，因此代码中写作 `rand.Int` 或者 `rand.Reader` |
| 与 `math/rand` 的区别 | `math/rand` 生成的是伪随机序列，序列由包内部的、可以还原的状态经过确定的运算推演出来；`crypto/rand` 从操作系统提供的随机源读取取值，序列与进程的内部状态无关 |
| 它的 API 面很小 | 全部导出项只有一个全局变量 `Reader` 与四个函数 `Int`、`Read`、`Prime`、`Text`，因此只需要掌握本小节列出的内容 |

| 导出项 | 签名 | 作用 | 本工程是否使用 |
|---|---|---|---|
| `Reader` | `var Reader io.Reader` | 一个全局共享的密码学安全随机源，文档说明它可以被并发使用 | 使用，作为 `rand.Int` 的第一个参数 |
| `Int` | `func Int(rand io.Reader, max *big.Int) (n *big.Int, err error)` | 返回一个落在区间 $[0, max)$ 内的均匀分布整数；当 `max <= 0` 时该函数 panic；当 `rand.Read` 返回错误时该函数返回一个非 nil 的 error | 使用，用来生成短码的下标 |
| `Read` | `func Read(b []byte) (n int, err error)` | 把字节切片 `b` 完整填满随机字节。文档写明它不会返回错误，并且一定会把 `b` 全部填满 | 本工程不使用 |
| `Prime` | `func Prime(r io.Reader, bits int) (*big.Int, error)` | 返回一个指定位数的、以很高概率为素数的整数 | 本工程不使用 |
| `Text` | `func Text() string`，Go 1.24 新增 | 返回一个使用标准 RFC 4648 base32 字母表的随机字符串，其中包含至少 128 位的随机性 | 本工程不使用 |

关于 `Reader` 在 Linux 上的实现来源：文档说明它在 Linux、FreeBSD、Dragonfly 与 Solaris 上使用 `getrandom(2)` 系统调用，在版本低于 3.17 的旧版 Linux 上改为在首次使用时打开 `/dev/urandom`。这一条说明随机取值的来源是内核维护的随机源，而不是 Go 运行时的内部状态，因此进程重建不会让随机序列回到可预测的状态。

关于 `err` 返回值仍然必须处理的原因：`Int` 的文档说明「当 `rand.Read` 返回错误时返回错误」，而 `Read` 的文档又说明「它不会返回错误，并且总是把 `b` 填满」。把这两条合起来可以得到结论：在受支持的平台上几乎不会发生错误，但函数签名中保留了 error 返回值，因此调用方仍然必须写出错误分支。这是标准库在「保证接口稳定」与「允许实现改进」之间取得平衡的结果。

关于取模偏差的两种做法，区别如下。

| 做法 | 分布是否均匀 | 原因 |
|---|---|---|
| 先取得一个大范围的随机整数，再对它执行 `% 62` | 不均匀 | 假定随机源产生区间 $[0, 2^{32})$ 内的整数，因为 $2^{32}$ 不能被 62 整除，所以余数落在较小取值范围内的输入数量比落在较大取值范围内的输入数量更多，下标较小的字符因此被选中的概率偏高 |
| 使用 `rand.Int`，并且把上界设为 62 | 均匀 | `rand.Int` 内部采用拒绝采样：当随机取值落在「不能被 62 整除的尾部区间」内时，该取值被丢弃并且重新抽取，因此每个下标被选中的概率严格相等 |

> 关于一个常见的误解：`math/rand` 的 `Intn(n)` 与 `math/rand/v2` 的 `IntN(n)` 这两个函数内部同样采用拒绝采样，它们本身不产生偏差。偏差只出现在「由使用者自己手写取模运算」的写法中。

### 14.2 math/big 包：多精度整数

| 概念 | 说明 |
|---|---|
| 它解决的问题 | Go 的内置整数类型 `int64` 的取值上限是 $2^{63}-1$，而 `math/big` 提供的 `Int` 可以表示任意大的整数 |
| 与内置整数类型的关键差别 | `Int` 的全部运算与转换都以指针 `*Int` 作为参数与接收者；文档说明「每一个取值都需要各自的指针，浅拷贝不被支持，并且可能导致错误」 |
| 零值可以使用 | `Int` 的零值代表 0，因此可以直接声明 `var z big.Int` 而不需要额外的初始化 |
| 本工程为什么需要它 | `crypto/rand.Int` 的上界参数类型是 `*big.Int`，因此即使上界只有 62，也必须先把它包装成 `big.Int` |

| 导出项 | 签名 | 作用 |
|---|---|---|
| `Int` | `type Int struct` | 有符号多精度整数类型 |
| `NewInt` | `func NewInt(x int64) *Int` | 分配一个新的 `Int`，并且把它的取值设为 `x` |
| `SetInt64` | `func (z *Int) SetInt64(x int64) *Int` | 把已有的 `Int` 的取值设为 `x`，用于复用已经分配好的对象 |
| `Int64` | `func (x *Int) Int64() int64` | 转换成 `int64`。文档说明「如果取值不能在 `int64` 中表示，结果是未定义的」，因此转换之前应当先用 `IsInt64` 判断 |
| `IsInt64` | `func (x *Int) IsInt64() bool` | 判断这个取值是否可以由 `int64` 表示 |
| `Uint64` | `func (x *Int) Uint64() uint64` | 转换成 `uint64`，取值超出可表示范围时结果同样是未定义的 |
| `BitLen` | `func (x *Int) BitLen() int` | 返回取值的绝对值在二进制下需要的位数，0 的位数是 0 |
| `Cmp` | `func (x *Int) Cmp(y *Int) int` | 比较两个取值，小于时返回 -1，相等时返回 0，大于时返回 1 |
| `String` | `func (x *Int) String() string` | 返回十进制字符串，`Int` 因此满足 `fmt.Stringer` 接口 |
| `Text` | `func (x *Int) Text(base int) string` | 返回指定进制（2 到 62）的字符串表示 |

关于一个必须避开的写法：`math/big` 包内的 `Int` 类型自带一个名为 `Rand` 的方法，它的签名是 `func (z *Int) Rand(rnd *rand.Rand, n *Int) *Int`，它接受的随机源来自 `math/rand`。该方法的文档明确写出「由于它使用 `math/rand` 包，因此不得用于安全敏感的工作，应当改用 `crypto/rand.Int`」。如果实现短码时写成 `new(big.Int).Rand(mathrand.New(mathrand.NewSource(固定种子)), 上界)`，那么即使代码中出现了 `big.Int`，随机源实际上仍然是伪随机序列，缺陷与直接使用 `math/rand` 完全相同。

关于命名上的一处容易混淆的地方：`crypto/rand` 包中名为 `Int` 的标识符是包级函数，调用形式是 `rand.Int(...)`；`math/big` 包中名为 `Int` 的标识符是类型，而 `Int64` 是该类型的方法，调用形式是「某个取值 `.Int64()`」。两者名字相似，但含义与调用形式都不同。

### 14.3 本工程中的实际用法

| 代码片段 | 含义 | 需要注意的地方 |
|---|---|---|
| 导入 `crypto/rand` 与 `math/big` | 取得随机源与上界类型 | 导入路径必须写成 `crypto/rand`；写成 `math/rand` 会得到伪随机序列 |
| `big.NewInt(int64(len(Alphabet)))` | 构造上界 62 | `NewInt` 的参数类型是 `int64`，而 `len` 的返回值类型是 `int`，因此必须显式转换 |
| `rand.Int(rand.Reader, 上界)` | 取得一个落在区间 $[0, 62)$ 内的整数 | 第一个返回值是 `*big.Int`，第二个返回值是 `error` |
| 判断第二个返回值是否为 `nil` | 处理随机源读取失败的情况 | 失败时 `Generate` 应当返回空字符串与这个错误，由第 4 组的 `handleCreateLink` 把它转换成状态码 500 |
| `Alphabet[int(第一个返回值.Int64())]` | 按下标取出一个字符 | 下标的合法范围是 0 到 61，而上界是开区间，因此不会越界 |

---

## 15. pgx 与 PostgreSQL SQL

本工程通过 `github.com/jackc/pgx/v5` 访问 PostgreSQL，版本是 v5.11.0。本节覆盖第 2 组、第 3 组之后的存储层实现所需要的全部数据库 API 与 SQL 语法。

### 15.1 三个包的分工

| 导入路径 | 它提供什么 | 本工程使用它做什么 |
|---|---|---|
| `github.com/jackc/pgx/v5/pgxpool` | 连接池类型 `Pool` 与它的配置类型 `Config` | 在已经写好的 `NewPostgres` 中创建连接池，在七个方法中执行语句 |
| `github.com/jackc/pgx/v5` | 高层类型与错误：`Row` 接口、`Rows` 接口、`Tx` 接口、`ErrNoRows` 哨兵错误，以及 `CollectRows` 一类泛型辅助函数 | 使用 `pgx.ErrNoRows` 判断「没有查到行」 |
| `github.com/jackc/pgx/v5/pgconn` | 协议层：`PgError`、`CommandTag`、`PgConn` | 使用 `PgError` 的 `Code` 字段判断 SQLSTATE，使用 `CommandTag` 的 `RowsAffected` 方法判断影响行数 |

### 15.2 连接池 Pool 的方法

| 方法 | 签名 | 用途 | 本工程是否使用 |
|---|---|---|---|
| `Exec` | `Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)` | 执行不返回结果行的语句 | 使用。`CreateLink`、`AddClicks`、`DeleteLink`，以及已经写好的 `EnsureSchema` |
| `QueryRow` | `QueryRow(ctx context.Context, sql string, args ...any) pgx.Row` | 执行只取一行的语句 | 使用。`GetLink`、`CountLinks`、`SumClicks` |
| `Query` | `Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)` | 执行返回多行的语句 | 使用。`ListLinks` |
| `Ping` | `Ping(ctx context.Context) error` | 确认连接可达 | 已经写好。`Postgres.Ping` 与就绪探针使用它 |
| `Close` | `Close()` | 关闭池中的全部连接 | 已经写好。`Postgres.Close` 使用它 |
| `Begin` | `Begin(ctx context.Context) (pgx.Tx, error)` | 开启一个显式事务 | 本工程不使用，原因见 15.6 节 |
| `Acquire` | `Acquire(ctx context.Context) (*pgxpool.Conn, error)` | 从池中直接借出一个连接 | 本工程不使用 |

关于 `QueryRow` 有一处需要记住的机制：它的签名之中没有 error 返回值。语句执行失败或者没有查到行时，这些错误会延迟到调用 `Scan` 的那一刻才暴露出来。因此 `GetLink` 的错误分支实际上是 `Scan` 返回值的分支，而不是 `QueryRow` 返回值的分支，这是它与 `Query` 最明显的差别。

### 15.3 三个接口的契约

`pgx.Row` 接口只声明了一个方法，它的契约如下。

| 方法 | 契约 |
|---|---|
| `Scan(dest ...any) error` | 没有查到行时返回 `pgx.ErrNoRows`；查到多行时只使用第一行，其余行被忽略 |

`pgx.Rows` 接口的方法与契约如下，其中前四个在 `ListLinks` 中都要用到。

| 方法 | 契约 |
|---|---|
| `Next() bool` | 把游标移动到下一行，有行可读时返回 true。返回 false 时有两种可能：全部行已经读完，或者发生了致命错误。这两种情况下 Rows 都会被自动关闭 |
| `Scan(dest ...any) error` | 把当前行按位置扫描到目标变量。在 `Next` 返回 true 之前调用它是错误的用法。目标变量的数量必须与结果集的列数相等 |
| `Err() error` | 返回执行语句或者读取结果过程中发生的错误。文档强调：**必须在 Rows 关闭之后调用**；如果在关闭之前调用，那么即使服务器端执行已经失败，它也可能返回 nil。检测「迭代是否因为错误提前结束」只能依靠它 |
| `Close()` | 关闭结果集并且把连接归还给池，重复调用是安全的 |
| `CommandTag() pgconn.CommandTag` | 返回命令标签，只在 Rows 关闭之后可用 |
| `FieldDescriptions() []pgconn.FieldDescription` | 返回各列的元信息，包括列名与数据类型 OID |
| `Values() ([]any, error)` | 把当前行解码成一个取值切片，本工程不使用 |

有了这份契约，`ListLinks` 的骨架可以概括为四步：调用 `Query` 取得 Rows、紧接着 `defer rows.Close()`、循环调用 `Next` 与 `Scan` 把每一行追加到切片、循环结束后检查 `rows.Err()`。其中 `defer` 与 `rows.Err()` 这两步正是容易遗漏的两步，它们各自的依据在 15.3 节与 15.7 节中说明。

`pgconn.CommandTag` 的方法如下，本工程只使用其中一个。

| 方法 | 用途 |
|---|---|
| `RowsAffected() int64` | 返回语句影响的行数。`UPDATE` 与 `DELETE` 在影响 0 行时不返回错误，只能依靠这个取值判断是否命中记录 |
| `String() string` | 返回原始标签文本，例如 `INSERT 0 1`、`UPDATE 1`、`DELETE 1` |

### 15.4 错误的分类与对应的判断方式

| 错误的种类 | Go 侧的类型或者取值 | 判断方式 |
|---|---|---|
| 没有查到行 | `pgx.ErrNoRows` | `errors.Is(err, pgx.ErrNoRows)` |
| 数据库返回的错误，例如约束冲突与语句错误 | `*pgconn.PgError` | `errors.As(err, &pgErr)` 之后再检查它的字段 |
| 连接建立失败 | `*pgconn.ConnectError` | `errors.As(err, &connErr)`，它的 `Unwrap` 方法返回底层的网络错误 |
| 超时 | 被内部类型 `errTimeout` 包装 | `pgconn.Timeout(err)` |

`pgconn.PgError` 的字段很多，与本工程相关的部分如下。

| 字段 | 含义 |
|---|---|
| `Code` | SQLSTATE 错误码，例如 `23505` |
| `Severity` | 严重级别，取值是 `ERROR`、`FATAL`、`PANIC` |
| `Message` | 主要错误信息 |
| `Detail` | 详细说明。主键冲突时这里会写出重复的那个键值 |
| `ConstraintName` | 被违反的约束名称。主键冲突时取值是 `links_pkey` |
| `TableName` | 相关的表名 |
| `Hint` | 服务器给出的修改建议 |

`PgError` 的 `Error()` 方法返回的文本形式是「严重级别 + 冒号 + 消息 + 括号中的 SQLSTATE 码」。这一点解释了为什么不能依靠错误文本来判断错误类型：文本是为了让人阅读而拼接的，而 `Code` 字段是由数据库规范固定的取值。如果判断条件写得足够严格，可以在 `Code` 等于 `23505` 的基础上再检查 `ConstraintName` 是否等于 `links_pkey`，从而把「短码重复」与「其他唯一性冲突」区分开。

SQLSTATE `23505` 的含义是 unique_violation，它属于类别 `23`，也就是完整性约束违例。同一类别下的其他常见取值包括 `23503` 外键违例与 `23502` 非空违例。

### 15.5 本工程用到的 SQL 语句与特性

七个方法各自对应的语句如下。

| 方法 | SQL 语句 | 用到的特性 |
|---|---|---|
| `CreateLink` | `INSERT INTO links (code, url) VALUES ($1, $2)` | 占位符；不写出的列使用建表语句中的默认值 |
| `GetLink` | `SELECT code, url, clicks, created_at FROM links WHERE code = $1` | 主键查询；列顺序必须与扫描顺序一致 |
| `ListLinks` | `SELECT code, url, clicks, created_at FROM links ORDER BY created_at DESC, code DESC LIMIT $1 OFFSET $2` | 多列排序；分页 |
| `CountLinks` | `SELECT count(*) FROM links` | 聚合函数 |
| `SumClicks` | `SELECT coalesce(sum(clicks), 0) FROM links` | 聚合函数与 NULL 处理 |
| `AddClicks` | `UPDATE links SET clicks = clicks + $2 WHERE code = $1` | 表达式更新与原子性 |
| `DeleteLink` | `DELETE FROM links WHERE code = $1` | 依靠影响行数判断是否命中 |

这些语句涉及的 SQL 特性如下。

| 特性 | 说明 | 为什么本工程需要它 |
|---|---|---|
| 占位符写作 `$1`、`$2` | PostgreSQL 使用按位置编号的占位符，MySQL 使用问号 | 把取值与语句结构分开，因此用户提交的长网址无法改变语句结构，这是避免 SQL 注入的方式 |
| 未加引号的标识符会被折叠成小写 | `SELECT Code FROM Links` 会被服务器当作 `select code from links` 执行 | 建表语句中的列名是小写，因此 SQL 中也统一写小写，避免依赖折叠规则 |
| `count(*)` 的返回类型是 `bigint` | 计数结果不会超出 `int64` 的范围 | 可以直接扫描到 `int64` 变量 |
| `sum(clicks)` 的返回类型是 `numeric` 而不是 `bigint` | PostgreSQL 的 `sum(bigint)` 重载返回 `numeric`，目的是避免累加过程溢出 | pgx 会把 `numeric` 解码到 `int64` 目标变量，前提是该取值没有小数部分并且落在 `int64` 范围内；超出范围时返回扫描错误 |
| `coalesce(取值, 备选)` | 第一个参数为 NULL 时返回第二个参数，否则返回第一个参数 | 表中没有记录时 `sum(clicks)` 的结果是 NULL，而把 NULL 扫描到 `int64` 变量会直接返回错误，于是统计接口在空表状态下会返回 503 |
| `ORDER BY 列1 DESC, 列2 DESC` | 依次比较多个排序键，只有在第一个键相等时才比较第二个键 | 若干条记录的 `created_at` 完全相同时，仅按 `created_at` 排序的顺序不确定，相邻两页可能重复或者遗漏记录 |
| `LIMIT n OFFSET m` | 先跳过 m 行，再取 n 行 | 分页的两个参数直接使用调用方传入的 `limit` 与 `offset` |
| `clicks = clicks + $2` | 在数据库端读取当前取值并且相加，而不是在 Go 进程里完成 | 「读取、相加、写回」三步之间存在时间窗口，两个写回协程同时执行时后写的结果会覆盖先写的结果 |
| 单条语句自带隐式事务 | PostgreSQL 中每一条语句自身运行在一个事务里，语句结束时自动提交 | 本工程的七个方法各自只发送一条语句，因此不需要显式事务 |

### 15.6 事务与隔离级别的边界条件

本工程不使用显式事务，依据在上面那张表的最后一行：每一个方法只发送一条语句，而单条语句在 PostgreSQL 中已经是原子的。

需要说明的边界条件有两个。第一个条件是删除操作需要同时删除 PostgreSQL 中的记录与 Redis 中的两个键，这两个存储系统不属于同一个事务管理器，因此这件事无法用数据库事务保证。本工程的处理方式是由调用方按顺序执行，如果中途失败就会留下「数据库已经删除、缓存仍然存在」这种不一致状态，等待缓存键的生存时间到期之后自动消除。第二个条件是 `clicks = clicks + $2` 的原子性来自于行级锁：在 PostgreSQL 的默认隔离级别 READ COMMITTED 下，`UPDATE` 会对命中的行加行级锁，第二个执行同一语句的事务会等待第一个提交，然后基于最新的行版本重新求值，因此两次累加的结果是叠加而不是覆盖。

### 15.7 连接池与 Rows 生命周期之间的关系

连接池的参数已经写在 `NewPostgres` 中：`MaxConns = 10`、`MinConns = 1`。调用 `Query` 时连接池会借出一个连接，只有在该 `Rows` 关闭之后这个连接才会归还。由此可以推出两条结论。

第一条结论：如果实现 `ListLinks` 时忘记调用 `Close`，并且没有让 `Next` 迭代到结束，那么每一次请求都会长期占用一个连接，请求数量累积到 10 之后，后续请求会全部阻塞在等待连接上。这就是必须写出 `defer rows.Close()` 的机制依据，它的作用不只是代码风格。

第二条结论：`rows.Err()` 必须在 Rows 关闭之后调用，因为 `Next` 返回 false 时 Rows 被自动关闭，而只有关闭动作才会把服务器端返回的错误写入 `err` 字段，因此在循环结束后紧接着检查 `rows.Err()` 正好满足这个时序要求。

关于 `ctx` 参数：七个方法的第一个参数都是 `ctx`，pgx 在语句执行期间监视它。当 `ctx` 被取消或者超时时，pgx 会向服务器发送一个取消请求，并且把这条连接标记为不可用，因此调用方传入带超时的 `ctx` 可以让慢查询被中断，而不是一直占用连接。

### 15.8 本工程不使用的功能

| 功能 | 说明 | 本工程不使用的原因 |
|---|---|---|
| `Begin` 与 `pgx.Tx` | 显式事务 | 每个方法只发送一条语句，而单条语句已经是原子的 |
| `Acquire` 与 `pgxpool.Conn` | 从池中直接借出连接 | 本工程的方法都是一条语句结束之后立即归还连接，不需要跨越多次调用的连接 |
| `Batch` 与 `SendBatch` | 把多条语句放在一次网络往返中执行 | 本工程的语句之间没有这种批量关系 |
| `CopyFrom` | 批量导入数据 | 本工程没有批量写入的场景 |
| `CollectRows`、`CollectOneRow`、`CollectExactlyOneRow`、`ForEachRow`、`RowToStructByPos` 等泛型辅助函数 | 它们可以替代手写的 `Next` 与 `Scan` 循环，并且会自动关闭 Rows | 本工程的骨架刻意要求手写循环，目的是让「关闭 Rows」与「检查 `rows.Err()`」这两个步骤显式地出现在代码中 |

---

## 16. Redis 与 go-redis

本工程通过 `github.com/redis/go-redis/v9` 访问 Redis，版本是 v9.22.0。本节从「Redis 是什么」开始，覆盖第 3 组、第 6 组与第 8 组要用到的全部内容。

### 16.1 Redis 是什么，以及它的执行模型

| 概念 | 说明 |
|---|---|
| 它是什么 | 一个把数据全部存放在内存中的键值存储服务。客户端通过 TCP 连接向它发送文本形式的命令，它执行之后返回结果 |
| 键与值的数据类型 | 键永远是字符串；值有若干种数据类型：字符串、列表、哈希、集合、有序集合、位图、HyperLogLog、流。本工程只使用字符串这一种 |
| 单线程执行命令 | 它用一个线程串行执行到达的命令，因此单条命令的处理过程不会被打断 |
| 内存与持久化 | 数据默认全部在内存；它也可以把数据落到磁盘（RDB 快照与 AOF 日志），但本工程不配置持久化，因此进程重启会丢掉数据 |
| 为什么在本工程里需要它 | 第一是读取快，一次查询就是一次内存访问；第二是它能自动过期，每个键可以设置生存时间 |
| 它不是关系型数据库 | 没有表结构、没有外键、不支持多列条件查询；它也不保证数据不丢 |

单线程执行命令这条性质有两条直接推论，它们解释了本工程的多处写法。

| 推论 | 在本工程中的体现 |
|---|---|
| 单条命令天然是原子的，不需要加锁 | `IncrClick` 直接使用 `INCR` 命令，不要「先读取、在 Go 里加一、再写回」，后一种写法存在时间窗口，两个请求同时执行时可能只留下一次的结果 |
| 一条耗时的命令会阻塞其他所有命令 | 枚举键必须使用 `SCAN` 而不是 `KEYS`。`KEYS` 在一次调用中遍历整个键空间，遍历期间服务端不对其他命令作出响应 |

### 16.2 本工程用到的四类键

| 键名模式 | 值的内容 | 生存时间 | 谁写入 | 谁读取 |
|---|---|---|---|---|
| `link:{code}` | 原始长网址 | 300 秒 | 重定向未命中数据库时回填 | 重定向 |
| `clicks:{code}` | 尚未写回数据库的点击增量 | 不设置 | 每次成功重定向时自增 | 后台写回协程 |
| `stats:cache_hits` | 累计命中次数 | 不设置 | 每次缓存命中时自增 | 统计接口 |
| `stats:cache_misses` | 累计未命中次数 | 不设置 | 每次缓存未命中时自增 | 统计接口 |

关于生存时间的机制：它由服务端维护，到期之后键被删除，客户端不需要做任何事。设置它的意义是「这类数据可以随时从数据库重建，因此可以自动释放内存」；不设置它的意义是「这类取值需要长期累计，过期会让统计清零」。`TTL` 命令返回剩余的秒数，返回 -1 表示键存在但没有设置生存时间，返回 -2 表示键不存在。

### 16.3 本项目用到的 Redis 命令

| 操作 | 命令 | 语义要点 |
|---|---|---|
| 读取一个键 | `GET` | 键不存在时返回的不是空字符串，而是一个约定的空回复，客户端把它呈现为 `redis.Nil` |
| 写入一个键并设置生存时间 | `SET` 加 `EX` | 生存时间参数取 0 时表示不设置，也就是永久存在 |
| 删除一个或多个键 | `DEL` | 返回被删除的键数量，取值可以是 0；删除不存在的键不算错误 |
| 自增 | `INCR` | 把值按整数解析之后加一；键不存在时先视为 0 |
| 扣除 | `DECRBY` | 把值减去指定的整数；结果可以是 0，也可以是负数 |
| 一次读取多个键 | `MGET` | 返回一个数组，长度与请求的键数量相同，位置与请求顺序一致；不存在的键在对应位置返回空 |
| 批量枚举键 | `SCAN` | 每次调用返回一批键与「下一个游标」，游标回到 0 时迭代结束 |
| 查询生存时间 | `TTL` | 返回剩余秒数；键不存在时返回 -2 |
| 判断键是否存在 | `EXISTS` | 返回存在的键数量 |

### 16.4 go-redis 的 API 结构

| 层次 | 内容 |
|---|---|
| 客户端 | `redis.NewClient(&redis.Options{...})` 返回 `*redis.Client`。`Options` 中本工程用到三个字段：`Addr`、`Password`、`DB`。`DB` 是逻辑数据库编号，同一个 Redis 实例可以被多个项目共用，用编号隔离 |
| 连接维护 | `client.Ping(ctx)` 返回 `*redis.StatusCmd`；`client.Close()` 关闭连接池 |
| 命令方法 | 客户端的方法与 Redis 命令同名，首字母大写：`Get`、`Set`、`Del`、`Incr`、`DecrBy`、`MGet`、`Scan`、`Exists`、`TTL` |
| 命令对象 | 命令方法不直接返回取值，而是返回一个命令对象。例如 `Get` 返回 `*redis.StringCmd` |
| 取值 | 命令对象提供 `Result()` 与 `Err()` 两个方法：`Result()` 返回「取值与错误」两个结果，`Err()` 只返回错误 |
| 第一个参数 | 全部命令方法的第一个参数都是 `ctx`，与存储层一致 |

命令与返回类型的对应关系如下。

| 命令 | 返回类型 | `Result()` 的返回值 |
|---|---|---|
| `GET` | `*redis.StringCmd` | 字符串与错误；键不存在时错误是 `redis.Nil` |
| `SET` | `*redis.StatusCmd` | 状态文本（通常是 `OK`）与错误 |
| `DEL` | `*redis.IntCmd` | 被删除的键数量（`int64`）与错误 |
| `INCR` | `*redis.IntCmd` | 自增之后的取值（`int64`）与错误 |
| `DECRBY` | `*redis.IntCmd` | 减去之后的取值（`int64`）与错误 |
| `MGET` | `*redis.SliceCmd` | `[]interface{}` 与错误；不存在的键对应 `nil` |
| `SCAN` | `*redis.ScanCmd` | 这一批键（`[]string`）、下一个游标（`uint64`）与错误 |
| `TTL` | `*redis.DurationCmd` | `time.Duration` 与错误 |
| `EXISTS` | `*redis.IntCmd` | 存在的键数量（`int64`）与错误 |

关于 `redis.Nil`：它的类型是 `proto.RedisError`，取值就是文本 `redis: nil`，在 go-redis 中声明为一个常量。它表示的不是「连接 Redis 失败」，而是服务端对不存在的键给出的一个约定空回复。判断方式与存储层的哨兵错误相同，使用 `errors.Is(err, redis.Nil)`。

关于 `SliceCmd` 为什么返回 `[]interface{}` 而不是 `[]string`：`MGET` 的返回位置可能为空，因此每个元素既可能是字符串也可能是 `nil`，类型上只能用 `interface{}` 表达。使用之前必须先判断它是不是 `nil`，再做类型断言。

关于 `ScanCmd`：它还有一个 `Iterator()` 方法，返回一个可以逐批取键的迭代器。本工程不使用它，因为本组的实现要求是明确写出游标迭代，以便看清「迭代结束」的判断条件。

### 16.5 SCAN 的游标迭代语义

`SCAN` 的调用方式是「传入游标，取回一批键与下一个游标」，循环直到游标回到 0。下面是一次完整的迭代过程举例。

| 调用次序 | 传入的游标 | 返回的键 | 返回的游标 | 说明 |
|---|---|---|---|---|
| 第 1 次 | 0 | `clicks:a1B2c3`、`clicks:d4E5f6` | 17 | 第一次调用必须以 0 作为起始游标 |
| 第 2 次 | 17 | `clicks:g7H8i9` | 0 | 返回的游标是 0，说明迭代结束 |

关于 `SCAN` 有四条需要记住的语义。

| 语义 | 说明 |
|---|---|
| 这一批可能为空 | 服务端只保证遍历完整个键空间，不保证每一批都有键。因此「这一批没有命中任何键」不能作为「迭代结束」的判断依据，判断依据只能是返回的游标是否为 0 |
| 每一批的键数量不是固定的 | 调用时传入的数量只是一个提示值，服务端可以返回更多或更少的键 |
| 一致性是弱的 | 在一轮完整迭代中，始终存在的键一定会被返回至少一次；而迭代期间新增或删除的键可能被返回，也可能不被返回 |
| 它不会阻塞其他命令 | 因为每次调用只处理一部分键并且立即返回，这是它相对于 `KEYS` 的唯一优势 |

### 16.6 八个方法各自的实现要点

| 方法 | 使用的命令 | 返回值的处理 |
|---|---|---|
| `GetURL` | `GET` | `Result()` 的错误满足 `errors.Is(err, redis.Nil)` 时返回 `ErrMiss`，其余错误原样返回 |
| `SetURL` | `SET` | 只需要判断错误 |
| `DeleteLink` | `DEL`，一次传两个键名 | 只需要判断错误；返回值是删除数量，取值为 0 不算错误 |
| `IncrClick` | `INCR` | 只需要判断错误 |
| `RecordCacheResult` | `INCR`，按命中与否选择键 | 只需要判断错误 |
| `CacheStats` | `MGET`，一次传两个键 | 对每个元素先判断是否为 `nil`，再断言成 `string` 并用 `strconv.ParseInt` 转换；取值不是合法整数时返回错误 |
| `CollectClicks` | `SCAN` 加批量读取 | 循环直到游标回到 0；对每个键取出取值并转换 |
| `SubtractClicks` | `DECRBY` | 只需要判断错误 |

### 16.7 本工程不使用的功能

| 功能 | 说明 | 本工程不使用的原因 |
|---|---|---|
| 事务（`MULTI` 与 `EXEC`）与 Lua 脚本 | 把多条命令当成一个整体执行 | 本工程需要原子性的操作都只有单条命令 |
| 管道（`Pipeline`） | 把多条命令放在一次网络往返中执行 | 本工程的方法是「一次调用对应一次往返」的形式 |
| 发布订阅（`PubSub`） | 服务端主动向客户端推送消息 | 本工程没有事件通知的需求 |
| `KEYS` | 一次返回全部匹配的键 | 它会阻塞服务端，一律改用 `SCAN` |
| 持久化配置 | RDB 快照与 AOF 日志 | 本工程依赖容器默认配置，因此 Redis 重启之后统计键会清零，这是需要写进笔记的边界条件 |
