package browser

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrPoolClosed 表示浏览器池已关闭
	ErrPoolClosed = errors.New("browser: pool closed")

	// ErrAcquireTimeout 表示获取浏览器实例超时
	ErrAcquireTimeout = errors.New("browser: acquire timeout")

	// ErrProviderNotFound 表示未注册对应类型的 Provider
	ErrProviderNotFound = errors.New("browser: provider not found")

	// ErrUnsupported 表示当前浏览器实现不支持该操作
	ErrUnsupported = errors.New("browser: operation not supported")

	// ErrAllBlocked 表示所有浏览器类型都被风控拦截
	ErrAllBlocked = errors.New("browser: all browser types blocked")

	// ErrNoHealthyEndpoint 表示没有可用的健康远程节点
	ErrNoHealthyEndpoint = errors.New("browser: no healthy endpoint available")

	// ErrWAFBlocked 表示页面被 WAF/风控系统拦截
	ErrWAFBlocked = errors.New("browser: waf blocked")
)

// WAFBlockedError 携带 WAF 拦截详情的结构化错误
type WAFBlockedError struct {
	Result BlockResult
	URL    string
}

func (e *WAFBlockedError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("browser: waf blocked (%s: %s) url=%s", e.Result.Type, e.Result.Reason, e.URL)
	}
	return fmt.Sprintf("browser: waf blocked (%s: %s)", e.Result.Type, e.Result.Reason)
}

func (e *WAFBlockedError) Unwrap() error { return ErrWAFBlocked }

var connectionErrorPatterns = []string{
	"use of closed network connection",
	"unexpected EOF",
	"broken pipe",
	"connection reset by peer",
	"connection refused",
	"i/o timeout",
	"rod create panic",
}

// IsConnectionError 判断是否为底层连接级错误
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, pattern := range connectionErrorPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}
