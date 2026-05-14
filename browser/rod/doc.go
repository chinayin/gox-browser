// Package rod 提供基于 go-rod 的 Chrome 浏览器 Provider 实现。
//
// 支持本地 Chrome 和远程 Chrome (WebSocket) 两种模式，
// 内置 stealth 反检测、指纹随机化、资源拦截等能力。
//
// 用法:
//
//	provider := rod.NewProvider(rod.Config{
//	    Headless:          true,
//	    StealthMode:       true,
//	    RandomFingerprint: true,
//	})
//	pool.Register(provider)
package rod
