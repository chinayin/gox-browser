//go:build integration

package stealthtest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/chinayin/gox-browser/browser"
)

// testFingerprintScan 检测 fingerprint-scan.com
// 等待 20s，读取 bot risk score，检测 headless 信号
func testFingerprintScan(ctx context.Context, b browser.Browser) (*testResult, error) {
	if err := b.Navigate(ctx, "https://fingerprint-scan.com"); err != nil {
		return nil, fmt.Errorf("stealthtest: fingerprint_scan navigate: %w", err)
	}
	if err := b.WaitStable(ctx); err != nil {
		return nil, fmt.Errorf("stealthtest: fingerprint_scan wait stable: %w", err)
	}

	// 轮询等待分数出现（最多 20s）
	var scoreText string
	scoreJS := `() => (function(){
		var el = document.querySelector('#fingerprintScore') || document.querySelector('[class*="score"]');
		return el ? el.textContent.trim() : '';
	})()`

	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		var err error
		scoreText, err = b.Eval(ctx, scoreJS)
		if err == nil && scoreText != "" {
			break
		}
	}

	// 解析分数
	re := regexp.MustCompile(`([0-9.]+)`)
	match := re.FindStringSubmatch(scoreText)
	riskScore := -1.0 // -1 表示未读到
	if len(match) > 1 {
		if v, err := strconv.ParseFloat(match[1], 64); err == nil {
			riskScore = v
		}
	}

	// 检测 headless 信号
	headlessJS := `() => JSON.stringify((function(){
		var signals = {};
		signals.noTaskbar = (window.outerHeight === window.innerHeight);
		signals.noContentIndex = !('contentIndex' in navigator || 'getInstalledRelatedApps' in navigator);
		signals.webdriver = navigator.webdriver === true;
		signals.automationControlled = !!window.cdc_adoQpoasnfa76pfcZLmcfl_Array;
		return signals;
	})())`

	signalsRaw, err := b.Eval(ctx, headlessJS)
	if err != nil {
		slog.Warn("stealthtest: fingerprint_scan eval headless signals failed", "error", err)
		signalsRaw = "{}"
	}

	var signals map[string]bool
	if err := json.Unmarshal([]byte(signalsRaw), &signals); err != nil {
		signals = map[string]bool{}
	}

	// 统计 headless 信号
	headlessSignals := 0
	for _, v := range signals {
		if v {
			headlessSignals++
		}
	}

	// 判断通过条件：读到分数且 < 50，且无 headless 信号
	passed := riskScore >= 0 && riskScore < 50 && headlessSignals == 0
	score := 0.0
	if riskScore >= 0 {
		score = 1.0 - (riskScore / 100.0)
	}

	return &testResult{
		passed: passed,
		score:  score,
		details: map[string]any{
			"risk_score":       riskScore,
			"score_found":      riskScore >= 0,
			"headless_signals": headlessSignals,
			"signals":          signals,
		},
	}, nil
}

// testCreepJS 检测 CreepJS (abrahamjuliot.github.io/creepjs)
// 等待 30s，从 window.Fingerprint 对象提取 headless 评分
func testCreepJS(ctx context.Context, b browser.Browser) (*testResult, error) {
	if err := b.Navigate(ctx, "https://abrahamjuliot.github.io/creepjs"); err != nil {
		return nil, fmt.Errorf("stealthtest: creepjs navigate: %w", err)
	}
	if err := b.WaitStable(ctx); err != nil {
		return nil, fmt.Errorf("stealthtest: creepjs wait stable: %w", err)
	}

	// 轮询等待 Fingerprint 对象出现（最多 30s）
	fpJS := `() => (function(){
		try {
			if(window.Fingerprint && window.Fingerprint.headless) {
				return JSON.stringify(window.Fingerprint.headless);
			}
		} catch(e) {}
		return '';
	})()`

	var fpRaw string
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		var err error
		fpRaw, err = b.Eval(ctx, fpJS)
		if err == nil && fpRaw != "" && fpRaw != "{}" {
			break
		}
	}
	if fpRaw == "" {
		fpRaw = "{}"
	}

	var fpData struct {
		HeadlessRating     float64 `json:"headlessRating"`
		LikeHeadlessRating float64 `json:"likeHeadlessRating"`
		StealthRating      float64 `json:"stealthRating"`
		Chromium           bool    `json:"chromium"`
	}
	var fpHeadless map[string]any
	_ = json.Unmarshal([]byte(fpRaw), &fpData)
	_ = json.Unmarshal([]byte(fpRaw), &fpHeadless)

	// 如果从 JS 对象读到了数据，用对象里的 rating
	headlessPercent := fpData.HeadlessRating
	likeHeadlessPercent := fpData.LikeHeadlessRating
	stealthPercent := fpData.StealthRating

	// 如果 JS 对象没数据，尝试从页面文本正则提取
	if headlessPercent == 0 && likeHeadlessPercent == 0 && stealthPercent == 0 {
		text, _ := b.Text(ctx)
		headlessPercent = extractPercent(text, `headless\s*[:\s]*([0-9.]+)%`)
		stealthPercent = extractPercent(text, `stealth\s*[:\s]*([0-9.]+)%`)
		likeHeadlessPercent = extractPercent(text, `likeHeadless\s*[:\s]*([0-9.]+)%`)
	}

	// headlessRating 和 stealthRating 越低越好
	passed := headlessPercent <= 50 && likeHeadlessPercent <= 50 && stealthPercent <= 50

	return &testResult{
		passed: passed,
		score:  1.0 - (headlessPercent / 100.0),
		details: map[string]any{
			"headless_percent":      headlessPercent,
			"stealth_percent":       stealthPercent,
			"like_headless_percent": likeHeadlessPercent,
			"fingerprint_headless":  fpHeadless,
		},
	}, nil
}

// extractPercent 从文本中用正则提取百分比数值
func extractPercent(text, pattern string) float64 {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		if v, err := strconv.ParseFloat(match[1], 64); err == nil {
			return v
		}
	}
	return 100.0 // 默认最差
}
