package flusher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"shortener/internal/store"
)

type mockClickCache struct {
	collected  map[string]int64
	collectErr error
	subtract   func(context.Context, string, int64) error
}

func (m *mockClickCache) CollectClicks(context.Context) (map[string]int64, error) {
	return m.collected, m.collectErr
}

func (m *mockClickCache) SubtractClicks(ctx context.Context, code string, delta int64) error {
	if m.subtract != nil {
		return m.subtract(ctx, code, delta)
	}
	return nil
}

type mockClickStore struct {
	add func(context.Context, string, int64) error
}

func (m *mockClickStore) AddClicks(ctx context.Context, code string, delta int64) error {
	if m.add != nil {
		return m.add(ctx, code, delta)
	}
	return nil
}

func newTestFlusher(rdb ClickCache, pg ClickStore) *Flusher {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(rdb, pg, time.Second, logger)
}

func TestFlushOnceWritesBeforeSubtracting(t *testing.T) {
	var calls []string
	rdb := &mockClickCache{
		collected: map[string]int64{"abc123": 4},
		subtract: func(_ context.Context, code string, delta int64) error {
			calls = append(calls, "subtract")
			if code != "abc123" || delta != 4 {
				t.Fatalf("扣除参数为 %q, %d，期望 abc123, 4", code, delta)
			}
			return nil
		},
	}
	pg := &mockClickStore{
		add: func(_ context.Context, code string, delta int64) error {
			calls = append(calls, "add")
			if code != "abc123" || delta != 4 {
				t.Fatalf("写入参数为 %q, %d，期望 abc123, 4", code, delta)
			}
			return nil
		},
	}

	if err := newTestFlusher(rdb, pg).FlushOnce(context.Background()); err != nil {
		t.Fatalf("FlushOnce 返回错误：%v", err)
	}
	if want := []string{"add", "subtract"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("调用顺序为 %v，期望 %v", calls, want)
	}
}

func TestFlushOnceDoesNotSubtractAfterStoreFailure(t *testing.T) {
	storeErr := errors.New("database unavailable")
	subtracted := false
	rdb := &mockClickCache{
		collected: map[string]int64{"abc123": 4},
		subtract: func(context.Context, string, int64) error {
			subtracted = true
			return nil
		},
	}
	pg := &mockClickStore{
		add: func(context.Context, string, int64) error {
			return storeErr
		},
	}

	err := newTestFlusher(rdb, pg).FlushOnce(context.Background())
	if !errors.Is(err, storeErr) {
		t.Fatalf("FlushOnce 错误为 %v，期望保留存储错误 %v", err, storeErr)
	}
	if subtracted {
		t.Fatal("存储写入失败后仍扣除了缓存增量，可能导致点击数据丢失")
	}
}

func TestFlushOnceSkipsDeletedLinks(t *testing.T) {
	subtracted := false
	rdb := &mockClickCache{
		collected: map[string]int64{"abc123": 4},
		subtract: func(context.Context, string, int64) error {
			subtracted = true
			return nil
		},
	}
	pg := &mockClickStore{
		add: func(context.Context, string, int64) error {
			return store.ErrNotFound
		},
	}

	if err := newTestFlusher(rdb, pg).FlushOnce(context.Background()); err != nil {
		t.Fatalf("短链接已删除时应跳过且不返回错误，实际为：%v", err)
	}
	if subtracted {
		t.Fatal("短链接已删除时不应扣除缓存增量")
	}
}
