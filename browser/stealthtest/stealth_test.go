//go:build integration

package stealthtest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/chinayin/gox-browser/browser"
)

// testSannysoft 检测 bot.sannysoft.com 的反检测结果
// 解析页面表格，统计 failed 行数
func testSannysoft(ctx context.Context, b browser.Browser) (*testResult, error) {
	if err := b.Navigate(ctx, "https://bot.sannysoft.com"); err != nil {
		return nil, fmt.Errorf("stealthtest: sannysoft navigate: %w", err)
	}
	if err := b.WaitStable(ctx); err != nil {
		return nil, fmt.Errorf("stealthtest: sannysoft wait stable: %w", err)
	}
	time.Sleep(3 * time.Second)

	js := `() => JSON.stringify((function(){
		var rows = document.querySelectorAll('table tr');
		var total = 0, failed = 0, details = [];
		rows.forEach(function(row){
			var cells = row.querySelectorAll('td');
			if(cells.length >= 2){
				total++;
				var cls = cells[cells.length-1].className || '';
				if(cls.indexOf('failed') !== -1){
					failed++;
					details.push(cells[0].textContent.trim());
				}
			}
		});
		return {total: total, failed: failed, failed_items: details};
	})())`

	raw, err := b.Eval(ctx, js)
	if err != nil {
		return nil, fmt.Errorf("stealthtest: sannysoft eval: %w", err)
	}

	var data struct {
		Total       int      `json:"total"`
		Failed      int      `json:"failed"`
		FailedItems []string `json:"failed_items"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("stealthtest: sannysoft parse result: %w", err)
	}

	score := 1.0
	if data.Total > 0 {
		score = float64(data.Total-data.Failed) / float64(data.Total)
	}

	return &testResult{
		passed: data.Failed == 0,
		score:  score,
		details: map[string]any{
			"total":        data.Total,
			"failed":       data.Failed,
			"failed_items": data.FailedItems,
		},
	}, nil
}

// testIncolumitas 检测 bot.incolumitas.com
// 轮询等待 30+ 项检测完成，统计 OK/FAIL
func testIncolumitas(ctx context.Context, b browser.Browser) (*testResult, error) {
	if err := b.Navigate(ctx, "https://bot.incolumitas.com"); err != nil {
		return nil, fmt.Errorf("stealthtest: incolumitas navigate: %w", err)
	}
	if err := b.WaitStable(ctx); err != nil {
		return nil, fmt.Errorf("stealthtest: incolumitas wait stable: %w", err)
	}

	var text string
	var lastCount int
	stableRounds := 0

	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		var err error
		text, err = b.Text(ctx)
		if err != nil {
			slog.Warn("stealthtest: incolumitas get text failed", "error", err)
			continue
		}
		okCount := strings.Count(text, "OK")
		failCount := strings.Count(text, "FAIL")
		current := okCount + failCount
		if current >= 30 && current == lastCount {
			stableRounds++
			if stableRounds >= 2 {
				break
			}
		} else {
			stableRounds = 0
		}
		lastCount = current
	}

	okCount := strings.Count(text, "OK")
	failCount := strings.Count(text, "FAIL")
	total := okCount + failCount
	score := 0.0
	if total > 0 {
		score = float64(okCount) / float64(total)
	}

	return &testResult{
		passed: score >= 0.9,
		score:  score,
		details: map[string]any{
			"ok":    okCount,
			"fail":  failCount,
			"total": total,
		},
	}, nil
}

// testBrowserscan 检测 browserscan.net/bot-detection
// 统计 Normal/Abnormal 出现次数
func testBrowserscan(ctx context.Context, b browser.Browser) (*testResult, error) {
	if err := b.Navigate(ctx, "https://www.browserscan.net/bot-detection"); err != nil {
		return nil, fmt.Errorf("stealthtest: browserscan navigate: %w", err)
	}
	if err := b.WaitStable(ctx); err != nil {
		return nil, fmt.Errorf("stealthtest: browserscan wait stable: %w", err)
	}
	time.Sleep(5 * time.Second)

	text, err := b.Text(ctx)
	if err != nil {
		return nil, fmt.Errorf("stealthtest: browserscan get text: %w", err)
	}

	normal := strings.Count(text, "Normal")
	abnormal := strings.Count(text, "Abnormal")
	total := normal + abnormal
	score := 0.0
	if total > 0 {
		score = float64(normal) / float64(total)
	}

	return &testResult{
		passed: abnormal == 0,
		score:  score,
		details: map[string]any{
			"normal":   normal,
			"abnormal": abnormal,
			"total":    total,
		},
	}, nil
}

// testDeviceAndBrowserInfo 检测 deviceandbrowserinfo.com/are_you_a_bot
// 解析页面 HTML 中的 isBot JSON 字段
func testDeviceAndBrowserInfo(ctx context.Context, b browser.Browser) (*testResult, error) {
	if err := b.Navigate(ctx, "https://deviceandbrowserinfo.com/are_you_a_bot"); err != nil {
		return nil, fmt.Errorf("stealthtest: deviceandbrowserinfo navigate: %w", err)
	}
	if err := b.WaitStable(ctx); err != nil {
		return nil, fmt.Errorf("stealthtest: deviceandbrowserinfo wait stable: %w", err)
	}
	time.Sleep(5 * time.Second)

	// 用 HTML 而非 Text，因为 JSON 数据可能在 script 或 pre 标签中
	html, err := b.HTML(ctx)
	if err != nil {
		return nil, fmt.Errorf("stealthtest: deviceandbrowserinfo get html: %w", err)
	}

	// 同时尝试 Text
	text, _ := b.Text(ctx)
	content := html + "\n" + text

	// 匹配多种格式: "isBot": true, "isBot":true, isBot: true
	reBot := regexp.MustCompile(`(?i)["']?isBot["']?\s*[:=]\s*(true|false)`)
	match := reBot.FindStringSubmatch(content)
	isBot := true // 默认假设被检测到
	if len(match) > 1 {
		isBot = strings.EqualFold(match[1], "true")
	}

	// 也检查页面中是否有明确的 "not a bot" 或 "human" 文案
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "you are not a bot") || strings.Contains(lowerText, "you are human") {
		isBot = false
	}
	if strings.Contains(lowerText, "you are a bot") || strings.Contains(lowerText, "bot detected") {
		isBot = true
	}

	return &testResult{
		passed: !isBot,
		score:  boolToScore(!isBot),
		details: map[string]any{
			"is_bot":    isBot,
			"matched":   len(match) > 0,
			"raw_match": match,
		},
	}, nil
}

// testFingerprintDemo 检测 demo.fingerprint.com/web-scraping
// 点击 Search 按钮，检查是否被拦截
func testFingerprintDemo(ctx context.Context, b browser.Browser) (*testResult, error) {
	if err := b.Navigate(ctx, "https://demo.fingerprint.com/web-scraping"); err != nil {
		return nil, fmt.Errorf("stealthtest: fingerprint_demo navigate: %w", err)
	}
	if err := b.WaitStable(ctx); err != nil {
		return nil, fmt.Errorf("stealthtest: fingerprint_demo wait stable: %w", err)
	}
	time.Sleep(3 * time.Second)

	// 尝试点击 Search 按钮
	_ = b.Click(ctx, "button[type='submit']")
	time.Sleep(5 * time.Second)

	text, err := b.Text(ctx)
	if err != nil {
		return nil, fmt.Errorf("stealthtest: fingerprint_demo get text: %w", err)
	}

	lowerText := strings.ToLower(text)

	// 检查是否被明确拦截（特定的拦截文案）
	blocked := strings.Contains(lowerText, "bot visitor detected") ||
		strings.Contains(lowerText, "access denied") ||
		strings.Contains(lowerText, "automated browser") ||
		strings.Contains(lowerText, "request blocked")

	// 检查是否有正常搜索结果（航班价格等）
	hasResult := strings.Contains(text, "$") ||
		strings.Contains(lowerText, "flight") ||
		strings.Contains(lowerText, "price")

	passed := !blocked || hasResult

	return &testResult{
		passed: passed,
		score:  boolToScore(passed),
		details: map[string]any{
			"blocked":    blocked,
			"has_result": hasResult,
		},
	}, nil
}

// testRecaptchaDemo 检测 recaptcha-demo.appspot.com
// 获取 reCAPTCHA v3 score
func testRecaptchaDemo(ctx context.Context, b browser.Browser) (*testResult, error) {
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

	// 点击提交按钮
	_ = b.Click(ctx, "button[type='submit']")
	_ = b.Click(ctx, "input[type='submit']")
	time.Sleep(5 * time.Second)

	text, err := b.Text(ctx)
	if err != nil {
		return &testResult{
			passed: true,
			score:  -1,
			details: map[string]any{
				"skipped": true,
				"reason":  err.Error(),
			},
		}, nil
	}

	re := regexp.MustCompile(`"score"\s*:\s*([0-9.]+)`)
	match := re.FindStringSubmatch(text)
	score := 0.0
	if len(match) > 1 {
		fmt.Sscanf(match[1], "%f", &score)
	}

	return &testResult{
		passed: score >= recaptchaScoreThreshold,
		score:  score,
		details: map[string]any{
			"recaptcha_score": score,
		},
	}, nil
}

func boolToScore(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
