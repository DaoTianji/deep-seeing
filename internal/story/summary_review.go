package story

import (
	"errors"
	"fmt"
	"strings"
)

// Public verdicts, not hidden reasoning. Each field is accounted for so a
// blanket approval cannot silently omit the short summaries around the story.
type fieldReview struct {
	Field  string `json:"field"`
	OK     *bool  `json:"ok"`
	Reason string `json:"reason"`
}

type endingReview struct {
	OK     bool          `json:"ok"`
	Reason string        `json:"reason"`
	Fields []fieldReview `json:"field_checks"`
}

func (r endingReview) verdict() (bool, string, error) {
	// An explicit rejection is already sufficient to prevent publication.
	if !r.OK && strings.TrimSpace(r.Reason) != "" {
		return false, r.Reason, nil
	}
	required := map[string]bool{"resolution": false, "story": false, "new_timeline": false, "contributions": false}
	if len(r.Fields) != len(required) {
		return false, "", errors.New("结局核验未覆盖全部字段，本轮未保存")
	}
	issues := []string{}
	for _, f := range r.Fields {
		seen, known := required[f.Field]
		if !known || seen || f.OK == nil || strings.TrimSpace(f.Reason) == "" {
			return false, "", errors.New("结局核验字段缺失或重复，本轮未保存")
		}
		required[f.Field] = true
		if !*f.OK {
			issues = append(issues, fmt.Sprintf("%s: %s", f.Field, f.Reason))
		}
	}
	if len(issues) > 0 {
		return false, strings.Join(issues, "；"), nil
	}
	if !r.OK {
		return false, "结局核验未明确通过，本轮未保存", nil
	}
	return true, "", nil
}

const publicOutcomeRule = `
公开摘要同样受事实约束。告知价格或物品性质、愿意赔偿、提出免赔、对方明确免除责任、退款与债务清偿是不同事件，不能相互替代。物品便宜、是假货、说出真相或表现诚实，都不自动意味着无需赔偿或问题全部解决。只有实际有权决定的人明确作出免除决定，才可写免赔；未发生则保留未定。不能把人物误解改成叙述者确认的事实；“外形相同的替代品”也不等于“赝品替代真品”。文学比喻可以自由展开，但不能借比喻添加实际结果。读者可对照原著未来，角色当时的内心、台词及记忆不能知道尚未发生的精确年数和命运。`

const endingFieldReviewPrompt = `
不要用一个总体“忠实”替代逐字段核验。先审resolution每个结果性主张，再审story、new_timeline、contributions；正文未发生的结果不能在短摘要中成立。每个字段返回一个简短公开判定，不输出思维过程。通过时必须覆盖四个字段；任何字段不通过，总体ok=false。最终JSON格式为：{"ok":true,"reason":"短总评","field_checks":[{"field":"resolution","ok":true,"reason":"短判定"},{"field":"story","ok":true,"reason":"短判定"},{"field":"new_timeline","ok":true,"reason":"短判定"},{"field":"contributions","ok":true,"reason":"短判定"}]}。`

type continuityReview struct {
	OK          bool         `json:"ok"`
	Reason      string       `json:"reason"`
	Observation *fieldReview `json:"observation_check"`
}

func (r continuityReview) verdict() (bool, string) {
	if !r.OK {
		return false, r.Reason
	}
	if r.Observation == nil || r.Observation.Field != "observation" || r.Observation.OK == nil || strings.TrimSpace(r.Observation.Reason) == "" {
		return false, "公开变化说明尚未完成独立核对"
	}
	if !*r.Observation.OK {
		return false, r.Observation.Reason
	}
	return true, ""
}

const observationReviewPrompt = `
另外单独核对decision.observation：它是面向读者的待检总结，不是证明自己的证据。按visitor_message、actor_reply、已有事实与合法增量检查它是否改写了建议的含义、真假关系、时态或实际结果。决定去做不是已经完成，角色自认的真伪不等于物品实际真伪。
尤其分开原物、拟购替代物、人物对它们的认识：一个对象的性质不能移给另一个对象。人物以为原物是真品，不能证明她打算买赝品；建议买外形相同的物品，也没有指定替代物的材质或真假。即使观察句以“她内心倾向”开头，新增的计划内容仍必须在消息、回复或认知增量中有依据，不能以“符合人物误解”为理由补造其未表达的计划。若涉及多个物品的属性，在短reason中明确指出各属性是否有对应对象的依据。
最终JSON除ok和reason外，必须含"observation_check":{"field":"observation","ok":true或false,"reason":"短公开判定"}；摘要不准确时总体ok=false。`
