// Package cache 负责与 Redis 交互。
//
// Redis 在本项目里承担两个互不相干的职责：
//  1. 短链接的读缓存，对应键 link:{code}，目的是让重定向请求不必每次都访问 PostgreSQL；
//  2. 点击次数的写入缓冲区，对应键 clicks:{code}，目的是把高频的自增操作合并成低频的数据库更新。
//
// 这两个职责共用同一个 Redis 客户端，但使用不同的键前缀，因此互不影响。
package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrMiss 表示缓存中没有对应的键，也就是缓存未命中。
//
// 与存储层的 ErrNotFound 一样，引入独立错误的原因是调用方需要区分两种情况：
// 「键不存在」时应当回落到数据库查询，而「Redis 连接出错」时应当把请求判定为失败。
var ErrMiss = errors.New("缓存未命中")

// 键名前缀与统计键。集中定义在常量里，避免在多个文件中出现拼写不一致的字符串。
const (
	// linkKeyPrefix 是短链接缓存键的前缀，完整键名形如 link:a1B2c3。
	linkKeyPrefix = "link:"
	// clicksKeyPrefix 是点击增量键的前缀，完整键名形如 clicks:a1B2c3。
	clicksKeyPrefix = "clicks:"
	// KeyCacheHits 是缓存命中次数的统计键。
	KeyCacheHits = "stats:cache_hits"
	// KeyCacheMisses 是缓存未命中次数的统计键。
	KeyCacheMisses = "stats:cache_misses"
	// scanBatchSize 是扫描键空间时每一批返回的键数量，仅作为给 Redis 的提示值。
	scanBatchSize = 100
)

// LinkKey 返回短链接缓存键。
func LinkKey(code string) string {
	return linkKeyPrefix + code
}

// ClicksKey 返回点击增量键。
func ClicksKey(code string) string {
	return clicksKeyPrefix + code
}

// Redis 封装 Redis 客户端。
type Redis struct {
	client *redis.Client
}

// New 创建 Redis 客户端并且确认 Redis 可达。
//
// 本函数已经写好，你不需要修改它。与 NewPostgres 相同，这里也执行了一次 Ping，
// 目的是让「Redis 地址填写错误」在启动阶段就暴露出来，而不是等到第一个请求到达时才暴露。
func New(ctx context.Context, addr string, password string, db int) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,

		// 下面四项把客户端的超时边界固定下来，目的是让就绪探针在确定的时间之内得到结论。
		//
		// ContextTimeoutEnabled 的默认取值是 false，此时客户端会把调用方传入的 context
		// 替换成 context.Background()，调用方设置的时限被完全丢弃；
		// 建立连接改为使用 DialTimeout，而 DialTimeout 的默认取值是 5 秒。
		// 实测现象：Redis 容器正在停止的过程中，Docker 的端口代理仍然会接受 TCP 连接，
		// 此时就绪检查要等待 5 秒才有结论，超过 TASKS.md 第 7 组验收要求的 3 秒。
		// 把这一项设置为 true 之后，调用方的 context 时限会一路传到连接建立与读写，
		// 就绪检查使用的 2 秒时限因此能够真正生效。
		ContextTimeoutEnabled: true,
		DialTimeout:           2 * time.Second,
		ReadTimeout:           2 * time.Second,
		WriteTimeout:          2 * time.Second,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("连接 Redis 失败，请检查 Redis 是否已经启动以及 REDIS_ADDR 的取值：%w", err)
	}

	return &Redis{client: client}, nil
}

// Close 关闭客户端。进程退出之前应当调用它。
func (r *Redis) Close() error {
	return r.client.Close()
}

// Addr 返回客户端实际使用的地址，用于启动日志输出。
func (r *Redis) Addr() string {
	return r.client.Options().Addr
}

// Ping 检查 Redis 是否可达，供就绪探针使用。
func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// GetURL 查询短链接缓存。
//
// 实现要求：
//  1. 键名使用 LinkKey(code)。
//  2. 键不存在时必须返回 ErrMiss，判断方式是 errors.Is(err, redis.Nil)。
//     redis.Nil 是客户端库用来表示「键不存在」的哨兵错误，它不是一个真正的故障。
//  3. 其余错误包装之后向上返回。调用方需要区分「未命中」与「Redis 不可用」这两种情况，
//     前者要回落到数据库查询，后者要把请求判定为失败。
//
// 提示：使用 r.client.Get，并且读取返回的 *redis.StringCmd 的 Result 方法。
func (r *Redis) GetURL(ctx context.Context, code string) (string, error) {
	url, err := r.client.Get(ctx, LinkKey(code)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrMiss
		}

		return "", fmt.Errorf("读取短码 %s 的缓存失败：%w", code, err)
	}

	return url, nil
}

// SetURL 写入短链接缓存，并设置生存时间。
//
// 实现要求：
//  1. 键名使用 LinkKey(code)，生存时间使用参数 ttl。
//  2. 生存时间是必须设置的。不设置生存时间的键会永久占用内存，
//     而缓存中的数据是可以随时从数据库重建的，让它可以自动过期可以避免容量无限增长。
//
// 提示：使用 r.client.Set，它的第三个参数就是生存时间。
func (r *Redis) SetURL(ctx context.Context, code string, url string, ttl time.Duration) error {
	if err := r.client.Set(ctx, LinkKey(code), url, ttl).Err(); err != nil {
		return fmt.Errorf("写入短码 %s 的缓存失败：%w", code, err)
	}

	return nil
}

// DeleteLink 删除某个短码对应的两个键：短链接缓存键与点击增量键。
//
// 实现要求：
//  1. 一次调用删除两个键，而不是分两次调用。减少往返次数可以缩短删除操作的耗时。
//  2. 删除不存在的键不算错误，Redis 的删除命令会返回被删除的键数量，取值可以是 0。
//
// 提示：使用 r.client.Del，它接受可变数量的键名参数。
func (r *Redis) DeleteLink(ctx context.Context, code string) error {
	if err := r.client.Del(ctx, LinkKey(code), ClicksKey(code)).Err(); err != nil {
		return fmt.Errorf("删除短码 %s 的缓存键与增量键失败：%w", code, err)
	}

	return nil
}

// IncrClick 把某个短码的点击增量加一。
//
// 实现要求：使用 Redis 的原子自增命令，不要先读取当前取值、在 Go 里加一、再写回去。
// 后一种写法存在时间窗口：两个请求同时执行时，两次自增可能只留下一次的结果。
//
// 提示：使用 r.client.Incr。键不存在时该命令会把键的值视为 0 再加一，因此不需要先初始化。
func (r *Redis) IncrClick(ctx context.Context, code string) error {
	if err := r.client.Incr(ctx, ClicksKey(code)).Err(); err != nil {
		return fmt.Errorf("累加短码 %s 的增量键失败：%w", code, err)
	}

	return nil
}

// RecordCacheResult 记录一次缓存查询的结果，供统计接口计算命中率。
//
// 实现要求：命中时让 KeyCacheHits 自增，未命中时让 KeyCacheMisses 自增。
// 两个统计键都不设置生存时间，因此在 Redis 重启（且没有开启持久化）之后会被清零，
// 此时统计接口返回的命中率会从零开始重新累计，这一点需要写进你的笔记。
//
// 提示：使用 r.client.Incr，命中时第一个参数取 KeyCacheHits，未命中时取 KeyCacheMisses。
func (r *Redis) RecordCacheResult(ctx context.Context, hit bool) error {
	key := KeyCacheMisses
	if hit {
		key = KeyCacheHits
	}

	if err := r.client.Incr(ctx, key).Err(); err != nil {
		return fmt.Errorf("累加统计键 %s 失败：%w", key, err)
	}

	return nil
}

// CacheStats 返回缓存命中与未命中的累计次数。
//
// 实现要求：
//  1. 一次调用读取两个键，而不是分两次调用。
//  2. 两个统计键在 Redis 中不存在时，返回值必须是 0 而不是错误。
//     键不存在说明还没有发生过任何一次缓存查询，此时命中次数与未命中次数都应当视为 0。
//  3. 取值可能不是合法的整数（例如键被其他程序占用），此时应当返回错误而不是静默地当作 0。
//
// 提示：使用 r.client.MGet 一次性读取两个键。它返回的切片中，
// 不存在的键对应的元素是 nil，需要先判断类型再做整数转换；
// 转换可以使用类型断言得到 string，再使用 strconv.ParseInt。
func (r *Redis) CacheStats(ctx context.Context) (hits int64, misses int64, err error) {
	values, err := r.client.MGet(ctx, KeyCacheHits, KeyCacheMisses).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("读取统计键 %s 与 %s 失败：%w", KeyCacheHits, KeyCacheMisses, err)
	}

	hits, err = counterValue(values[0], KeyCacheHits)
	if err != nil {
		return 0, 0, err
	}

	misses, err = counterValue(values[1], KeyCacheMisses)
	if err != nil {
		return 0, 0, err
	}

	return hits, misses, nil
}

// counterValue 把一个 MGET 返回的元素转换成 64 位整数。
//
// 参数 value 的静态类型是 any，因为 MGET 的返回值中每个位置既可能是字符串，也可能是 nil。
// value 是 nil 表示对应的键在 Redis 中不存在，此时按 0 处理；
// value 不是字符串，或者转换成整数失败时返回错误，避免把异常的取值当作 0 处理。
func counterValue(value any, key string) (int64, error) {
	if value == nil {
		return 0, nil
	}

	text, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("键 %s 的取值类型是 %T，不是字符串", key, value)
	}

	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("键 %s 的取值 %q 不是合法的整数：%w", key, text, err)
	}

	return parsed, nil
}

// CollectClicks 读出全部尚未写回数据库的点击增量，返回「短码到增量」的映射。
// 本函数由后台写回协程调用，请求处理路径不会调用它。
//
// 实现要求：
//  1. 不要使用 KEYS 命令枚举键。KEYS 会一次性遍历整个键空间并且阻塞其他命令的执行，
//     键的数量较多时会让 Redis 停止响应，因此生产环境一律使用 SCAN 命令。
//  2. SCAN 采用游标迭代：第一次调用时游标取 0，每次调用返回一批键与下一个游标，
//     当返回的游标重新变成 0 时迭代结束。取值为 0 的游标同时也被后续的键扫描用于判断，
//     因此必须把「迭代结束」与「这一批没有命中任何键」这两件事分开处理。
//  3. 扫描时使用匹配模式 clicks:* 与提示批次 scanBatchSize。
//  4. 取到键之后逐个读取取值并且转换成整数。取值为 0 的键可以跳过，不需要写回。
//  5. 键名中的短码部分需要从完整的键名中去掉前缀才能得到。
//
// 提示：使用 r.client.Scan 取得 *redis.ScanCmd，再对返回的键列表调用
// r.client.MGet 批量读取取值，或者逐个使用 r.client.Get。
// 转换字符串到整数使用 strconv.ParseInt，第二个参数填写 10，第三个参数填写 64。
func (r *Redis) CollectClicks(ctx context.Context) (map[string]int64, error) {
	collected := make(map[string]int64)

	cursor := uint64(0)
	for {
		keys, next, err := r.client.Scan(ctx, cursor, clicksKeyPrefix+"*", scanBatchSize).Result()
		if err != nil {
			return nil, fmt.Errorf("扫描点击增量键失败：%w", err)
		}

		if len(keys) > 0 {
			values, err := r.client.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, fmt.Errorf("读取点击增量键的取值失败：%w", err)
			}

			for index, key := range keys {
				delta, err := counterValue(values[index], key)
				if err != nil {
					return nil, err
				}

				if delta == 0 {
					continue
				}

				collected[strings.TrimPrefix(key, clicksKeyPrefix)] = delta
			}
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	return collected, nil
}

// SubtractClicks 从某个短码的点击增量中扣除已经写回数据库的部分。
// 本函数由后台写回协程在数据库更新成功之后调用。
//
// 实现要求：
//  1. 使用减法命令而不是直接删除键，理由是：扣除之前键里可能已经累积了新的增量，
//     这些新增量来自本函数执行期间到达的请求，直接删除会把它们一起丢掉。
//  2. 减法执行之后键的取值可能变成 0，甚至可能变成负数。
//     取值变成 0 属于正常情况，不需要处理；
//     负数说明扣除的增量超过了键里实际存在的增量，这只可能出现在实现有误的情况下，
//     因此不需要为它编写特殊处理，但应当把它写进你的笔记作为边界条件。
//
// 提示：使用 r.client.DecrBy，第二个参数是要扣除的数量。
func (r *Redis) SubtractClicks(ctx context.Context, code string, delta int64) error {
	if err := r.client.DecrBy(ctx, ClicksKey(code), delta).Err(); err != nil {
		return fmt.Errorf("扣除短码 %s 的点击增量失败：%w", code, err)
	}

	return nil
}
