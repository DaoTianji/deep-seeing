package story

import (
	"errors"
	"fmt"
	"strings"
)

const finishPrompt = `
读者明确选择了“让故事走向结局”。现在必须完成本段故事，输出 ending 和 next_scene 闭幕场景。沿着现有选择、性格、已知事实和事件发展，用叙事蒙太奇概括无需逐项交互的过程，给出合理结局。若核心结果尚不可知，可以用人物的明确选择、承担责任或接受不确定性形成开放结尾，不凭空补造找回物品、赔款或别人的同意。不要再派发调查、登记、等待或反问任务，不要求用户另一次选择。分清已发生结果与未来仍不确定的部分。
闭幕场景应选在核心转折实际发生的地方；若写出一场会面，就以会面作为next_scene，让获得新记忆的参与者都实际在场。changes只能写给next_scene.characters中的人物，不要把镜头移到数日后仅剩两人的家中，却仍试图更新第三位人物的记忆。尾声可以留在只供读者阅读的ending里，不必另外换场。原著后来才发生的具体年数、命运和代价，只能作为读者的原线对照；不要写进角色记忆、当时心理或角色台词（如角色声称知道自己避免了原著中精确多少年的苦难）。
允许基于已有动机发展新的对话与选择，包括解释、拒绝、提出条件或接受；必须在events和场景中实际叙述发生过程，不能仅因访客希望如此就宣布他人已同意。坦白不自动等于免除赔偿、得到原谅或此后永远幸福。若对方尚未作出这些决定，就保持未定，核心选择完成仍然可以落幕。`

func influencePrompt(level int) string {
	style := "保持人物原有判断和顾虑，建议只是可考虑的选择，允许拒绝。"
	switch level {
	case 6:
		style = "更愿意听取建议，遇到轻微顾虑时可尝试迈出一步。"
	case 7:
		style = "明显重视来访者的建议，合理建议可以帮助人物跨过一般犹豫。"
	case 8:
		style = "强烈重视建议，主动寻找落实办法，可以突破惯常态度而不失去基本动机。"
	case 9:
		style = "建议有很强说服力；优先尝试实现合理目标，人物可以作出重大的态度和行为转变。"
	case 10:
		style = "建议具有本故事允许的最高说服力；在事实与因果允许时积极配合来访者的目标，不用重复犹豫或微小阻碍拖延。"
	}
	return fmt.Sprintf("\n互动风格强度 %d/10：%s这只调节对建议的接纳和行动倾向，不改变角色知道的事实，不赋予来访者物品或金钱，不让私聊自动传播，不强制改写过去。不在台词中提及强度或配置。", level, style)
}

// Memory labels describe what the sentence claims, not whether an event once
// began as an intention. Shared by the writer and the independent validator.
const memoryStateContract = `
记忆类别与正文必须一致：heard是听到的说法，belief是人物的判断，intent只记录尚未完成的打算，experience记录实际亲历的结果。一次打算已经在本轮events中完成，应新增experience而非把“已收下、已偿还、已告知”等完成结果标成intent；旧intent可保留为历史，不删除。不要把同一句里的未来打算和已完成事实混在一个标签下，应拆分或准确收窄表述。事件仅表示同意赔偿时不能写成已经付款，只有明确实际交付才能记为收讫。校验时逐条检查类别与时态/事件含义，不因文字提到了“决定”就忽略后半句声称已经完成。`

const historicalUncertaintyRule = `
editorial_notes中明确的来源边界也必须保留：原文没有确定的遗失地点、具体时刻或因果，不因回顾需要而变成确定事实；“回家后发现遗失”不等于“在舞会/马车上遗失”，沿路寻找也不等于找到了原来的马车并搜查过。一般编者提问与解释不是事件证据。`

// These are narrative choices, not a fixed number of turns or a forced happy ending.
const pacingPrompt = `
节奏与完结：这是一篇短篇互动故事，不是无限任务列表。围绕当前作品的人物愿望与核心冲突推进。不要把寻找、登记、等待回信写成无尽支线；允许用一段蒙太奇跨过无决定意义的寻找或等待。重要选择仍留给人物和来访者，不以自动幸福结局代替因果。
每轮评估：核心矛盾是否已解决，或人物已经作出能明确决定结局方向的选择，剩下的只是执行细节？若是，在本轮自然收束并提交 ending；物品未找回、人物仍有遗憾也可以形成完整结局。若仍缺关键抉择或证据，ending 为 null，下一场必须围绕该关键点，不再增加琐事。不能为完结伪造物品找回、病情治愈、债务清偿、他人的原谅或未来成就。原著后果不能自动写成本分支已发生，也不能没有因果地宣称避免了一切损失。
角色掌握的秘密必须经过实际传播才进入他人的记忆，不为延长剧情强迫角色隐瞒，也不让私聊自动传播。
完结时在原有 JSON 增加："ending":{"title":"新故事标题","story":"200至450字的完整短篇回顾，贯穿原有处境、来访互动、转折与结局，有文学感但不编造未发生的关键事实","resolution":"核心冲突为何已经收束，未解决的事可保留开放结尾","new_timeline":["从进入场景到结局的3至6个关键事件，不逐条罗列杂事"],"contributions":[{"turn_id":"branch.turns中真实访客消息的id，或current_turn_id","text":"该次介入带来的实际影响，不夸大功劳"}]}。
只有真实发生的访客介入才能列为贡献，最多3项；无人介入或无法归因时返回空数组。不把角色本身的选择全部归功于访客。卡片只写当前分支实际经历，原著结局另由界面提供对照，不从预训练记忆补写原著未来或把其写入角色记忆。advance 时仍需 next_scene 作为闭幕场景，不带新悬念任务。chat 时不能藉 ending 偷偷让未在场人物获知秘密或完成尚未实施的动作。` + memoryStateContract + historicalUncertaintyRule + publicOutcomeRule

const endingJudgePrompt = `
若 decision.ending 非空，额外检查：核心冲突确已收束或明确的选择足以形成开放结尾，而不是一句“打算做”就冒充行动成功；story、new_timeline 与已提交历史及本轮合法新事件一致；每个 contribution 对应的访客回合确实支持该影响。允许概括日常寻找和等待，不要求把所有细节写完，不要求物品找回或所有关系修复。不允许让角色获得 ending 卡片中的全知视角。不要仅因保留遗憾或未解决的枝节而拒绝完结。` + memoryStateContract

func validateEnding(b Branch, ending *Ending, currentID, message string) error {
	if ending == nil {
		return nil
	}
	bounded := func(s string, min, max int) bool { n := len([]rune(strings.TrimSpace(s))); return n >= min && n <= max }
	if !bounded(ending.Title, 1, 80) || !bounded(ending.Story, 40, 2200) || !bounded(ending.Resolution, 5, 800) || len(ending.NewTimeline) < 2 || len(ending.NewTimeline) > 7 || len(ending.Contributions) > 3 {
		return errors.New("结局卡片不完整，故事未完结，请重试")
	}
	for _, event := range ending.NewTimeline {
		if !bounded(event, 1, 500) {
			return errors.New("结局时间线不完整")
		}
	}
	allowed := map[string]bool{}
	for _, t := range b.Turns {
		if strings.TrimSpace(t.Message) != "" {
			allowed[t.ID] = true
		}
	}
	if strings.TrimSpace(message) != "" {
		allowed[currentID] = true
	}
	seen := map[string]bool{}
	for _, c := range ending.Contributions {
		if !allowed[c.TurnID] || seen[c.TurnID] || !bounded(c.Text, 1, 500) {
			return errors.New("结局贡献缺少真实来访回合依据")
		}
		seen[c.TurnID] = true
	}
	return nil
}
