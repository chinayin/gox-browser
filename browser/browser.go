package browser

import "context"

// Browser 统一浏览器操作接口
//
// 所有浏览器后端（Rod、Browserless、Surf 等）都实现此接口。
// 不支持的操作应返回 ErrUnsupported。
type Browser interface {
	Navigate(ctx context.Context, url string) error
	WaitStable(ctx context.Context) error
	HTML(ctx context.Context) (string, error)
	Text(ctx context.Context) (string, error)
	Title(ctx context.Context) (string, error)
	URL(ctx context.Context) (string, error)
	Screenshot(ctx context.Context) ([]byte, error)
	Eval(ctx context.Context, js string) (string, error)
	// EvalDirect 直接通过 CDP 执行 JS，不等待 navigation lifecycle。
	// 使用场景: 页面目标元素已确认存在，只需操作 DOM (SPA 表单交互、DOM 读取)。
	// 不确定目标元素是否存在时，请先用 WaitSelector 或自行轮询确认后再调用。
	EvalDirect(ctx context.Context, js string) (string, error)
	Click(ctx context.Context, selector string) error
	Type(ctx context.Context, selector, text string) error
	WaitSelector(ctx context.Context, selector string) error
	Cookies(ctx context.Context) ([]Cookie, error)
	SetCookies(ctx context.Context, cookies []Cookie) error
	BrowserType() Type
	Close() error
}

// Provider 浏览器实例工厂接口
//
// 每种浏览器类型实现一个 Provider，负责创建和管理该类型的实例。
type Provider interface {
	Type() Type
	Create(ctx context.Context, opts AcquireOpts) (Browser, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

// BlockDetector 风控检测接口
type BlockDetector interface {
	Detect(html, title string, statusCode int) BlockResult
}
