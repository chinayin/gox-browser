package browser

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// UserAgents 预定义的 User-Agent 列表
var UserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
}

// Viewports 预定义的桌面视口列表
var Viewports = []Viewport{
	{Width: 1920, Height: 1080, Scale: 1.0},
	{Width: 1366, Height: 768, Scale: 1.0},
	{Width: 1536, Height: 864, Scale: 1.25},
	{Width: 1440, Height: 900, Scale: 1.0},
	{Width: 1680, Height: 1050, Scale: 1.0},
	{Width: 2560, Height: 1440, Scale: 1.0},
	{Width: 1280, Height: 720, Scale: 1.0},
	{Width: 1600, Height: 900, Scale: 1.0},
}

// Timezones 预定义的时区列表
var Timezones = []string{
	"America/New_York", "America/Chicago", "America/Denver",
	"America/Los_Angeles", "America/Phoenix",
	"Europe/London", "Europe/Paris", "Europe/Berlin",
	"Asia/Tokyo", "Asia/Shanghai", "Asia/Singapore",
	"Australia/Sydney",
}

// Locales 预定义的语言区域列表
var Locales = []string{
	"en-US", "en-GB", "en-CA", "en-AU",
	"de-DE", "fr-FR", "ja-JP", "zh-CN",
}

// fingerprint.go 中使用 math/rand/v2 是有意为之：
// 浏览器指纹随机化不需要密码学安全的随机数，只需要分布均匀。
// 使用 crypto/rand 会引入不必要的性能开销。

//nolint:gosec // G404: 指纹随机化不需要密码学安全随机数
func RandomUserAgent() string {
	return UserAgents[rand.IntN(len(UserAgents))]
}

// RandomViewport 随机选择一个桌面视口
//
//nolint:gosec // G404: 指纹随机化不需要密码学安全随机数
func RandomViewport() Viewport {
	return Viewports[rand.IntN(len(Viewports))]
}

// RandomTimezone 随机选择一个时区
//
//nolint:gosec // G404: 指纹随机化不需要密码学安全随机数
func RandomTimezone() string {
	return Timezones[rand.IntN(len(Timezones))]
}

// RandomLocale 随机选择一个语言区域
//
//nolint:gosec // G404: 指纹随机化不需要密码学安全随机数
func RandomLocale() string {
	return Locales[rand.IntN(len(Locales))]
}

// TruncateUA 截断过长的 User-Agent 用于日志输出
func TruncateUA(ua string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 60
	}
	if len(ua) > maxLen {
		return ua[:maxLen] + "..."
	}
	return ua
}

// PlatformFromUA 根据 User-Agent 推断平台标识
func PlatformFromUA(ua string) string {
	switch {
	case strings.Contains(ua, "Macintosh"):
		return "MacIntel"
	case strings.Contains(ua, "Linux"):
		return "Linux x86_64"
	default:
		return "Win32"
	}
}

// PlatformOverrideScript 生成 navigator.platform 覆盖脚本
func PlatformOverrideScript(platform string) string {
	return fmt.Sprintf(`
		Object.defineProperty(navigator, 'platform', {
			get: () => '%s',
			configurable: true
		});
	`, platform)
}
