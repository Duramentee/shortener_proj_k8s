// Package store 负责与 PostgreSQL 交互。
//
// 本层只做三件事：建立连接池、保证数据表存在、执行四条基础语句（插入、查询、更新、删除）。
// 它不做任何参数校验，也不生成短码，那些属于处理函数与 shortcode 包的职责。
// 这样划分的目的是让「数据在哪里、以什么形式存在」这件事只在这一个包里出现。
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound 表示查询的短码在数据库中不存在。
//
// 引入这个独立错误的原因是调用方必须区分两种完全不同的情况：
// 「短码不存在」应当返回 HTTP 状态码 404，而「数据库查询出错」应当返回 503 或者 500。
// 这两种情况在错误文本上可能都包含「no rows」之类的字样，因此不能靠文本判断，
// 必须由存储层显式地返回一个可以用 errors.Is 判断的哨兵错误。
var ErrNotFound = errors.New("短码不存在")

// ErrConflict 表示插入的短码与已有记录冲突，也就是 links 表的主键约束被违反。
//
// 它由 CreateLink 在数据库返回 SQLSTATE 23505 时返回，调用方据此决定是否重新生成短码再试一次。
// 与 ErrNotFound 一样，它是一个可以用 errors.Is 判断的哨兵错误；
// 数据库返回的原始 *pgconn.PgError 依然保留在同一条错误链中，因此也可以用 errors.As 取出。
var ErrConflict = errors.New("主键冲突")

// Link 是 links 表在 Go 侧对应的结构体。
// 字段名与列名的对应关系见 README 第 3 节的数据模型。
type Link struct {
	Code      string
	URL       string
	Clicks    int64
	CreatedAt time.Time
}

// Postgres 封装连接池，并且对外提供数据访问方法。
// 之所以把连接池放在结构体里而不是使用包级变量，是为了让测试可以替换掉它，
// 也为了避免在包被导入时就产生副作用。
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres 建立连接池并且确认数据库可达。
//
// 本函数已经写好，你不需要修改它。它做了四件事：
// 解析连接字符串、设置连接池参数、创建连接池、执行一次 Ping 确认连接可用。
// 其中 Ping 这一步是必要的：pgxpool.NewWithConfig 只创建池对象而不建立真实连接，
// 如果省略 Ping，那么数据库地址填写错误时程序会正常启动，
// 直到第一个请求到达才报错，这会把配置错误伪装成运行期故障。
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析数据库连接串失败，请检查 DATABASE_URL 的取值格式：%w", err)
	}

	// 连接池的上限与回收策略。取值偏保守，因为本项目的并发量很低，
	// 把上限设得过大只会让数据库端的连接数变多，而不会提升吞吐。
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 1
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接池失败：%w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连接数据库失败，请检查数据库是否已经启动以及 DATABASE_URL 的取值：%w", err)
	}

	return &Postgres{pool: pool}, nil
}

// Close 关闭连接池。进程退出之前应当调用它。
func (p *Postgres) Close() {
	p.pool.Close()
}

// Ping 检查数据库是否可达，供就绪探针使用。
func (p *Postgres) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Host 返回连接池实际使用的主机名。
// 启动日志中输出它可以快速确认应用连向了哪一个地址，
// 阶段 6 制造「依赖地址填写错误」这类故障时，这一行日志是定位问题的第一条线索。
func (p *Postgres) Host() string {
	return p.pool.Config().ConnConfig.Host
}

// Name 返回连接池实际使用的数据库名称。
func (p *Postgres) Name() string {
	return p.pool.Config().ConnConfig.Database
}

// 建表语句。两条语句分开执行，而不是拼成一条多语句字符串，
// 原因是多语句在同一个 Exec 调用中的行为取决于连接使用的查询协议，分开执行可以得到确定的语义。
const createTableDDL = `
CREATE TABLE IF NOT EXISTS links (
    code       varchar(16) PRIMARY KEY,
    url        text        NOT NULL,
    clicks     bigint      NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
)`

const createIndexDDL = `
CREATE INDEX IF NOT EXISTS links_created_at_idx ON links (created_at DESC)`

// EnsureSchema 保证数据表与索引存在。本函数已经写好，你不需要修改它。
//
// 语句全部使用 IF NOT EXISTS 形式，因此本函数可以重复执行而不会报错，
// 这就是 README 第 3 节所说的「容许重复执行的建表语句」：
// 部署顺序不再需要人为编排，后端每次启动都会自己确认表结构存在。
func (p *Postgres) EnsureSchema(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, createTableDDL); err != nil {
		return fmt.Errorf("创建 links 表失败：%w", err)
	}
	if _, err := p.pool.Exec(ctx, createIndexDDL); err != nil {
		return fmt.Errorf("创建 links_created_at_idx 索引失败：%w", err)
	}
	return nil
}

// CreateLink 向 links 表插入一条记录。
//
// 实现要求：
//  1. 只写入 code 与 url 两列。clicks 与 created_at 由建表语句中的默认值提供，
//     也就是 0 与 now()，因此不要在插入语句里显式给它们赋值。
//  2. 参数必须使用 $1、$2 这样的占位符，不要把取值拼接进 SQL 文本。
//     拼接会让用户提交的网址有机会改变语句结构，这是 SQL 注入的成因。
//  3. 短码已经存在时数据库会返回主键冲突错误，此时返回的错误必须让调用方能够用
//     errors.Is 判断出这是冲突，因此需要使用 pgx 提供的错误类型
//     github.com/jackc/pgx/v5/pgconn 中的 *pgconn.PgError，判断它的 Code 字段是否等于 "23505"。
//     调用方依靠这个判断决定是否重新生成短码再试一次。
//
// 提示：使用 p.pool.Exec。
func (p *Postgres) CreateLink(ctx context.Context, link Link) error {
	const insertSQL = "INSERT INTO links (code, url) VALUES ($1, $2)"

	if _, err := p.pool.Exec(ctx, insertSQL, link.Code, link.URL); err != nil {
		// 包装的对象必须是原始的 err，而不是从它里面取出的 pgErr。
		// 原因有两个：第一，err 之外可能还包着更外层的错误，直接返回 pgErr 会把那一层丢掉；
		// 第二，%w 建立的错误链可以被 errors.As 逐层展开，
		// 因此包装之后调用方依然能够取出内部的 *pgconn.PgError 并检查它的 Code 字段。
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// 同一条语句中可以使用两个 %w（Go 1.20 起支持），返回的错误因此具有两条身份：
			// 调用方可以用 errors.Is(err, ErrConflict) 判断「短码已被占用」，
			// 也可以用 errors.As(err, &pgErr) 取出 SQLSTATE 做更细的判断。
			return fmt.Errorf("%w：插入短码 %s 失败：%w", ErrConflict, link.Code, err)
		}

		return fmt.Errorf("插入短码 %s 失败：%w", link.Code, err)
	}

	return nil
}

// GetLink 按短码查询一条记录。
//
// 实现要求：
//  1. 查询 code、url、clicks、created_at 四列，并且按与 Link 结构体字段相同的顺序扫描。
//  2. 没有查到记录时必须返回 ErrNotFound，而不是把 pgx 的原始错误直接返回。
//     判断方式是 errors.Is(err, pgx.ErrNoRows)，pgx 的导入路径是 github.com/jackc/pgx/v5。
//  3. 其余错误原样返回。调用方需要区分「不存在」与「数据库出错」这两种情况。
//
// 提示：使用 p.pool.QueryRow 与 Scan。
func (p *Postgres) GetLink(ctx context.Context, code string) (Link, error) {
	var link Link

	const selectSQL = "SELECT code, url, clicks, created_at FROM links WHERE code = $1"

	// QueryRow 的签名中没有 error 返回值，因此语句执行阶段的错误与
	// 「没有查到行」这两种情况都会延迟到调用 Scan 的时候才暴露出来。
	// 语句中使用了 $1，因此第二个参数之后必须按顺序传入与占位符对应的取值。
	row := p.pool.QueryRow(ctx, selectSQL, code)

	// Scan 的目标必须是指针，数量必须等于结果集的列数，
	// 顺序必须与 SELECT 中列出的顺序一致，也就是与 Link 结构体的字段顺序一致。
	if err := row.Scan(&link.Code, &link.URL, &link.Clicks, &link.CreatedAt); err != nil {
		// 出错时返回 Link{} 而不是 link。Scan 是逐列写入的，
		// 如果某一列失败，位于它之前的列可能已经被写入 link，
		// 直接返回 link 相当于把一个只填了一部分的记录交给调用方。
		if errors.Is(err, pgx.ErrNoRows) {
			// 只包装 ErrNotFound。短码不存在是可预期的结果而不是故障，
			// 因此不需要把 pgx.ErrNoRows 也放进错误链中。
			return Link{}, fmt.Errorf("短码 %s 不存在：%w", code, ErrNotFound)
		}

		// 其余错误按第 3 条要求原样向上传递，只在文本中补充本次使用的短码。
		return Link{}, fmt.Errorf("查询短码 %s 失败：%w", code, err)
	}

	return link, nil
}

// ListLinks 按创建时间倒序分页查询记录。
//
// 实现要求：
//  1. 排序方式必须与索引 links_created_at_idx 的定义一致，也就是 created_at DESC。
//  2. 分页使用 LIMIT 与 OFFSET 两个参数。
//  3. code 的取值必须满足唯一性，因此当若干条记录的 created_at 完全相同时，
//     仅按 created_at 排序会让相邻两页出现重复或者遗漏的记录。
//     正确的写法是在排序条件里追加一个唯一的列作为次级排序条件，例如 created_at DESC, code DESC。
//  4. 查询结果为空时返回长度为 0 的切片而不是 nil，
//     因为处理函数要把它序列化成 JSON 数组，nil 切片会被序列化成 null 而不是 []。
//
// 提示：使用 p.pool.Query 取得 pgx.Rows，然后调用 rows.Next 与 rows.Scan 逐行读取，
// 最后必须检查 rows.Err()，因为迭代过程中发生的错误只会通过它暴露出来。
func (p *Postgres) ListLinks(ctx context.Context, limit int, offset int) ([]Link, error) {
	// 用 make 建立长度为 0、容量为 limit 的切片，而不是用 var 声明。
	// var 声明出来的是 nil 切片，它序列化之后会变成 null，
	// 而接口契约规定 items 字段始终是数组。
	links := make([]Link, 0, limit)

	const selectSQL = "SELECT code, url, clicks, created_at FROM links ORDER BY created_at DESC, code DESC LIMIT $1 OFFSET $2"

	rows, err := p.pool.Query(ctx, selectSQL, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询列表失败：%w", err)
	}

	// 连接在结果集关闭之后才归还给连接池，因此这一步不能省略。
	defer rows.Close()

	// Next 同时承担「推进游标」与「报告还有没有下一行」两件事，
	// 因此它直接写在 for 的条件位置，循环体内部不需要再判断。
	for rows.Next() {
		var link Link

		// 与 GetLink 一致：目标是字段的地址，数量与顺序必须与 SELECT 中列出的列一致。
		if err := rows.Scan(&link.Code, &link.URL, &link.Clicks, &link.CreatedAt); err != nil {
			return nil, fmt.Errorf("读取列表记录失败：%w", err)
		}

		links = append(links, link)
	}

	// Next 返回 false 有两种可能：全部行已经读完，或者中途发生了错误。
	// 区分这两者只能依靠 Err()，并且它必须在结果集关闭之后调用。
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历列表记录失败：%w", err)
	}

	return links, nil
}

// CountLinks 返回 links 表中的记录总数，供统计接口使用。
//
// 实现要求：使用 count(*) 聚合函数，并且把结果扫描到 int64 变量中。
//
// 提示：使用 p.pool.QueryRow 与 Scan。
func (p *Postgres) CountLinks(ctx context.Context) (int64, error) {
	var cnt int64

	const countSQL = "SELECT count(*) FROM links"

	row := p.pool.QueryRow(ctx, countSQL)

	if err := row.Scan(&cnt); err != nil {
		return 0, fmt.Errorf("查询记录总数失败：%w", err)
	}

	return cnt, nil
}

// SumClicks 返回 links 表中 clicks 列的总和，供统计接口使用。
//
// 实现要求：使用 coalesce(sum(clicks), 0) 而不是 sum(clicks)。
// 原因是表中没有任何记录时 sum(clicks) 的结果是 NULL，
// 把 NULL 扫描到 int64 变量里会直接返回错误。coalesce 的作用是把 NULL 替换成第二个参数。
//
// 提示：使用 p.pool.QueryRow 与 Scan。
func (p *Postgres) SumClicks(ctx context.Context) (int64, error) {
	var total int64

	const sumClicksSQL = "SELECT coalesce(sum(clicks), 0) FROM links"

	row := p.pool.QueryRow(ctx, sumClicksSQL)

	if err := row.Scan(&total); err != nil {
		return 0, fmt.Errorf("查询点击总数失败：%w", err)
	}

	return total, nil
}

// AddClicks 把指定的增量累加到某条记录的 clicks 列上。
// 本函数由后台写回协程调用，请求处理路径不会调用它。
//
// 实现要求：
//  1. 使用 UPDATE links SET clicks = clicks + $2 WHERE code = $1 这种形式，
//     也就是把加法交给数据库执行。
//     不要先查询出当前取值、在 Go 里相加、再写回去，
//     因为「读取、相加、写回」这三步之间存在时间窗口，
//     两个写回协程同时执行时后写的结果会覆盖先写的结果，从而丢失一次增量。
//  2. 记录不存在时返回 ErrNotFound，供调用方判断是否需要清理对应的 Redis 计数键。
//
// 提示：使用 p.pool.Exec，并且通过返回的 CommandTag 的 RowsAffected 方法判断是否命中了记录。
func (p *Postgres) AddClicks(ctx context.Context, code string, delta int64) error {
	const updateSQL = "UPDATE links SET clicks = clicks + $2 WHERE code = $1"

	tag, err := p.pool.Exec(ctx, updateSQL, code, delta)
	if err != nil {
		return fmt.Errorf("累加短码 %s 的点击次数失败：%w", code, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("短码 %s 不存在：%w", code, ErrNotFound)
	}

	return nil
}

// DeleteLink 按短码删除一条记录。
//
// 实现要求：记录不存在时返回 ErrNotFound，供调用方决定是否返回 404。
//
// 提示：使用 p.pool.Exec 与 CommandTag 的 RowsAffected 方法。
func (p *Postgres) DeleteLink(ctx context.Context, code string) error {
	const deleteSQL = "DELETE FROM links WHERE code = $1"

	// DELETE 与 UPDATE 在「没有命中任何记录」时不会返回错误，
	// 只返回一个影响行数为 0 的命令标签。因此「记录不存在」这个分支
	// 只能通过 CommandTag 的 RowsAffected 方法判断，不能依靠 err。
	tag, err := p.pool.Exec(ctx, deleteSQL, code)
	if err != nil {
		return fmt.Errorf("删除短码 %s 失败：%w", code, err)
	}

	// 影响行数为 0 才说明没有记录被删除。
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("短码 %s 不存在：%w", code, ErrNotFound)
	}

	return nil
}
