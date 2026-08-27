package runtime

import (
	"fmt"
	"strings"

	"deep-seeing/internal/attention"
)

func formatAttentionSnapshot(snapshot attention.Snapshot) string {
	var b strings.Builder
	b.WriteString("这是当前会话的临时注意工作区，只表示此前选择保留的线索，不是正文、事实证明或长期记忆。")
	fmt.Fprintf(&b, "\n- capacity: center=%d support=%d periphery=%d", snapshot.Capacity.Center, snapshot.Capacity.Support, snapshot.Capacity.Periphery)
	if len(snapshot.Items) == 0 {
		b.WriteString("\n- items: empty")
	} else {
		for _, tier := range []attention.Tier{attention.Center, attention.Support, attention.Periphery} {
			var values []string
			for _, item := range snapshot.Items {
				if item.Tier != tier {
					continue
				}
				values = append(values, fmt.Sprintf("%s:%s role=%s idle_turns=%d", item.Source, item.ID, item.Role, item.IdleTurns))
			}
			if len(values) > 0 {
				fmt.Fprintf(&b, "\n- %s: %s", tier, strings.Join(values, "; "))
			}
		}
	}
	b.WriteString("\nID 只是连续性线索；需要依赖内容时仍须调用对应 read 工具，并遵守该来源的证据角色。")
	return b.String()
}
