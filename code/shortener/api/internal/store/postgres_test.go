package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// 本文件是需要真实 PostgreSQL 的集成测试。
//
// 运行方式：设置 TEST_DATABASE_URL 之后再执行 go test，例如
//   TEST_DATABASE_URL='postgres://shortener:shortener_dev_password@localhost:5432/shortener?sslmode=disable' go test ./internal/store/...
// 未设置该环境变量时全部测试会被跳过，因此 go test ./... 在没有数据库的机器上也可以正常通过。
//
// 测试使用的短码统一带 zzstore 前缀，与真实短码（固定六位）不会冲突，
// 并且每条测试记录都在 t.Cleanup 中删除，重复运行不会留下残留数据。

const testCodePrefix = "zzstore"

// newTestStore 建立用于测试的连接，并且保证数据表存在。
func newTestStore(t *testing.T) *Postgres {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 TEST_DATABASE_URL，跳过需要真实数据库的测试")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pg, err := NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败：%v", err)
	}
	t.Cleanup(pg.Close)

	if err := pg.EnsureSchema(ctx); err != nil {
		t.Fatalf("确认数据表存在时失败：%v", err)
	}

	return pg
}

// insertTestLink 插入一条测试记录并且注册清理动作。
func insertTestLink(t *testing.T, pg *Postgres, code string, url string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 先删除同名的残留记录，保证测试可以重复运行。
	_ = pg.DeleteLink(ctx, code)

	if err := pg.CreateLink(ctx, Link{Code: code, URL: url}); err != nil {
		t.Fatalf("插入测试记录 %s 失败：%v", code, err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := pg.DeleteLink(cleanupCtx, code); err != nil {
			t.Logf("清理测试记录 %s 时失败：%v", code, err)
		}
	})
}

// TestCreateLinkAndGetLink 检查插入之后能够按短码读出全部四个字段。
func TestCreateLinkAndGetLink(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	const code = testCodePrefix + "Get"
	const url = "https://example.com/store/get"

	insertTestLink(t, pg, code, url)

	link, err := pg.GetLink(ctx, code)
	if err != nil {
		t.Fatalf("按短码 %s 查询失败：%v", code, err)
	}
	if link.Code != code {
		t.Errorf("查询结果的 code 是 %q，期望 %q", link.Code, code)
	}
	if link.URL != url {
		t.Errorf("查询结果的 url 是 %q，期望 %q", link.URL, url)
	}
	// 新插入的记录必须满足两个默认值：clicks 为 0，created_at 接近当前时间。
	// 这两条断言用来验证插入语句没有显式写入这两列。
	if link.Clicks != 0 {
		t.Errorf("新插入记录的 clicks 是 %d，期望默认值 0", link.Clicks)
	}
	if link.CreatedAt.IsZero() {
		t.Error("新插入记录的 created_at 是零值，说明读取时没有取到该列，或者插入时写入了零值")
	}
	if time.Since(link.CreatedAt) > time.Minute {
		t.Errorf("新插入记录的 created_at 是 %s，与当前时间相差过大", link.CreatedAt.Format(time.RFC3339))
	}
}

// TestGetLinkNotFound 检查查询不存在的短码时返回 ErrNotFound。
func TestGetLinkNotFound(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	_, err := pg.GetLink(ctx, testCodePrefix+"NoSuchRecord")
	if err == nil {
		t.Fatal("查询不存在的短码时没有返回错误")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("查询不存在的短码时返回的错误是 %v，期望它满足 errors.Is(err, ErrNotFound)", err)
	}
}

// TestCreateLinkDuplicate 检查重复插入同一个短码时返回 PostgreSQL 的唯一性约束错误。
//
// 处理函数依赖这个错误来判断「短码已被占用」，因此这里必须验证错误的形态：
// 它应当是一个 *pgconn.PgError，并且 SQLSTATE 取值是 23505。
func TestCreateLinkDuplicate(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	const code = testCodePrefix + "Dup"
	insertTestLink(t, pg, code, "https://example.com/store/dup/first")

	err := pg.CreateLink(ctx, Link{Code: code, URL: "https://example.com/store/dup/second"})
	if err == nil {
		t.Fatal("重复插入同一个短码时没有返回错误")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("重复插入时返回的错误是 %v，期望它能够被转换为 *pgconn.PgError", err)
	}
	if pgErr.Code != "23505" {
		t.Fatalf("重复插入时返回的 SQLSTATE 是 %s，期望 23505（唯一性约束冲突）", pgErr.Code)
	}
}

// TestListLinksOrderAndPaging 检查列表的排序方式与分页结果是否与整体顺序一致。
func TestListLinksOrderAndPaging(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	// 插入两条记录。为了得到确定的先后顺序，两条记录之间留出足够的时间间隔，
	// 避免 created_at 的取值过于接近而导致排序结果不确定。
	insertTestLink(t, pg, testCodePrefix+"Old", "https://example.com/store/old")
	time.Sleep(20 * time.Millisecond)
	insertTestLink(t, pg, testCodePrefix+"New", "https://example.com/store/new")

	full, err := pg.ListLinks(ctx, 1000, 0)
	if err != nil {
		t.Fatalf("查询列表失败：%v", err)
	}
	if len(full) < 2 {
		t.Fatalf("查询列表得到 %d 条记录，期望至少 2 条", len(full))
	}

	// 断言排序方式是 created_at 倒序：后一条记录的创建时间不得晚于前一条。
	for i := 1; i < len(full); i++ {
		if full[i].CreatedAt.After(full[i-1].CreatedAt) {
			t.Fatalf("列表中第 %d 条记录的创建时间是 %s，晚于第 %d 条记录的 %s，说明排序不是倒序",
				i+1, full[i].CreatedAt.Format(time.RFC3339Nano),
				i, full[i-1].CreatedAt.Format(time.RFC3339Nano))
		}
	}

	// 断言新记录出现在旧记录之前。
	newIndex, oldIndex := -1, -1
	for i, link := range full {
		switch link.Code {
		case testCodePrefix + "New":
			newIndex = i
		case testCodePrefix + "Old":
			oldIndex = i
		}
	}
	if newIndex < 0 || oldIndex < 0 {
		t.Fatalf("列表中缺少测试记录，newIndex=%d oldIndex=%d", newIndex, oldIndex)
	}
	if newIndex > oldIndex {
		t.Fatalf("新记录的位置是 %d，旧记录的位置是 %d，说明排序不是按创建时间倒序", newIndex, oldIndex)
	}

	// 断言逐条分页的结果与整体查询的前若干条一致。
	pages := min(3, len(full))
	for i := 0; i < pages; i++ {
		page, err := pg.ListLinks(ctx, 1, i)
		if err != nil {
			t.Fatalf("查询第 %d 页时失败：%v", i, err)
		}
		if len(page) != 1 {
			t.Fatalf("查询第 %d 页得到 %d 条记录，期望 1 条", i, len(page))
		}
		if page[0].Code != full[i].Code {
			t.Fatalf("查询第 %d 页得到记录 %s，整体查询的第 %d 条是 %s，说明分页与排序的组合不正确",
				i, page[0].Code, i, full[i].Code)
		}
	}
}

// TestListLinksEmptyResultIsNotNil 检查没有命中记录时返回的是空切片而不是 nil。
//
// 这一点直接影响列表接口的响应形态：nil 切片会被序列化成 null，
// 而接口契约规定 items 字段始终是数组。
func TestListLinksEmptyResultIsNotNil(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	// 取一个远大于表内记录数的偏移量，返回结果必然是空的。
	links, err := pg.ListLinks(ctx, 10, 1_000_000)
	if err != nil {
		t.Fatalf("使用很大的偏移量查询列表时失败：%v", err)
	}
	if links == nil {
		t.Fatal("空结果返回的是 nil，序列化之后会变成 null，应当返回长度为 0 的切片")
	}
	if len(links) != 0 {
		t.Fatalf("使用很大的偏移量查询得到 %d 条记录，期望 0 条", len(links))
	}
}

// TestCountLinksAndSumClicks 检查总数与点击总数的累加关系。
//
// 本测试不使用绝对值断言，而是比较插入前后的差值，
// 这样即使数据库中存在其他记录，测试结果也不受影响。
func TestCountLinksAndSumClicks(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	countBefore, err := pg.CountLinks(ctx)
	if err != nil {
		t.Fatalf("查询记录总数失败：%v", err)
	}
	sumBefore, err := pg.SumClicks(ctx)
	if err != nil {
		t.Fatalf("查询点击总数失败：%v", err)
	}

	const code = testCodePrefix + "Sum"
	insertTestLink(t, pg, code, "https://example.com/store/sum")

	if err := pg.AddClicks(ctx, code, 7); err != nil {
		t.Fatalf("累加点击次数失败：%v", err)
	}

	countAfter, err := pg.CountLinks(ctx)
	if err != nil {
		t.Fatalf("再次查询记录总数失败：%v", err)
	}
	sumAfter, err := pg.SumClicks(ctx)
	if err != nil {
		t.Fatalf("再次查询点击总数失败：%v", err)
	}

	if countAfter-countBefore != 1 {
		t.Errorf("记录总数的增量是 %d，期望 1（插入前后分别是 %d 与 %d）", countAfter-countBefore, countBefore, countAfter)
	}
	if sumAfter-sumBefore != 7 {
		t.Errorf("点击总数的增量是 %d，期望 7（累加前后分别是 %d 与 %d）", sumAfter-sumBefore, sumBefore, sumAfter)
	}
}

// TestAddClicksAccumulates 检查多次累加是叠加而不是覆盖。
func TestAddClicksAccumulates(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	const code = testCodePrefix + "Acc"
	insertTestLink(t, pg, code, "https://example.com/store/acc")

	// 连续累加三次。如果实现方式写成了「读取、相加、写回」，
	// 在没有并发的情况下结果同样是 6，因此本测试只能验证基本行为；
	// 并发情况下的正确性由数据库端执行的加法语句保证。
	for i := 0; i < 3; i++ {
		if err := pg.AddClicks(ctx, code, 2); err != nil {
			t.Fatalf("第 %d 次累加点击次数失败：%v", i, err)
		}
	}

	link, err := pg.GetLink(ctx, code)
	if err != nil {
		t.Fatalf("查询记录失败：%v", err)
	}
	if link.Clicks != 6 {
		t.Errorf("累加三次、每次加 2 之后 clicks 是 %d，期望 6", link.Clicks)
	}
}

// TestAddClicksNotFound 检查对不存在的短码累加时返回 ErrNotFound。
func TestAddClicksNotFound(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	err := pg.AddClicks(ctx, testCodePrefix+"NoSuchRecord", 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("对不存在的短码累加时返回的错误是 %v，期望它满足 errors.Is(err, ErrNotFound)", err)
	}
}

// TestDeleteLink 检查删除之后记录确实消失，并且再次删除返回 ErrNotFound。
func TestDeleteLink(t *testing.T) {
	pg := newTestStore(t)
	ctx := context.Background()

	const code = testCodePrefix + "Del"
	insertTestLink(t, pg, code, "https://example.com/store/del")

	if err := pg.DeleteLink(ctx, code); err != nil {
		t.Fatalf("删除记录失败：%v", err)
	}

	if _, err := pg.GetLink(ctx, code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除之后查询返回的错误是 %v，期望它满足 errors.Is(err, ErrNotFound)", err)
	}

	if err := pg.DeleteLink(ctx, code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除时返回的错误是 %v，期望它满足 errors.Is(err, ErrNotFound)", err)
	}
}
