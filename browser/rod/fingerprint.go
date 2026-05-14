package rod

import browser "github.com/chinayin/gox-browser/browser"

// randomViewport 委托给共享实现
func randomViewport() browser.Viewport {
	return browser.RandomViewport()
}

// randomTimezone 委托给共享实现
func randomTimezone() string {
	return browser.RandomTimezone()
}

// randomLocale 委托给共享实现
func randomLocale() string {
	return browser.RandomLocale()
}
