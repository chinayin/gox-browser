package rod

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	browser "github.com/chinayin/gox-browser/browser"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

const resolveRemoteTimeout = 10 * time.Second

// Config Rod 浏览器 Provider 配置
type Config struct {
	RemoteURL             string   `json:"remote_url" yaml:"remote_url"`
	RemoteToken           string   `json:"remote_token" yaml:"remote_token"`
	Headless              bool     `json:"headless" yaml:"headless"`
	StealthMode           bool     `json:"stealth_mode" yaml:"stealth_mode"`
	RandomFingerprint     bool     `json:"random_fingerprint" yaml:"random_fingerprint"`
	Proxy                 string   `json:"proxy" yaml:"proxy"`
	BlockResources        []string `json:"block_resources" yaml:"block_resources"`
	BlockThirdPartyScript bool     `json:"block_third_party_script" yaml:"block_third_party_script"`
	AllowedScriptDomains  []string `json:"allowed_script_domains" yaml:"allowed_script_domains"`
	BinPath               string   `json:"bin_path" yaml:"bin_path"`
}

// 编译期接口合规断言
var _ browser.Provider = (*Provider)(nil)

// Provider Rod 浏览器实例工厂
type Provider struct {
	cfg     Config
	browser *rod.Browser
	mu      sync.Mutex
}

func NewProvider(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Type() browser.Type {
	if p.cfg.Headless {
		return browser.TypeRodHeadless
	}
	return browser.TypeRodHeaded
}

func (p *Provider) Create(ctx context.Context, opts browser.AcquireOpts) (b browser.Browser, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("browser: rod create recovered from panic", "panic", r)
			p.mu.Lock()
			p.browser = nil
			p.mu.Unlock()
			b = nil
			err = fmt.Errorf("browser: rod create panic: %v", r)
		}
	}()

	rb, connErr := p.ensureBrowser(ctx)
	if connErr != nil {
		return nil, fmt.Errorf("browser: rod ensure browser: %w", connErr)
	}

	var page *rod.Page
	var pageErr error
	if p.cfg.StealthMode {
		page, pageErr = stealth.Page(rb)
		if pageErr != nil {
			return nil, fmt.Errorf("browser: rod stealth create page: %w", pageErr)
		}
	} else {
		page, pageErr = rb.Page(proto.TargetCreateTarget{URL: "about:blank"})
		if pageErr != nil {
			return nil, fmt.Errorf("browser: rod create page: %w", pageErr)
		}
	}

	// NOTE: 不再调用 applyEvasion — Rod 后端完全依赖 go-rod/stealth 包的原生 evasion。
	// applyEvasion 会与 stealth.js 产生冲突 (重复覆盖 chrome.runtime 等)，
	// 反而增加被 Cloudflare 等 WAF 检测到的风险。
	// if p.cfg.StealthMode {
	// 	if err := applyEvasion(page, opts.UserAgent); err != nil {
	// 		slog.Warn("browser: rod evasion injection failed", "error", err)
	// 	}
	// }

	vp := opts.Viewport
	if vp.Width == 0 {
		vp = browser.DefaultViewport
	}

	if p.cfg.RandomFingerprint {
		if vp == browser.DefaultViewport {
			vp = randomViewport()
		}

		tz := randomTimezone()
		if err := (proto.EmulationSetTimezoneOverride{TimezoneID: tz}).Call(page); err != nil {
			slog.Warn("browser: rod set timezone override failed", "timezone", tz, "error", err)
		}

		locale := randomLocale()
		if err := (proto.EmulationSetLocaleOverride{Locale: locale}).Call(page); err != nil {
			slog.Warn("browser: rod set locale override failed", "locale", locale, "error", err)
		}

		slog.Debug("browser: rod fingerprint randomized",
			"viewport", fmt.Sprintf("%dx%d", vp.Width, vp.Height),
			"timezone", tz,
			"locale", locale,
		)
	}

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             vp.Width,
		Height:            vp.Height,
		DeviceScaleFactor: vp.Scale,
		Mobile:            vp.Mobile,
	}); err != nil {
		page.Close()
		return nil, fmt.Errorf("browser: rod set viewport: %w", err)
	}

	var router *rod.HijackRouter
	if p.hijackEnabled() {
		router, err = p.setupRequestInterception(page)
		if err != nil {
			page.Close()
			return nil, err
		}
	}

	return &rodBrowser{
		page:     page,
		router:   router,
		headless: p.cfg.Headless,
	}, nil
}

// hijackEnabled 是否启用了请求拦截
func (p *Provider) hijackEnabled() bool {
	return len(p.cfg.BlockResources) > 0 || p.cfg.BlockThirdPartyScript
}

// setupRequestInterception 启用请求拦截，返回的 router 由调用方负责在关闭时 Stop。
func (p *Provider) setupRequestInterception(page *rod.Page) (*rod.HijackRouter, error) {
	blocked := make(map[proto.NetworkResourceType]bool, len(p.cfg.BlockResources))
	for _, rt := range p.cfg.BlockResources {
		blocked[proto.NetworkResourceType(rt)] = true
	}

	router := page.HijackRequests()
	if err := router.Add("*", "", func(ctx *rod.Hijack) {
		rt := ctx.Request.Type()

		if blocked[rt] {
			ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}

		if p.cfg.BlockThirdPartyScript && rt == proto.NetworkResourceTypeScript {
			if !isAllowedScriptDomain(ctx.Request.URL().Host, p.cfg.AllowedScriptDomains) {
				ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
				return
			}
		}

		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	}); err != nil {
		return nil, fmt.Errorf("browser: rod setup request interception: %w", err)
	}
	go router.Run()

	slog.Debug("browser: rod request interception enabled",
		"blocked_types", p.cfg.BlockResources,
		"block_3p_script", p.cfg.BlockThirdPartyScript,
	)
	return router, nil
}

func (p *Provider) ensureBrowser(ctx context.Context) (*rod.Browser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.browser != nil {
		if _, err := p.browser.Pages(); err != nil {
			slog.Warn("browser: rod stale connection detected, reconnecting", "error", err)
			p.browser = nil
		} else {
			return p.browser, nil
		}
	}

	var controlURL string

	if p.cfg.RemoteURL != "" { //nolint:nestif // 本地/远程两种模式的分支逻辑
		var err error
		controlURL, err = p.resolveRemoteURL()
		if err != nil {
			return nil, err
		}
		slog.Info("browser: rod connecting to remote chrome", "url", controlURL)
	} else {
		binPath := p.cfg.BinPath
		if binPath == "" {
			var found bool
			binPath, found = launcher.LookPath()
			if !found {
				return nil, fmt.Errorf("browser: rod chrome binary not found")
			}
		}

		l := launcher.New().Bin(binPath).
			Set("disable-blink-features", "AutomationControlled").
			Headless(p.cfg.Headless)

		if p.cfg.Proxy != "" {
			l = l.Set("proxy-server", p.cfg.Proxy)
		}

		var err error
		controlURL, err = l.Context(ctx).Launch()
		if err != nil {
			return nil, fmt.Errorf("browser: rod launch chrome: %w", err)
		}

		slog.Info("browser: rod launched local chrome", "headless", p.cfg.Headless)
	}

	var b *rod.Browser
	if p.cfg.RemoteURL != "" {
		// 远程连接时使用标准 Sec-WebSocket-Key，兼容 aiohttp 等严格校验的 WebSocket 服务端
		wsHeader := http.Header{
			"Sec-WebSocket-Key": {generateWebSocketKey()},
		}
		client, err := cdp.StartWithURL(ctx, controlURL, wsHeader)
		if err != nil {
			return nil, fmt.Errorf("browser: rod connect: %w", err)
		}
		b = rod.New().Client(client)
	} else {
		b = rod.New().ControlURL(controlURL)
	}
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("browser: rod connect: %w", err)
	}

	if _, err := b.Pages(); err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("browser: rod post-connect health check: %w", err)
	}

	p.browser = b
	return b, nil
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	p.mu.Lock()
	b := p.browser
	p.mu.Unlock()

	if b == nil {
		return nil
	}

	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("browser: rod health check: %w", err)
	}
	if err := page.Close(); err != nil {
		return fmt.Errorf("browser: rod health check close: %w", err)
	}

	return nil
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.browser == nil {
		return nil
	}

	var err error
	if p.cfg.RemoteURL != "" {
		pages, pErr := p.browser.Pages()
		if pErr == nil {
			for _, page := range pages {
				_ = page.Close()
			}
		}
	} else {
		err = p.browser.Close()
		if err != nil {
			err = fmt.Errorf("browser: rod close: %w", err)
		}
	}

	p.browser = nil
	return err
}

func isAllowedScriptDomain(host string, allowed []string) bool {
	for _, domain := range allowed {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func (p *Provider) resolveRemoteURL() (string, error) {
	// 如果 RemoteURL 已经是 ws:// 或 wss:// 开头，直接使用，不做 resolve
	if strings.HasPrefix(p.cfg.RemoteURL, "ws://") || strings.HasPrefix(p.cfg.RemoteURL, "wss://") {
		return p.cfg.RemoteURL, nil
	}

	if p.cfg.RemoteToken == "" {
		resolved, err := launcher.ResolveURL(p.cfg.RemoteURL)
		if err != nil {
			return "", fmt.Errorf("browser: rod resolve remote url %q: %w", p.cfg.RemoteURL, err)
		}
		return resolved, nil
	}

	versionURL := strings.TrimRight(p.cfg.RemoteURL, "/") + "/json/version?token=" + p.cfg.RemoteToken

	client := &http.Client{Timeout: resolveRemoteTimeout}
	resp, err := client.Get(versionURL)
	if err != nil {
		return "", fmt.Errorf("browser: rod resolve remote url %q: %w", versionURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("browser: rod read version response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("browser: rod version endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"` //nolint:tagliatelle // Chrome DevTools Protocol 定义的字段名
	}
	if err := json.Unmarshal(body, &version); err != nil {
		return "", fmt.Errorf("browser: rod parse version response: %w", err)
	}

	wsURL := version.WebSocketDebuggerURL
	if wsURL == "" {
		return "", fmt.Errorf("browser: rod empty webSocketDebuggerUrl from %q", versionURL)
	}

	remoteHost := strings.TrimPrefix(p.cfg.RemoteURL, "http://")
	remoteHost = strings.TrimPrefix(remoteHost, "https://")
	remoteHost = strings.TrimRight(remoteHost, "/")

	wsURL = strings.Replace(wsURL, "0.0.0.0", strings.Split(remoteHost, ":")[0], 1)
	if !strings.Contains(wsURL, remoteHost) {
		wsURL = "ws://" + remoteHost + "/"
	}

	if strings.Contains(wsURL, "?") {
		wsURL += "&token=" + p.cfg.RemoteToken
	} else {
		wsURL += "?token=" + p.cfg.RemoteToken
	}

	return wsURL, nil
}

// generateWebSocketKey 生成符合 RFC 6455 规范的 Sec-WebSocket-Key (16 字节随机数的 base64 编码)
func generateWebSocketKey() string {
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	return base64.StdEncoding.EncodeToString(key)
}
