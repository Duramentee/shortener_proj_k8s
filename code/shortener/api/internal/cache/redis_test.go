package cache

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
)

// 本文件是需要真实 Redis 的集成测试。
//
// 运行方式：设置 TEST_REDIS_ADDR 之后再执行 go test，例如
//   TEST_REDIS_ADDR=localhost:6379 go test ./internal/cache/...
// 未设置该环境变量时全部测试会被跳过。
//
// 测试使用的短码统一带 zzcache 前缀，与真实短码（固定六位）不会冲突。
// 每个测试开始时会先删除自己使用的键，结束时也会再清理一次，因此可以重复运行。
//
// 注意：统计命中与未命中的测试会先清空 stats:cache_hits 与 stats:cache_misses 两个键，
// 因此运行该测试之前应当确认后端进程没有在同时处理请求，
// 否则后端写入的统计值会让断言失败。

const testCodePrefix = "zzcache"

// newTestRedis 建立用于测试的客户端。
func newTestRedis(t *testing.T) *Redis {
	t.Helper()

	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("未设置 TEST_REDIS_ADDR，跳过需要真实 Redis 的测试")
	}

	db := 0
	if raw := os.Getenv("TEST_REDIS_DB"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("环境变量 TEST_REDIS_DB 的取值 %q 不是合法的整数：%v", raw, err)
		}
		db = parsed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rdb, err := New(ctx, addr, os.Getenv("TEST_REDIS_PASSWORD"), db)
	if err != nil {
		t.Fatalf("连接 Redis 失败：%v", err)
	}
	t.Cleanup(func() {
		if err := rdb.Close(); err != nil {
			t.Logf("关闭 Redis 客户端时失败：%v", err)
		}
	})

	return rdb
}

// TestSetURLAndGetURL 检查写入之后能够读出，并且键上确实设置了生存时间。
func TestSetURLAndGetURL(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	const code = testCodePrefix + "Get"
	const url = "https://example.com/cache/get"
	const ttl = 60 * time.Second

	t.Cleanup(func() { _ = rdb.client.Del(ctx, LinkKey(code)) })
	_ = rdb.client.Del(ctx, LinkKey(code))

	if err := rdb.SetURL(ctx, code, url, ttl); err != nil {
		t.Fatalf("写入缓存失败：%v", err)
	}

	got, err := rdb.GetURL(ctx, code)
	if err != nil {
		t.Fatalf("读取缓存失败：%v", err)
	}
	if got != url {
		t.Errorf("读取到的长网址是 %q，期望 %q", got, url)
	}

	// 生存时间必须被设置。取值的余地取决于 Redis 的过期检查粒度，
	// 因此这里只断言它落在 0 与设定的生存时间之间。
	actualTTL, err := rdb.client.TTL(ctx, LinkKey(code)).Result()
	if err != nil {
		t.Fatalf("查询键的生存时间失败：%v", err)
	}
	if actualTTL <= 0 || actualTTL > ttl {
		t.Errorf("键的生存时间是 %s，期望它大于 0 且不超过 %s，说明写入时没有正确设置生存时间", actualTTL, ttl)
	}
}

// TestGetURLMiss 检查键不存在时返回 ErrMiss 而不是其他错误。
func TestGetURLMiss(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	_, err := rdb.GetURL(ctx, testCodePrefix+"NoSuchKey")
	if err == nil {
		t.Fatal("读取不存在的键时没有返回错误")
	}
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("读取不存在的键时返回的错误是 %v，期望它满足 errors.Is(err, ErrMiss)", err)
	}
}

// TestDeleteLinkRemovesBothKeys 检查删除操作会把短链接键与点击计数键一起删除。
func TestDeleteLinkRemovesBothKeys(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	const code = testCodePrefix + "Del"

	t.Cleanup(func() { _ = rdb.client.Del(ctx, LinkKey(code), ClicksKey(code)) })
	_ = rdb.client.Del(ctx, LinkKey(code), ClicksKey(code))

	if err := rdb.SetURL(ctx, code, "https://example.com/cache/del", time.Minute); err != nil {
		t.Fatalf("写入缓存失败：%v", err)
	}
	if err := rdb.IncrClick(ctx, code); err != nil {
		t.Fatalf("累加点击次数失败：%v", err)
	}

	if err := rdb.DeleteLink(ctx, code); err != nil {
		t.Fatalf("删除缓存键失败：%v", err)
	}

	remaining, err := rdb.client.Exists(ctx, LinkKey(code), ClicksKey(code)).Result()
	if err != nil {
		t.Fatalf("查询键是否存在时失败：%v", err)
	}
	if remaining != 0 {
		t.Fatalf("删除之后仍然存在 %d 个键，期望两个键都被删除", remaining)
	}
}

// TestIncrClickAndCollectClicks 检查点击增量的累加、读出与扣除三个动作。
func TestIncrClickAndCollectClicks(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	const code = testCodePrefix + "Incr"

	t.Cleanup(func() { _ = rdb.client.Del(ctx, ClicksKey(code)) })
	_ = rdb.client.Del(ctx, ClicksKey(code))

	for i := 0; i < 3; i++ {
		if err := rdb.IncrClick(ctx, code); err != nil {
			t.Fatalf("第 %d 次累加点击次数失败：%v", i, err)
		}
	}

	collected, err := rdb.CollectClicks(ctx)
	if err != nil {
		t.Fatalf("读出点击增量失败：%v", err)
	}
	if collected[code] != 3 {
		t.Fatalf("读出短码 %s 的点击增量是 %d，期望 3；完整结果是 %v", code, collected[code], collected)
	}

	if err := rdb.SubtractClicks(ctx, code, 2); err != nil {
		t.Fatalf("扣除点击增量失败：%v", err)
	}

	afterSubtract, err := rdb.CollectClicks(ctx)
	if err != nil {
		t.Fatalf("扣除之后再次读出点击增量失败：%v", err)
	}
	if afterSubtract[code] != 1 {
		t.Fatalf("扣除 2 之后短码 %s 的点击增量是 %d，期望 1", code, afterSubtract[code])
	}
}

// TestCollectClicksSkipsMissingKeys 检查没有点击增量时返回的是空映射并且不返回错误。
func TestCollectClicksSkipsMissingKeys(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	collected, err := rdb.CollectClicks(ctx)
	if err != nil {
		t.Fatalf("读出点击增量失败：%v", err)
	}
	if collected == nil {
		t.Fatal("没有点击增量时返回的是 nil 映射，应当返回长度为 0 的映射，否则调用方需要额外判断 nil")
	}
}

// TestRecordCacheResultAndCacheStats 检查命中与未命中的统计以及命中率的计算依据。
func TestRecordCacheResultAndCacheStats(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	t.Cleanup(func() { _ = rdb.client.Del(ctx, KeyCacheHits, KeyCacheMisses) })
	if err := rdb.client.Del(ctx, KeyCacheHits, KeyCacheMisses).Err(); err != nil {
		t.Fatalf("清空统计键失败：%v", err)
	}

	for i := 0; i < 3; i++ {
		if err := rdb.RecordCacheResult(ctx, true); err != nil {
			t.Fatalf("第 %d 次记录缓存命中失败：%v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := rdb.RecordCacheResult(ctx, false); err != nil {
			t.Fatalf("第 %d 次记录缓存未命中失败：%v", i, err)
		}
	}

	hits, misses, err := rdb.CacheStats(ctx)
	if err != nil {
		t.Fatalf("读取缓存统计失败：%v", err)
	}
	if hits != 3 {
		t.Errorf("缓存命中次数是 %d，期望 3", hits)
	}
	if misses != 2 {
		t.Errorf("缓存未命中次数是 %d，期望 2", misses)
	}
}

// TestCacheStatsOnMissingKeys 检查统计键不存在时返回 0 而不是错误。
func TestCacheStatsOnMissingKeys(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	t.Cleanup(func() { _ = rdb.client.Del(ctx, KeyCacheHits, KeyCacheMisses) })
	if err := rdb.client.Del(ctx, KeyCacheHits, KeyCacheMisses).Err(); err != nil {
		t.Fatalf("清空统计键失败：%v", err)
	}

	hits, misses, err := rdb.CacheStats(ctx)
	if err != nil {
		t.Fatalf("统计键不存在时读取缓存统计返回错误：%v", err)
	}
	if hits != 0 {
		t.Errorf("统计键不存在时缓存命中次数是 %d，期望 0", hits)
	}
	if misses != 0 {
		t.Errorf("统计键不存在时缓存未命中次数是 %d，期望 0", misses)
	}
}
