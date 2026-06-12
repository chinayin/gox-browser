package browserless

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	browser "github.com/chinayin/gox-browser/browser"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	EditionOpenSource = ""
	EditionEnterprise = "enterprise"
)

// Config Browserless v2 Provider 配置
type Config struct {
	Edition               string           `json:"edition" yaml:"edition"`
	Stealth               bool             `json:"stealth" yaml:"stealth"`
	Headless              *bool            `json:"headless" yaml:"headless"`
	Evasion               bool             `json:"evasion" yaml:"evasion"`
	BlockAds              bool             `json:"block_ads" yaml:"block_ads"`
	RandomFingerprint     bool             `json:"random_fingerprint" yaml:"random_fingerprint"`
	Proxy                 string           `json:"proxy" yaml:"proxy"`
	BlockResources        []string         `json:"block_resources" yaml:"block_resources"`
	BlockThirdPartyScript bool             `json:"block_third_party_script" yaml:"block_third_party_script"`
	AllowedScriptDomains  []string         `json:"allowed_script_domains" yaml:"allowed_script_domains"`
	Endpoints             []EndpointConfig `json:"endpoints" yaml:"endpoints"`
	RouteStrategy         string           `json:"route_strategy" yaml:"route_strategy"`
	HealthCheckInterval   time.Duration    `json:"health_check_interval" yaml:"health_check_interval"`
}

type EndpointConfig struct {
	URL    string `json:"url" yaml:"url"`
	Weight int    `json:"weight" yaml:"weight"`
}

// Provider Browserless v2 集群 Provider
type Provider struct {
	cfg    Config
	router *Router
}

// 编译期接口合规断言
var _ browser.Provider = (*Provider)(nil)

func NewProvider(cfg Config) *Provider {
	if cfg.RouteStrategy == "" {
		cfg.RouteStrategy = "least-load"
	}
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 10 * time.Second
	}

	eps := make([]Endpoint, len(cfg.Endpoints))
	for i, ec := range cfg.Endpoints {
		w := ec.Weight
		if w <= 0 {
			w = 1
		}
		baseURL, token := parseEndpointURL(ec.URL)
		eps[i] = Endpoint{URL: baseURL, Token: token, Weight: w, Healthy: true}
	}

	return &Provider{
		cfg:    cfg,
		router: NewRouter(eps, cfg.RouteStrategy, cfg.HealthCheckInterval),
	}
}

func (p *Provider) Type() browser.Type { return browser.TypeBrowserless }

func (p *Provider) Create(ctx context.Context, opts browser.AcquireOpts) (browser.Browser, error) {
	ep := p.router.Pick()
	if ep == nil {
		return nil, browser.ErrNoHealthyEndpoint
	}

	b, err := p.ensureBrowser(ctx, ep)
	if err != nil {
		p.router.MarkUnhealthy(ep.URL)
		return nil, fmt.Errorf("browser: browserless connect %s: %w", ep.URL, err)
	}

	page, err := p.createPage(b, opts)
	if err != nil {
		return nil, err
	}

	var router *rod.HijackRouter
	if p.hijackEnabled() {
		router, err = p.setupRequestInterception(page)
		if err != nil {
			_ = page.Close()
			return nil, err
		}
	}

	p.applyViewportAndFingerprint(page, opts)

	// 输出 DevTools debugger URL，可在浏览器中打开实时查看页面画面
	p.logDebuggerURL(ep, page)

	return &browserlessBrowser{page: page, browser: b, router: router, headless: true, endpoint: ep.URL}, nil
}

func (p *Provider) createPage(b *rod.Browser, opts browser.AcquireOpts) (*rod.Page, error) {
	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("browser: browserless create page: %w", err)
	}

	// NOTE: 不再注入自定义 evasionCoreScript — Browserless 后端完全依赖
	// 服务端 ?stealth=true 参数提供的官方 stealth 能力。
	// 自定义注入可能与服务端 stealth 冲突，增加被检测风险。
	// if p.cfg.Evasion {
	// 	if err := applyEvasionCore(page, opts.UserAgent); err != nil {
	// 		slog.Warn("browser: browserless evasion core injection failed", "error", err)
	// 	}
	// }

	return page, nil
}

func (p *Provider) applyViewportAndFingerprint(page *rod.Page, opts browser.AcquireOpts) {
	vp := opts.Viewport
	if vp.Width == 0 {
		vp = browser.DefaultViewport
	}

	if p.cfg.RandomFingerprint {
		if vp == browser.DefaultViewport {
			vp = randomViewport()
		}
		// 指纹覆盖失败意味着反检测部分失效，必须可观测
		tz := randomTimezone()
		if err := (proto.EmulationSetTimezoneOverride{TimezoneID: tz}).Call(page); err != nil {
			slog.Warn("browser: browserless set timezone override failed", "timezone", tz, "error", err)
		}
		locale := randomLocale()
		if err := (proto.EmulationSetLocaleOverride{Locale: locale}).Call(page); err != nil {
			slog.Warn("browser: browserless set locale override failed", "locale", locale, "error", err)
		}
	}

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width: vp.Width, Height: vp.Height, DeviceScaleFactor: vp.Scale, Mobile: vp.Mobile,
	}); err != nil {
		slog.Warn("browser: browserless set viewport failed",
			"viewport", fmt.Sprintf("%dx%d", vp.Width, vp.Height), "error", err)
	}
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
		host := ctx.Request.URL().Host

		if blocked[rt] {
			ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}

		if p.cfg.BlockThirdPartyScript && host != "" {
			allowed := false
			for _, domain := range p.cfg.AllowedScriptDomains {
				if host == domain || strings.HasSuffix(host, "."+domain) {
					allowed = true
					break
				}
			}
			if !allowed {
				ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
				return
			}
		}

		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	}); err != nil {
		return nil, fmt.Errorf("browser: browserless setup request interception: %w", err)
	}
	go router.Run()

	return router, nil
}

func (p *Provider) ensureBrowser(ctx context.Context, ep *Endpoint) (*rod.Browser, error) {
	if err := p.router.CheckCapacity(ctx, ep); err != nil {
		return nil, fmt.Errorf("browser: browserless pre-flight %s: %w", ep.URL, err)
	}

	wsURL := p.buildWSURL(ep)

	b := rod.New().ControlURL(wsURL)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("browser: browserless connect: %w", err)
	}

	if _, err := b.Pages(); err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("browser: browserless post-connect check: %w", err)
	}

	return b, nil
}

func (p *Provider) buildWSURL(ep *Endpoint) string {
	u := strings.TrimRight(ep.URL, "/")
	u = strings.Replace(u, "http://", "ws://", 1)
	u = strings.Replace(u, "https://", "wss://", 1)
	u += "/chromium"

	params := url.Values{}
	if ep.Token != "" {
		params.Set("token", ep.Token)
	}
	if p.cfg.Stealth {
		params.Set("stealth", "true")
	}
	if p.cfg.Headless != nil && !*p.cfg.Headless {
		params.Set("headless", "false")
	}
	if p.cfg.BlockAds {
		params.Set("blockAds", "true")
	}

	// 通过 launch args 透传代理给容器内 Chrome
	if p.cfg.Proxy != "" {
		launch := struct {
			Args []string `json:"args"`
		}{
			Args: []string{"--proxy-server=" + p.cfg.Proxy},
		}
		launchJSON, _ := json.Marshal(launch)
		params.Set("launch", string(launchJSON))
	}

	if q := params.Encode(); q != "" {
		u += "?" + q
	}

	return u
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	return p.router.HealthCheckAll(ctx)
}

func (p *Provider) Close() error {
	p.router.Stop()
	slog.Info("browser: browserless provider closed")
	return nil
}

// logDebuggerURL 输出 DevTools 远程调试 URL，可在浏览器中打开实时查看页面画面。
// 格式: http://<endpoint>/devtools/inspector.html?ws=<host>/devtools/page/<targetID>&token=<token>
func (p *Provider) logDebuggerURL(ep *Endpoint, page *rod.Page) {
	targetID := string(page.TargetID)
	if targetID == "" {
		return
	}

	// 从 endpoint URL 提取 host (去掉 scheme)
	host := strings.TrimPrefix(ep.URL, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimRight(host, "/")

	// 构建 devtools inspector URL
	wsPath := fmt.Sprintf("%s/devtools/page/%s", host, targetID)
	if ep.Token != "" {
		wsPath += "?token=" + ep.Token
	}

	debuggerURL := fmt.Sprintf("http://%s/devtools/inspector.html?ws=%s", host, wsPath)

	slog.Info("browser: browserless session created",
		"endpoint", ep.URL,
		"target_id", targetID,
		"debugger_url", debuggerURL,
	)
}

func parseEndpointURL(rawURL string) (baseURL, token string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, ""
	}
	token = u.Query().Get("token")
	u.RawQuery = ""
	return u.String(), token
}
