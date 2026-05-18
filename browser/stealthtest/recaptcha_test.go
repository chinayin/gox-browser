//go:build integration

package stealthtest

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/chinayin/gox-browser/browser"
	"github.com/chinayin/gox-browser/browser/browserless"
	"github.com/chinayin/gox-browser/browser/rod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- 配置 (修改这里即可) ---

const (
	// CloakBrowser 远程地址
	cloakBrowserRemoteURL = "http://127.0.0.1:9222"

	// Browserless 端点地址 (token 与 docker-compose.yml 中 TOKEN 一致)
	browserlessEndpointURL = "http://127.0.0.1:3000?token=test123"

	// 单个测试用例超时
	caseTimeout = 60 * time.Second

	// 导航超时（站点不可达时快速失败）
	navigateTimeout = 15 * time.Second

	// reCAPTCHA v3 通过阈值
	recaptchaScoreThreshold = 0.7
)

// screenshotDir 截图输出目录（相对于项目根目录）
var screenshotDir = filepath.Join("..", "..", "runtime", "stealthtest")

// --- 类型定义 ---

type testCase struct {
	name   string
	url    string
	runner func(ctx context.Context, b browser.Browser) (*testResult, error)
}

type testResult struct {
	passed   bool
	score    float64
	details  map[string]any
	duration time.Duration
}

// --- 用例注册 ---

var allStealthCases = []testCase{
	{name: "sannysoft", url: "https://bot.sannysoft.com", runner: testSannysoft},
	{name: "incolumitas", url: "https://bot.incolumitas.com", runner: testIncolumitas},
	{name: "browserscan", url: "https://www.browserscan.net/bot-detection", runner: testBrowserscan},
	{name: "deviceandbrowserinfo", url: "https://deviceandbrowserinfo.com/are_you_a_bot", runner: testDeviceAndBrowserInfo},
	{name: "fingerprint_demo", url: "https://demo.fingerprint.com/web-scraping", runner: testFingerprintDemo},
	{name: "recaptcha_demo", url: "https://recaptcha-demo.appspot.com/recaptcha-v3-request-scores.php", runner: testRecaptchaDemo},
}

var allFingerprintCases = []testCase{
	{name: "fingerprint_scan", url: "https://fingerprint-scan.com", runner: testFingerprintScan},
	{name: "creepjs", url: "https://abrahamjuliot.github.io/creepjs", runner: testCreepJS},
}

var allRecaptchaCases = []testCase{
	{name: "recaptcha_v3_score", url: "https://recaptcha-demo.appspot.com/recaptcha-v3-request-scores.php", runner: testRecaptchaScore},
}

func allCases() []testCase {
	cases := make([]testCase, 0, len(allStealthCases)+len(allFingerprintCases)+len(allRecaptchaCases))
	cases = append(cases, allStealthCases...)
	cases = append(cases, allFingerprintCases...)
	cases = append(cases, allRecaptchaCases...)
	return cases
}

// --- reCAPTCHA v3 评分测试 ---

// testRecaptchaScore reCAPTCHA v3 评分专项测试，期望 >= 0.7
func testRecaptchaScore(ctx context.Context, b browser.Browser) (*testResult, error) {
	navCtx, navCancel := context.WithTimeout(ctx, navigateTimeout)
	defer navCancel()

	if err := b.Navigate(navCtx, "https://recaptcha-demo.appspot.com/recaptcha-v3-request-scores.php"); err != nil {
		return &testResult{
			passed: true,
			score:  -1,
			details: map[string]any{
				"skipped": true,
				"reason":  err.Error(),
			},
		}, nil
	}
	if err := b.WaitStable(ctx); err != nil {
		return &testResult{
			passed: true,
			score:  -1,
			details: map[string]any{
				"skipped": true,
				"reason":  err.Error(),
			},
		}, nil
	}
	time.Sleep(3 * time.Second)

	// 点击提交
	_ = b.Click(ctx, "button[type='submit']")
	_ = b.Click(ctx, "input[type='submit']")
	time.Sleep(5 * time.Second)

	// 轮询提取 score
	re := regexp.MustCompile(`"score"\s*:\s*([0-9.]+)`)
	var score float64

	for i := 0; i < 10; i++ {
		text, err := b.Text(ctx)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		match := re.FindStringSubmatch(text)
		if len(match) > 1 {
			fmt.Sscanf(match[1], "%f", &score)
			if score > 0 {
				break
			}
		}
		time.Sleep(2 * time.Second)
	}

	if score == 0 {
		// 没拿到分数，可能页面没加载出来
		return &testResult{
			passed: true,
			score:  -1,
			details: map[string]any{
				"skipped":   true,
				"reason":    "score not found",
				"threshold": recaptchaScoreThreshold,
			},
		}, nil
	}

	return &testResult{
		passed: score >= recaptchaScoreThreshold,
		score:  score,
		details: map[string]any{
			"recaptcha_score": score,
			"threshold":       recaptchaScoreThreshold,
		},
	}, nil
}

// --- Runner ---

func runCase(ctx context.Context, t *testing.T, b browser.Browser, tc testCase) {
	t.Helper()

	caseCtx, cancel := context.WithTimeout(ctx, caseTimeout)
	defer cancel()

	start := time.Now()
	result, err := tc.runner(caseCtx, b)
	duration := time.Since(start)

	if err != nil {
		// 失败时尝试截图
		saveScreenshot(ctx, b, tc.name)
		assert.NoError(t, err, "name=%s duration=%s", tc.name, duration)
		return
	}

	result.duration = duration

	// 网络不可达等情况标记为 skip
	if skipped, ok := result.details["skipped"].(bool); ok && skipped {
		reason, _ := result.details["reason"].(string)
		t.Skipf("skipped: %s (duration: %s)", reason, duration.Round(time.Millisecond))
		return
	}

	slog.Info("stealthtest: case completed",
		"name", tc.name,
		"passed", result.passed,
		"score", fmt.Sprintf("%.2f", result.score),
		"duration", duration.Round(time.Millisecond),
	)

	if !result.passed {
		saveScreenshot(ctx, b, tc.name)
	}

	assert.True(t, result.passed, "name=%s score=%.2f details=%v", tc.name, result.score, result.details)
}

func saveScreenshot(ctx context.Context, b browser.Browser, name string) {
	data, err := b.Screenshot(ctx)
	if err != nil {
		slog.Warn("stealthtest: screenshot failed", "name", name, "error", err)
		return
	}

	if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
		slog.Warn("stealthtest: create screenshot dir failed", "error", err)
		return
	}

	filename := fmt.Sprintf("%s_%d.png", name, time.Now().Unix())
	path := filepath.Join(screenshotDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		slog.Warn("stealthtest: save screenshot failed", "path", path, "error", err)
		return
	}

	slog.Info("stealthtest: screenshot saved", "path", path)
}

// --- 测试入口 ---

func TestStealth(t *testing.T) {
	providers := []struct {
		name   string
		create func(t *testing.T) browser.Browser
	}{
		{
			name: "Rod",
			create: func(t *testing.T) browser.Browser {
				t.Helper()
				cfg := rod.Config{
					Headless:          true,
					StealthMode:       true,
					RandomFingerprint: true,
				}
				p := rod.NewProvider(cfg)
				t.Cleanup(func() { _ = p.Close() })

				b, err := p.Create(context.Background(), browser.AcquireOpts{})
				if err != nil {
					t.Skipf("rod provider not available: %v", err)
				}
				t.Cleanup(func() { _ = b.Close() })
				return b
			},
		},
		{
			name: "CloakBrowser",
			create: func(t *testing.T) browser.Browser {
				t.Helper()
				cfg := rod.Config{
					RemoteURL:         cloakBrowserRemoteURL,
					Headless:          true,
					StealthMode:       false,
					RandomFingerprint: false,
				}
				p := rod.NewProvider(cfg)
				t.Cleanup(func() { _ = p.Close() })

				b, err := p.Create(context.Background(), browser.AcquireOpts{})
				if err != nil {
					t.Skipf("cloakbrowser provider not available: %v", err)
				}
				t.Cleanup(func() { _ = b.Close() })
				return b
			},
		},
		{
			name: "Browserless",
			create: func(t *testing.T) browser.Browser {
				t.Helper()
				cfg := browserless.Config{
					Stealth:           true,
					RandomFingerprint: true,
					Endpoints: []browserless.EndpointConfig{
						{URL: browserlessEndpointURL},
					},
				}
				p := browserless.NewProvider(cfg)
				t.Cleanup(func() { _ = p.Close() })

				b, err := p.Create(context.Background(), browser.AcquireOpts{})
				if err != nil {
					t.Skipf("browserless provider not available: %v", err)
				}
				t.Cleanup(func() { _ = b.Close() })
				return b
			},
		},
	}

	cases := allCases()
	ctx := context.Background()

	for _, prov := range providers {
		t.Run(prov.name, func(t *testing.T) {
			b := prov.create(t)
			for _, tc := range cases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					runCase(ctx, t, b, tc)
				})
			}
		})
	}
}

// TestStealth_Quick 仅跑 sannysoft 快速验证
func TestStealth_Quick(t *testing.T) {
	cfg := rod.Config{
		Headless:          true,
		StealthMode:       true,
		RandomFingerprint: true,
	}
	p := rod.NewProvider(cfg)
	t.Cleanup(func() { _ = p.Close() })

	b, err := p.Create(context.Background(), browser.AcquireOpts{})
	if err != nil {
		t.Skipf("rod provider not available: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })

	ctx := context.Background()
	tc := allStealthCases[0] // sannysoft
	t.Run(tc.name, func(t *testing.T) {
		caseCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		result, err := tc.runner(caseCtx, b)
		require.NoError(t, err)
		t.Logf("score=%.2f details=%v", result.score, result.details)
		assert.True(t, result.passed)
	})
}
