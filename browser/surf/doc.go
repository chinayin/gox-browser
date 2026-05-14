// Package surf 提供基于 TLS 指纹伪装的纯 HTTP 客户端 Provider 实现。
//
// Surf 没有 JS 引擎，不支持页面交互 (Click、Type、Eval 等返回 ErrUnsupported)，
// 但资源占用极低 (~5MB)，适合不需要 JS 渲染的 SSR 页面抓取场景。
//
// 用法:
//
//	provider := surf.NewProvider(surf.Config{
//	    Timeout: 30 * time.Second,
//	    Proxy:   "socks5://127.0.0.1:1080",
//	})
//	pool.Register(provider)
package surf
