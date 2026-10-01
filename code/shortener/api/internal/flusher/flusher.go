// Package flusher 负责把 Redis 中累积的点击增量周期性地写回 PostgreSQL。
//
// 这个后台协程是本项目「写合并」这一设计的实现部分：
// 请求处理路径只对 Redis 做一次自增，数据库承担的是每一轮周期的固定次数写入，
// 因此重定向的响应时间只取决于 Redis，热点短链也不会在数据库上产生写竞争。
package flusher

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"shortener/internal/cache"
	"shortener/internal/store"
)

// Flusher 持有写回所需的依赖。
type Flusher struct {
	cache    *cache.Redis
	store    *store.Postgres
	interval time.Duration
	logger   *slog.Logger
}

// New 构造 Flusher。
func New(rdb *cache.Redis, pg *store.Postgres, interval time.Duration, logger *slog.Logger) *Flusher {
	return &Flusher{
		cache:    rdb,
		store:    pg,
		interval: interval,
		logger:   logger,
	}
}

// Run 启动周期性的写回循环，直到 ctx 被取消为止。
// 本函数由 main 以一个独立的协程启动。
//
// 本函数已经实现。
//
// 实现要求：
//  1. 创建一个按 f.interval 触发的定时器（time.NewTicker），在循环里用 select
//     同时等待定时器触发与 ctx.Done()。
//  2. 每一次触发时调用 FlushOnce。返回错误时记录一条警告日志并且继续循环，
//     不要因为一轮失败就退出循环：PostgreSQL 短暂不可用是很常见的情况，
//     退出循环会让点击计数从此永久停止写回，而继续循环可以在依赖恢复之后自动接上。
//  3. ctx 被取消时必须停止循环。使用 defer 停止定时器，
//     否则定时器会继续占用运行时资源（定时器是运行时对象，不停止会让它无法被回收）。
//  4. 退出循环之前不要做写回，收尾的写回由 main 调用 FlushOnce 完成，
//     这样「协程退出」与「收尾写回」两件事各自只有一处实现。
//
// 需要你添加的导入：无（context、time、slog 已经导入）。
//
// 验收方式：见 TASKS.md 第 8 组的验收表。
func (f *Flusher) Run(ctx context.Context) {
	ticker := time.NewTicker(f.interval)

	// 不停止定时器时，运行时对象不会被回收，因此这里必须用 defer 停止它。
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 退出之前不做写回：收尾的写回由 main 调用 FlushOnce 完成，
			// 这样「协程退出」与「收尾写回」两件事各自只有一处实现。
			f.logger.Info("后台点击写回协程收到停止信号，准备退出")
			return
		case <-ticker.C:
			if err := f.FlushOnce(ctx); err != nil {
				// 一轮失败之后继续循环：PostgreSQL 短暂不可用是很常见的情况，
				// 退出循环会让点击计数从此永久停止写回，继续循环可以在依赖恢复之后自动接上。
				f.logger.Warn("本轮点击写回失败，等待下一个周期重试", "错误", err.Error())
			}
		}
	}
}

// FlushOnce 执行一轮写回，包含三个步骤：
//  1. 调用 f.cache.CollectClicks 读出全部尚未写回数据库的增量，得到「短码到增量」的映射；
//  2. 逐个调用 f.store.AddClicks 把增量累加到数据库；
//  3. 数据库更新成功之后调用 f.cache.SubtractClicks 扣除已经写回的增量。
//
// 本函数已经实现。
//
// 实现要求：
//  1. 映射为空时直接返回，不需要记录日志，否则日志会被空轮次填满。
//  2. 某一个短码写回失败时记录警告日志并且跳过它，不要因为一条记录失败就中断整轮写回。
//  3. 数据库返回 store.ErrNotFound 时说明这条短链接已经被删除，
//     此时缓存中的计数键是残留数据，应当跳过并且记录一条日志说明这一情况，
//     不需要调用 SubtractClicks（键会随缓存过期或者下次删除操作被清理）。
//  4. SubtractClicks 必须在 AddClicks 成功之后调用。顺序颠倒会让「数据库更新失败」
//     这种情况下已经扣除的增量永久丢失。这个顺序就是 README 第 5 节所说的边界条件：
//     后端在扣除增量之前被强制杀死时，这一轮的增量会在下一轮被重复写回或者丢失，
//     具体表现为哪种结果取决于杀死发生的时刻，这一点需要写进你的笔记。
//  5. 一轮结束时返回第一个遇到的错误，同时把这个错误用于日志输出；
//     全部成功时返回 nil。
//
// 需要你添加的导入：errors、shortener/internal/cache 已经导入。
//
// 验收方式：见 TASKS.md 第 8 组的验收表。
func (f *Flusher) FlushOnce(ctx context.Context) error {
	collected, err := f.cache.CollectClicks(ctx)
	if err != nil {
		return err
	}

	// 映射为空时直接返回，不记录日志，否则日志会被空轮次填满。
	if len(collected) == 0 {
		return nil
	}

	// firstErr 保存本轮遇到的第一个错误，并且返回它用于日志输出。
	// 遇到错误之后不立即返回，是为了让其余短码的增量在同一轮里也被写回。
	var firstErr error

	for code, delta := range collected {
		if err := f.store.AddClicks(ctx, code, delta); err != nil {
			// 记录不存在说明这条短链接已经被删除，缓存中的增量键是残留数据，
			// 因此跳过即可，不需要扣除增量：键会随缓存过期或者下次删除操作被清理。
			if errors.Is(err, store.ErrNotFound) {
				f.logger.Warn("短链接已经不存在，跳过本轮写回", "短码", code, "增量", delta)
				continue
			}

			f.logger.Warn("写回点击增量失败，跳过该短码", "短码", code, "增量", delta, "错误", err.Error())
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// 顺序不能颠倒：必须先写数据库，再扣除缓存中的增量。
		// 颠倒之后，数据库更新失败会让已经扣除的增量永久丢失；
		// 而当前顺序下出现的偏差只会是「重复写回」，不会丢计数。
		if err := f.cache.SubtractClicks(ctx, code, delta); err != nil {
			f.logger.Warn("扣除已经写回的点击增量失败", "短码", code, "增量", delta, "错误", err.Error())
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}
