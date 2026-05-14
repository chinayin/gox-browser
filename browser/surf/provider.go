package surf

import (
	"context"
	"log/slog"
	"time"

	"github.com/enetx/g"
	"github.com/enetx/surf"

	browser "github.com/chinayin/gox-browser/browser"
)

const defaultTimeout = 30 * time.Second

// Config Surf HTTP 伪装 Provider 配置
type Config struct {
	Timeout time.Duration
	Proxy   string
}

// Provider Surf TLS 伪装 HTTP 客户端工厂
type Provider struct {
	cfg Config
}

// 编译期接口合规断言
var _ browser.Provider = (*Provider)(nil)

func NewProvider(cfg Config) *Provider {
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	return &Provider{cfg: cfg}
}

func (p *Provider) Type() browser.Type { return browser.TypeSurf }

func (p *Provider) Create(_ context.Context, opts browser.AcquireOpts) (browser.Browser, error) {
	builder := surf.NewClient().
		Builder().
		Impersonate().Chrome().
		Session().
		Timeout(p.cfg.Timeout)

	proxy := opts.Proxy
	if proxy == "" {
		proxy = p.cfg.Proxy
	}
	if proxy != "" {
		builder = builder.Proxy(g.String(proxy))
	}

	client := builder.Build().Unwrap()

	slog.Debug("browser: surf instance created")
	return &surfBrowser{client: client}, nil
}

func (p *Provider) HealthCheck(_ context.Context) error { return nil }

func (p *Provider) Close() error { return nil }
