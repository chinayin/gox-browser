package browser

import "time"

// Type 浏览器类型
type Type int

const (
	TypeRodHeadless Type = iota // headless Chrome + stealth
	TypeRodHeaded               // 有头 Chrome
	TypeBrowserless             // Browserless v2 集群
	TypeSurf                    // 纯 HTTP TLS 伪装
	TypeCamoufox                // Camoufox 反检测浏览器 (预留)
)

// String 返回浏览器类型的可读名称
func (t Type) String() string {
	switch t {
	case TypeRodHeadless:
		return "rod-headless"
	case TypeRodHeaded:
		return "rod-headed"
	case TypeBrowserless:
		return "browserless"
	case TypeSurf:
		return "surf"
	case TypeCamoufox:
		return "camoufox"
	default:
		return "unknown"
	}
}

// Viewport 视口配置
type Viewport struct {
	Width  int
	Height int
	Scale  float64
	Mobile bool
}

// DefaultViewport 默认桌面视口
var DefaultViewport = Viewport{
	Width:  1920,
	Height: 1080,
	Scale:  1.0,
	Mobile: false,
}

// AcquireOpts 获取浏览器实例的选项
type AcquireOpts struct {
	Type      Type
	Viewport  Viewport
	UserAgent string
	Locale    string
	Proxy     string
}

// Cookie HTTP cookie
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"http_only"`
}

// PoolConfig 浏览器池配置
type PoolConfig struct {
	MaxInstances    int
	IdleTimeout     time.Duration
	AcquireTimeout  time.Duration
	HealthCheckFreq time.Duration
	PingTimeout     time.Duration // 复用空闲实例前的存活探测超时，零值取 2s
}

// DefaultPoolConfig 默认池配置
var DefaultPoolConfig = PoolConfig{
	MaxInstances:    8,
	IdleTimeout:     5 * time.Minute,
	AcquireTimeout:  30 * time.Second,
	HealthCheckFreq: 30 * time.Second,
	PingTimeout:     2 * time.Second,
}

// PoolStats 浏览器池统计信息
type PoolStats struct {
	Total     int
	Available int
	InUse     int
	ByType    map[Type]int
}

// FetchResult 抓取结果
type FetchResult struct {
	HTML         string         `json:"html"`
	Title        string         `json:"title"`
	FinalType    Type           `json:"final_type"`
	Attempts     []FetchAttempt `json:"attempts"`
	TotalLatency time.Duration  `json:"total_latency"`
}

// FetchAttempt 单次尝试记录
type FetchAttempt struct {
	Type     Type          `json:"type"`
	Duration time.Duration `json:"duration"`
	Blocked  bool          `json:"blocked"`
	Reason   string        `json:"reason"`
	Err      error         `json:"-"`
}

// BlockResult 风控检测结果
type BlockResult struct {
	Blocked bool   `json:"blocked"`
	Reason  string `json:"reason"`
	Type    string `json:"type"`
}

// DetectResult 页面类型探测结果
type DetectResult struct {
	SuggestedType Type     `json:"suggested_type"`
	IsSSR         bool     `json:"is_ssr"`
	Score         int      `json:"score"`
	Signals       []string `json:"signals"`
}
