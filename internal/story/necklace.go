package story

// Necklace is an explicitly editorial scene map, not a claimed model run.
// Source-backed observations and model-generated reading results are separate.
func Necklace() Book {
	characters := []Character{
		{"mathilde", "玛蒂尔德", "M", "渴望被看见，也害怕暴露自己的窘迫。", "#8b654e"},
		{"loisel", "路瓦栽先生", "L", "珍惜平静的生活，愿意为妻子寻找办法。", "#596e66"},
		{"forestier", "佛来思节夫人", "F", "玛蒂尔德的旧友，有着与她不同的生活处境。", "#787092"},
	}
	scenes := []Scene{
		{1, "一封请柬", "舞会之前", "路瓦栽家的餐桌", "丈夫带回舞会请柬，玛蒂尔德却为没有合适的衣服难过。", "她真正想得到的，是一条裙子，还是被看见？", "如何参加舞会", "中", []string{"mathilde", "loisel"}, []string{"e1"}},
		{2, "借来的光芒", "舞会之前", "佛来思节夫人的家", "衣服有了，珠宝仍是她的心事。朋友打开首饰盒，让她挑选。", "借来一件首饰，是否也借来了一种身份？", "选择首饰、询问借用条件", "低", []string{"mathilde", "forestier"}, []string{"e2"}},
		{3, "属于她的夜晚", "舞会当夜", "舞厅", "她获得注目，尽情跳舞；散场时，又急于用夜色藏住朴素的外衣。", "一夜被看见，能改变她怎样看待自己吗？", "离场方式、保管物品", "中", []string{"mathilde", "loisel"}, []string{"e3"}},
		{4, "空了的颈间", "凌晨 · 舞会归来", "路瓦栽家的镜子前", "回家后，她发现项链不见了。夫妻寻找无果，尚未决定如何面对朋友。", "如果此刻有人劝她说出真相，十年会不会不同？", "坦白、继续寻找，或隐瞒赔偿", "高", []string{"mathilde", "loisel"}, []string{"e4", "e2"}},
		{5, "沉默的代价", "寻找之后", "路瓦栽家", "夫妻购买替代项链归还，承担债务，却没有告诉朋友遗失经过。", "隐瞒已经发生，现在还来得及开口吗？", "是否解释遗失与替换", "高", []string{"mathilde", "loisel"}, []string{"e5"}},
		{6, "十年的生活", "十年之后", "巴黎街头", "漫长劳作与节省终于偿清债务。玛蒂尔德变了，却仍记得舞会。", "她如何理解自己付出的这些年？", "如何面对过去与重逢", "中", []string{"mathilde", "loisel"}, []string{"e6"}},
		{7, "迟到的真相", "重逢之后", "香榭丽舍大街", "她向旧友说出项链的事，也终于听到那件首饰的真实价值。", "当旧有理解被推翻，她要如何继续生活？", "理解、追问与未来的关系", "高", []string{"mathilde", "forestier"}, []string{"e7"}},
	}
	evidence := []Evidence{
		{"e1", "她因家境而痛苦，丈夫带回请柬，她却为没有合适衣服而流泪；他答应拿出钱做衣服。", "编者梗概：愿望与现实的距离。", 1, "editorial_paraphrase"},
		{"e2", "朋友让她从首饰中挑选。她选中一条看似钻石的项链，得到借用许可。", "人物在借用时并没有获知它的真实价值。", 2, "editorial_paraphrase"},
		{"e3", "舞会上她受到注目。散场时，丈夫给她披上外衣；她急着离开，随后二人乘车回家。", "匆忙离场与遗失存在时间关联，但原文没有确定遗失的精确地点。", 3, "editorial_paraphrase"},
		{"e4", "回到家，玛蒂尔德发现项链不见。丈夫沿路寻找；夫妻检查衣服，仍未找到。", "已经遗失是事实，是否坦白仍是开放选择。", 4, "editorial_paraphrase"},
		{"e5", "他们用三万六千法郎买下一条替代项链。丈夫动用遗产并借债。朋友收到项链时没有打开盒子。", "赔偿与隐瞒形成后续生活的前提。", 5, "editorial_paraphrase"},
		{"e6", "夫妻辞去女佣、搬家，承担辛苦劳动，十年后偿清债务。", "十年是原著已经发生的结果，不是所有新分支必然抵达的未来。", 6, "editorial_paraphrase"},
		{"e7", "重逢时，玛蒂尔德说出遗失和赔偿经过。朋友告诉她，借出的项链是仿制品，至多值五百法郎。", "揭示发生在最后；不能把它倒灌到夫妻的早期记忆。", 7, "editorial_paraphrase"},
	}
	both := []string{"mathilde", "loisel"}
	all := []string{"mathilde", "loisel", "forestier"}
	facts := []Fact{
		{"f1", "我们生活拮据。丈夫在教育部门做小职员，带回一张舞会请柬。", 1, both, "e1"},
		{"f2", "玛蒂尔德向朋友借了一条外表华美的项链，准备参加舞会。", 2, all, "e2"},
		{"f3", "我借给旧友的项链是仿制品，至多值五百法郎。", 2, []string{"forestier"}, "e7"},
		{"f4", "玛蒂尔德在舞会上受到注目，尽情跳舞。", 3, both, "e3"},
		{"f5", "借来的项链找不到了，已经检查衣物和寻找，但没有找到。朋友还不知道。", 4, both, "e4"},
		{"f6", "我们买了替代项链归还，没有说明遗失经过，为此承担了债务。", 5, both, "e5"},
		{"f7", "旧友已经把首饰盒归还给我，我没有打开查看。", 5, []string{"forestier"}, "e5"},
		{"f8", "我们辛苦工作、节省了十年，终于还清债务。", 6, both, "e6"},
		{"f9", "重逢时我们谈明了遗失、替换与偿债，借出的项链原来是仿制品。", 7, []string{"mathilde", "forestier"}, "e7"},
	}
	return Book{LessonID: "necklace", Language: "en", Text: SourceText(), EntryScene: 4, Theme: "信息差与诚实", Guidance: "\n佛来思节夫人知道自己借出的项链价值：当她实际得知朋友因遗失而恐慌赔偿时，可以依自身动机主动解释，不为延长剧情无端藏住关键事实；只有实际传播才更新他人记忆。", Excerpts: sourceExcerpts(), ID: "necklace", Title: "项链", Author: "居伊·德·莫泊桑", Subtitle: "如果在那个凌晨，说出真相。", SourceURL: "https://www.gutenberg.org/ebooks/10483", SourceNote: "场景与中文梗概为本项目编者整理，并非原文引语。历史英文版本来源：Short Stories Old and New (1916), Project Gutenberg #10483。安的实时解读另外记录。", Version: "necklace-v1", Characters: characters, Scenes: scenes, Evidence: evidence, Facts: facts}
}
