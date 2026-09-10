package modelopts

import (
	"net/url"
	"strings"
)

// Extra enables reasoning on the verified DeepSeek provider. The current Eino
// adapter round-trips reasoning_content across tool calls; it is not published
// in our public trace. Do not apply provider-specific options to other gateways.
func Extra(base, model string) map[string]any {
	u, err := url.Parse(base)
	if err == nil && strings.EqualFold(u.Hostname(), "api.siliconflow.cn") && strings.HasPrefix(model, "deepseek-ai/DeepSeek-V4-") {
		return map[string]any{"enable_thinking": true, "reasoning_effort": "high"}
	}
	return nil
}
