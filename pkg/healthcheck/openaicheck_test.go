package healthcheck

import "testing"

// judgeOpenAIResponse 判定逻辑单测：模拟 OpenAI 真实响应形态
func TestJudgeOpenAIResponse(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		url    string
		want   openAIVerdict
	}{
		// api.openai.com：无鉴权 401 = 未封（主判据，最高优先级）
		{"api 401 = 未封", 401, `{"error":{"message":"You didn't provide an API key."}}`, "https://api.openai.com/v1/models", verdictUnlocked},
		// api 403 = 封禁（数据中心 IP / 地区不支持）
		{"api 403 = 封禁", 403, "Your access was terminated due to violation of our usage policies.", "https://api.openai.com/v1/models", verdictBlocked},
		// api 其他状态码：无法判定
		{"api 429 无法判定", 429, "Rate limit reached", "https://api.openai.com/v1/models", verdictUnknown},
		{"api 200 无法判定", 200, "ok", "https://api.openai.com/v1/models", verdictUnknown},
		{"api 5xx 无法判定", 500, "internal error", "https://api.openai.com/v1/models", verdictUnknown},

		// chat.openai.com：200 + HTML 特征 = 可访问
		{"chat 200 html = 未封", 200, "<!doctype html><html><head>ChatGPT</head></html>", "https://chat.openai.com", verdictUnlocked},
		{"chat 200 html 大写 = 未封", 200, "<HTML>hi</HTML>", "https://chat.openai.com", verdictUnlocked},
		// chat 403 = 封禁（text/plain 或 Cloudflare HTML 拦截页都不可用）
		{"chat 403 text/plain = 封禁", 403, "Your access was terminated", "https://chat.openai.com", verdictBlocked},
		{"chat 403 html = 封禁", 403, "<html><body>cf-error-details</body></html>", "https://chat.openai.com", verdictBlocked},
		// chat 200 但非 HTML（异常）＝ 无法判定
		{"chat 200 plain 无法判定", 200, "text/plain", "https://chat.openai.com", verdictUnknown},
		// chat 302 重定向 = 无法判定（由 loc 兜底）
		{"chat 302 无法判定", 302, "", "https://chat.openai.com", verdictUnknown},

		// platform.openai.com 同 chat
		{"platform 200 html = 未封", 200, "<!DOCTYPE html><html>platform</html>", "https://platform.openai.com", verdictUnlocked},
		{"platform 403 = 封禁", 403, "blocked", "https://platform.openai.com", verdictBlocked},

		// 无关 URL：无法判定（防御）
		{"其他域名无法判定", 200, "<html>google</html>", "https://www.google.com", verdictUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := judgeOpenAIResponse(c.status, []byte(c.body), c.url)
			if got != c.want {
				t.Fatalf("status=%d url=%s body=%q: got %d want %d", c.status, c.url, c.body, got, c.want)
			}
		})
	}
}
