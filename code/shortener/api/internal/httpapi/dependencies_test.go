package httpapi

import (
	"context"
	"errors"
	"time"

	"shortener/internal/store"
)

var errMockMethodNotConfigured = errors.New("测试替身方法未配置")

type mockStore struct {
	ping      func(context.Context) error
	create    func(context.Context, store.Link) error
	get       func(context.Context, string) (store.Link, error)
	list      func(context.Context, int, int) ([]store.Link, error)
	count     func(context.Context) (int64, error)
	sumClicks func(context.Context) (int64, error)
	delete    func(context.Context, string) error
}

func (m *mockStore) Ping(ctx context.Context) error {
	if m.ping != nil {
		return m.ping(ctx)
	}
	return errMockMethodNotConfigured
}

func (m *mockStore) CreateLink(ctx context.Context, link store.Link) error {
	if m.create != nil {
		return m.create(ctx, link)
	}
	return errMockMethodNotConfigured
}

func (m *mockStore) GetLink(ctx context.Context, code string) (store.Link, error) {
	if m.get != nil {
		return m.get(ctx, code)
	}
	return store.Link{}, errMockMethodNotConfigured
}

func (m *mockStore) ListLinks(ctx context.Context, limit int, offset int) ([]store.Link, error) {
	if m.list != nil {
		return m.list(ctx, limit, offset)
	}
	return nil, errMockMethodNotConfigured
}

func (m *mockStore) CountLinks(ctx context.Context) (int64, error) {
	if m.count != nil {
		return m.count(ctx)
	}
	return 0, errMockMethodNotConfigured
}

func (m *mockStore) SumClicks(ctx context.Context) (int64, error) {
	if m.sumClicks != nil {
		return m.sumClicks(ctx)
	}
	return 0, errMockMethodNotConfigured
}

func (m *mockStore) DeleteLink(ctx context.Context, code string) error {
	if m.delete != nil {
		return m.delete(ctx, code)
	}
	return errMockMethodNotConfigured
}

type mockCache struct {
	ping       func(context.Context) error
	getURL     func(context.Context, string) (string, error)
	setURL     func(context.Context, string, string, time.Duration) error
	deleteLink func(context.Context, string) error
	incrClick  func(context.Context, string) error
	record     func(context.Context, bool) error
	stats      func(context.Context) (int64, int64, error)
}

func (m *mockCache) Ping(ctx context.Context) error {
	if m.ping != nil {
		return m.ping(ctx)
	}
	return errMockMethodNotConfigured
}

func (m *mockCache) GetURL(ctx context.Context, code string) (string, error) {
	if m.getURL != nil {
		return m.getURL(ctx, code)
	}
	return "", errMockMethodNotConfigured
}

func (m *mockCache) SetURL(ctx context.Context, code string, url string, ttl time.Duration) error {
	if m.setURL != nil {
		return m.setURL(ctx, code, url, ttl)
	}
	return errMockMethodNotConfigured
}

func (m *mockCache) DeleteLink(ctx context.Context, code string) error {
	if m.deleteLink != nil {
		return m.deleteLink(ctx, code)
	}
	return errMockMethodNotConfigured
}

func (m *mockCache) IncrClick(ctx context.Context, code string) error {
	if m.incrClick != nil {
		return m.incrClick(ctx, code)
	}
	return errMockMethodNotConfigured
}

func (m *mockCache) RecordCacheResult(ctx context.Context, hit bool) error {
	if m.record != nil {
		return m.record(ctx, hit)
	}
	return errMockMethodNotConfigured
}

func (m *mockCache) CacheStats(ctx context.Context) (int64, int64, error) {
	if m.stats != nil {
		return m.stats(ctx)
	}
	return 0, 0, errMockMethodNotConfigured
}
