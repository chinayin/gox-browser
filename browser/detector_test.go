package browser_test

import (
	"strings"
	"testing"

	"github.com/chinayin/gox-browser/browser"
	"github.com/stretchr/testify/assert"
)

func TestDefaultBlockDetector_Detect_Cloudflare(t *testing.T) {
	detector := browser.NewBlockDetector()

	tests := []struct {
		name   string
		html   string
		title  string
		signal string
	}{
		{
			name:   "just_a_moment_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "Just a moment...</body></html>",
			title:  "Normal Page",
			signal: "just a moment",
		},
		{
			name:   "checking_your_browser_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "Checking your browser before accessing</body></html>",
			title:  "Normal Page",
			signal: "checking your browser",
		},
		{
			name:   "cf_browser_verification_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "<div id=\"cf-browser-verification\"></div></body></html>",
			title:  "Normal Page",
			signal: "cf-browser-verification",
		},
		{
			name:   "cf_chl_opt_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "<script>cf_chl_opt={}</script></body></html>",
			title:  "Normal Page",
			signal: "cf_chl_opt",
		},
		{
			name:   "ray_id_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "Ray ID: abc123</body></html>",
			title:  "Normal Page",
			signal: "ray id",
		},
		{
			name:   "turnstile_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "<div class=\"turnstile-widget\"></div></body></html>",
			title:  "Normal Page",
			signal: "turnstile",
		},
		{
			name:   "attention_required_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "Attention Required!</body></html>",
			title:  "Normal Page",
			signal: "attention required",
		},
		{
			name:   "just_a_moment_in_title",
			html:   "<html><body>" + strings.Repeat("x", 100) + "normal content</body></html>",
			title:  "Just a moment...",
			signal: "just a moment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := detector.Detect(tt.html, tt.title, 200)

			// Assert
			assert.True(t, result.Blocked, "should be blocked for signal: %s", tt.signal)
			assert.Equal(t, "cloudflare", result.Type)
			assert.Contains(t, result.Reason, tt.signal)
		})
	}
}

func TestDefaultBlockDetector_Detect_Akamai(t *testing.T) {
	detector := browser.NewBlockDetector()

	tests := []struct {
		name   string
		html   string
		title  string
		signal string
	}{
		{
			name:   "access_denied_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "Access Denied</body></html>",
			title:  "Normal Page",
			signal: "access denied",
		},
		{
			name:   "reference_hash_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "Reference #18.abc123</body></html>",
			title:  "Normal Page",
			signal: "reference #",
		},
		{
			name:   "akamai_in_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "Powered by Akamai</body></html>",
			title:  "Normal Page",
			signal: "akamai",
		},
		{
			name:   "akamai_in_title",
			html:   "<html><body>" + strings.Repeat("x", 100) + "normal content</body></html>",
			title:  "Akamai Security",
			signal: "akamai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := detector.Detect(tt.html, tt.title, 200)

			// Assert
			assert.True(t, result.Blocked, "should be blocked for signal: %s", tt.signal)
			assert.Equal(t, "akamai", result.Type)
			assert.Contains(t, result.Reason, tt.signal)
		})
	}
}

func TestDefaultBlockDetector_Detect_GenericWAF(t *testing.T) {
	detector := browser.NewBlockDetector()

	tests := []struct {
		name   string
		title  string
		signal string
	}{
		{name: "captcha", title: "Captcha Required", signal: "captcha"},
		{name: "are_you_a_robot", title: "Are you a robot?", signal: "are you a robot"},
		{name: "bot_detection", title: "Bot Detection", signal: "bot detection"},
		{name: "please_verify", title: "Please verify you are a human", signal: "please verify you are a human"},
		{name: "access_denied", title: "Access Denied", signal: "access denied"},
		{name: "blocked", title: "Blocked", signal: "blocked"},
		{name: "forbidden", title: "Forbidden", signal: "forbidden"},
		{name: "security_check", title: "Security Check", signal: "security check"},
	}

	normalHTML := "<html><body>" + strings.Repeat("x", 150) + "</body></html>"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := detector.Detect(normalHTML, tt.title, 200)

			// Assert
			assert.True(t, result.Blocked, "should be blocked for title signal: %s", tt.signal)
			assert.Contains(t, result.Reason, tt.signal)
		})
	}
}

func TestDefaultBlockDetector_Detect_ShortBody(t *testing.T) {
	detector := browser.NewBlockDetector()

	tests := []struct {
		name string
		html string
	}{
		{name: "empty_body", html: ""},
		{name: "very_short", html: "<html></html>"},
		{name: "under_100_chars", html: strings.Repeat("a", 99)},
		{name: "whitespace_only", html: strings.Repeat(" ", 200)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := detector.Detect(tt.html, "Normal Title", 200)

			// Assert
			assert.True(t, result.Blocked, "short body should be blocked")
			assert.Equal(t, "empty_response", result.Type)
		})
	}
}

func TestDefaultBlockDetector_Detect_Normal(t *testing.T) {
	detector := browser.NewBlockDetector()

	tests := []struct {
		name  string
		html  string
		title string
	}{
		{
			name:  "normal_page",
			html:  "<html><head><title>My Page</title></head><body>" + strings.Repeat("Normal content here. ", 10) + "</body></html>",
			title: "My Page",
		},
		{
			name:  "long_content",
			html:  strings.Repeat("Lorem ipsum dolor sit amet. ", 50),
			title: "Blog Post",
		},
		{
			name:  "html_with_scripts",
			html:  "<html><body>" + strings.Repeat("x", 200) + "<script>var x = 1;</script></body></html>",
			title: "App",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := detector.Detect(tt.html, tt.title, 200)

			// Assert
			assert.False(t, result.Blocked, "normal page should not be blocked")
			assert.Empty(t, result.Type)
			assert.Empty(t, result.Reason)
		})
	}
}

func TestDefaultBlockDetector_Detect_CaseInsensitive(t *testing.T) {
	detector := browser.NewBlockDetector()

	tests := []struct {
		name   string
		html   string
		title  string
		signal string
	}{
		{
			name:   "cloudflare_mixed_case_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "JUST A MOMENT...</body></html>",
			title:  "Normal",
			signal: "just a moment",
		},
		{
			name:   "akamai_upper_case_body",
			html:   "<html><body>" + strings.Repeat("x", 100) + "ACCESS DENIED</body></html>",
			title:  "Normal",
			signal: "access denied",
		},
		{
			name:   "waf_mixed_case_title",
			html:   "<html><body>" + strings.Repeat("x", 150) + "</body></html>",
			title:  "CAPTCHA Required",
			signal: "captcha",
		},
		{
			name:   "turnstile_camel_case",
			html:   "<html><body>" + strings.Repeat("x", 100) + "TurnStile Widget</body></html>",
			title:  "Normal",
			signal: "turnstile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := detector.Detect(tt.html, tt.title, 200)

			// Assert
			assert.True(t, result.Blocked, "case-insensitive detection should work for: %s", tt.signal)
			assert.Contains(t, result.Reason, tt.signal)
		})
	}
}
