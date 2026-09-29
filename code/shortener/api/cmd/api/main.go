// Package main 是后端的入口程序。
//
// 它只负责装配：读取配置、建立两个依赖的客户端、启动后台协程、启动 HTTP 服务、处理停止信号。
// 业务逻辑全部位于 internal 下面的各个包里，因此本文件不包含任何与短链接相关的处理逻辑。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shortener/internal/cache"
	"shortener/internal/config"
	"shortener/internal/flusher"
	"shortener/internal/httpapi"
	"shortener/internal/store"
)

// startupTimeout 是连接两个依赖并且完成建表的总时限。
// 单独设置这个时限的目的是：依赖不可达时能够在一个确定的时间内启动失败并给出明确的原因，
// 而不是让进程一直停留在启动状态。这与就绪探针的意图一致，只是发生的时机不同。
const startupTimeout = 15 * time.Second

func main() {
	// 使用标准库的 log/slog 输出结构化日志。这里使用文本格式是为了在阶段 2 与阶段 3 中便于阅读；
	// 进入 Kubernetes 阶段之后可以换成 slog.NewJSONHandler，便于日志系统按字段检索。
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil {
		logger.Error("服务退出", "原因", err.Error())
		os.Exit(1)
	}
}

// run 完成全部装配工作，并且在收到停止信号之后执行优雅退出。
//
// 本函数已经写好，你不需要修改它。
//
// 返回错误表示启动阶段失败或者优雅退出阶段失败；返回 nil 表示正常停止。
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("读取配置失败：%w", err)
	}
	logger.Info("配置读取完成", "摘要", cfg.Summary())

	// 收到 SIGINT（在终端按 Ctrl+C）或者 SIGTERM（容器被停止时由运行时发送）时，
	// 这个 context 会被取消。下面创建的上下文全部由它派生，
	// 因此一个信号就能让建连、后台协程与 HTTP 服务同时开始收尾。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupCtx, cancelStartup := context.WithTimeout(ctx, startupTimeout)
	defer cancelStartup()

	pg, err := store.NewPostgres(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pg.Close()
	logger.Info("已连接 PostgreSQL", "主机", pg.Host(), "数据库", pg.Name())

	if err := pg.EnsureSchema(startupCtx); err != nil {
		return err
	}
	logger.Info("数据表已就绪")

	rdb, err := cache.New(startupCtx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return err
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Warn("关闭 Redis 客户端时出错", "错误", err.Error())
		}
	}()
	logger.Info("已连接 Redis", "地址", rdb.Addr())

	fl := flusher.New(rdb, pg, cfg.ClickFlushInterval, logger)
	go fl.Run(ctx)

	handler := httpapi.New(cfg, pg, rdb, logger).Routes()
	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
		// 下面四个超时的作用是防止慢连接长期占用协程。
		// 第 1 周配置 nginx 时提到的超时对应的是代理这一层，
		// 这里配置的超时对应的是应用这一层，两层都需要配置，只配置一层会留下缺口。
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 在独立协程中启动监听。返回的错误通过通道传回主协程，
	// 这样主协程可以用 select 同时等待「监听出错」与「收到停止信号」两种情况。
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP 服务开始监听", "地址", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("HTTP 服务监听失败：%w", err)
	case <-ctx.Done():
		logger.Info("收到停止信号，开始优雅退出")
	}

	// 收尾使用的 context 必须从 context.Background() 派生，不能从已经取消的 ctx 派生，
	// 否则它会立刻处于已取消状态，srv.Shutdown 会直接返回错误而不等待任何请求处理完成。
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("HTTP 服务优雅退出失败：%w", err)
	}

	// 退出之前做最后一次点击增量写回，尽量不丢失缓冲区中已经累积的计数。
	if err := fl.FlushOnce(shutdownCtx); err != nil {
		logger.Warn("退出前的点击写回失败，本轮增量会留在 Redis 中等待下次启动写回", "错误", err.Error())
	}

	logger.Info("服务已停止")
	return nil
}
