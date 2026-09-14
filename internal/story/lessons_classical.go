package story

import (
	"fmt"
	"strings"
)

func classicalLessons() []Book {
	specs := []Book{
		{ID: "taohuayuan", Title: "桃花源记", Author: "陶渊明", Subtitle: "进去之后，是否还想回来", Theme: "路线与留白", SourceURL: "https://zh.wikisource.org/zh-hans/桃花源記", SourceNote: "《桃花源记》散文部分全文，简体整理；不含另篇《桃花源诗》。据维基文库修订版 2620894，省去校勘旁注，规范字形与标点；不是现实地理记录。"},
		{ID: "quanxue", Title: "劝学（节选）", Author: "荀子", Subtitle: "一句话，怎样成为一个论证", Theme: "比喻与论证", ReadOnly: true, TextScope: "课文节选", SourceURL: "https://www.gushiwen.cn/shiwenv_9b5ed8061abe.aspx", SourceNote: "完整展示所选的四段课文：君子曰、青取之于蓝、吾尝终日而思、积土成山至用心躁也。不是《荀子·劝学篇》全篇；各选段间原篇另有文字。仅录公版古文，不复制现代译注；保留知、生及𫐓等古字。"},
		{ID: "mulan", Title: "木兰诗", Author: "佚名 · 北朝乐府", Subtitle: "诗里写下的，与诗里省略的", Theme: "详略与选择", SourceURL: "https://www.gushiwen.cn/shiwenv_2d6b0c83a500.aspx", SourceNote: "《木兰诗》简体通行文本全文，七个阅读段落。仅录公版古诗，不复制现代译注。作者佚名，郭茂倩是《乐府诗集》的编者而非本诗作者；不同传本文字有异。"},
	}
	for i := range specs {
		b := &specs[i]
		raw, err := libraryFiles.ReadFile("texts/" + b.ID + ".txt")
		if err != nil {
			panic(err)
		}
		b.Text = string(raw)
		b.Language = "zh"
		b.Version = b.ID + "-classical-v1"
		b.Characters = []Character{}
		b.Scenes = []Scene{}
		b.Evidence = []Evidence{}
		b.Facts = []Fact{}
		b.Excerpts = map[string]Excerpt{}
		b.Guidance = "本篇古文/古诗属于作品文本。用简体中文回答。区分原文、现代释义、有限解读与模拟，不将后世改编当成原文，不伪造作者引语。"
		if b.ID == "taohuayuan" {
			b.Characters = []Character{{ID: "fisher", Name: "渔人", Initial: "渔", Description: "以捕鱼为业，正在经历一次陌生的见闻。", Color: "#467467"}, {ID: "villager", Name: "桃源村民", Initial: "村", Description: "桃源居民的文本视角演绎，不替原文补定姓名。", Color: "#807a55"}}
			addLessonScenes(b, []int{1, 2, 3, 4, 5},
				[]string{"缘溪而行", "豁然开朗", "问讯与辞行", "留下的标记", "故事的余音"},
				[]string{"渔人沿溪捕鱼，遇见桃花林，想继续前行。", "渔人穿过山口，看见田园、屋舍与居民。", "村民款待渔人，讲述先世避乱，与外界隔绝；渔人辞行，村民请他不要告诉外人。", "渔人离开后作标记并告知太守；再次寻路却未能找到。", "刘子骥想前往，未能成行便病终；后来无人再问路。"},
				[][]string{{"fisher"}, {"fisher"}, {"fisher", "villager"}, {"fisher"}, {"fisher"}},
				[]string{"继续前行，还是回去", "如何面对陌生世界", "离开、留下，或如何讲述见闻", "如何看待无法重走的路", "如何理解没有回答的结尾"})
			b.Guidance += "桃源是文学空间，不能宣称真实经纬度、仙术或确定历史。村民只在问讯段以后可交流。原文未写村民如何得知太守寻访，禁止将此赋予村民；前往者无权知道未来迷路。模拟路线不是原著留白的答案。"
		}
		if b.ID == "mulan" {
			b.Characters = []Character{{ID: "mulan", Name: "木兰", Initial: "兰", Description: "诗歌中的木兰；只知道当前诗段已经发生的经历。", Color: "#8d5263"}}
			addLessonScenes(b, []int{2, 3, 4, 5, 6, 7},
				[]string{"军帖与选择", "辞家远行", "被压缩的岁月", "归来与所愿", "回到家门", "双兔的比喻"},
				[]string{"父亲名列军书，家中无长兄，木兰愿代父从征。", "木兰备办行装离家，经过黄河和黑山。", "诗歌简写行军、寒夜、战死与归来，没有具体战役记载。", "木兰受到赏赐，不愿做尚书郎，希望回故乡。", "家人迎接木兰；她换回旧时衣裳，伙伴惊讶于她的身份。", "叙述以双兔同行的比喻收尾；这不是额外发生的一场战争。"},
				[][]string{{"mulan"}, {"mulan"}, {"mulan"}, {"mulan"}, {"mulan"}, {"mulan"}},
				[]string{"如何面对家中困境", "如何度过离家的不安", "如何理解没有写出的岁月", "留下任职，还是回乡", "如何向伙伴解释自己的选择", "如何回应对身份的判断"})
			b.Guidance += "作者佚名。不得引入影视中的龙、爱情对象、反派或具体战役。数字、军功与漫长岁月可讨论诗歌表达，不擅自编定确切生卒年或年表。诗中隐去的内容只能作为明确的模拟，不回填史实。"
		}
	}
	return specs
}

// Scene anchors come from complete original paragraphs. Nothing is model-generated.
func addLessonScenes(b *Book, paragraphs []int, titles, summaries []string, cast [][]string, choices []string) {
	ps := (&Engine{Book: *b}).Paragraphs()
	b.EntryScene = 1
	for i, p := range paragraphs {
		src := ps[p-1]
		id := fmt.Sprintf("e%d", i+1)
		b.Excerpts[id] = Excerpt{Text: src.Text, StartByte: src.Start, EndByte: src.End}
		b.Evidence = append(b.Evidence, Evidence{ID: id, Text: summaries[i], Note: "编者梗概；点开核对原文", Origin: "original", Scene: i + 1})
		b.Scenes = append(b.Scenes, Scene{ID: i + 1, Title: titles[i], Time: fmt.Sprintf("原文第 %d 段", p), Place: "作品中的场景", Summary: summaries[i], Question: choices[i] + "？", Choice: choices[i], Resistance: "中", Characters: cast[i], EvidenceIDs: []string{id}})
		b.Facts = append(b.Facts, Fact{ID: fmt.Sprintf("f%d", i+1), Text: summaries[i], Since: i + 1, Audience: cast[i], EvidenceID: id})
	}
}

func extraLesson(b Book) *ReadingLesson {
	l := &ReadingLesson{ID: b.ID, Subtitle: b.Subtitle, Views: []LessonView{}, Moments: []LessonMoment{}, Sources: []CompanionSource{}, Labels: []string{"文意线索", "值得追问"}}
	switch b.ID {
	case "necklace":
		l.Subtitle = "说出真相之前"
		l.Labels = []string{"人物所知", "另一种看法"}
		l.Moments = []LessonMoment{
			{Title: "一张请柬", Anchor: "He stopped, stupefied", Then: "同一张请柬，对带它回来的人与接到它的人意义可能不同。只从当前反应出发。", Now: "把难过概括为虚荣，会不会过早关闭对处境的理解？", Question: "请从这段请柬引发的反应比较夫妻的期待，不透露后续情节。"},
			{Title: "借来的首饰", Anchor: "All at once she discovered", Then: "这一刻可以观察她怎样看待首饰，但不能据此补出她不知道的属性。", Now: "借用一件东西，是否也借来了对身份的想象？这是阅读问题，不是作者的定论。", Question: "这一段如何表现人物对体面的期待？只谈当前已知信息，不泄露结尾。"},
			{Title: "发现遗失", Anchor: "They looked in the folds", Then: "寻找发生在不确定中。建议、计划与已经完成的事应当分开。", Now: "决定赔偿之前，是否还有一种沟通的选择？不预先保证它必然成功。", Question: "只在这个遗失时点比较坦白与隐瞒的风险，不透露后续结局。"},
		}
		l.Sources = []CompanionSource{
			{ID: "curated-necklace-author", Title: "已整理背景 · 莫泊桑与本篇", URL: b.SourceURL, Summary: "本篇署名 Guy de Maupassant。当前保留1916年英文选集中的完整历史译文；英文叙述的措辞不应直接称为作者法语原句。"},
			{ID: "curated-necklace-edition", Title: "已整理背景 · 读的是哪个版本", URL: b.SourceURL, Summary: "原文区沿用 Project Gutenberg 第10483号《Short Stories Old and New》收录文本。中文逐句译文使用已有翻译功能，标注AI辅助，不把现代译文冒充历史原著。"},
		}
	case "taohuayuan":
		l.Classical = true
		l.Subtitle = "沿着原文，走一趟桃源"
		l.Moments = []LessonMoment{
			{Title: "进入：由狭到阔", Anchor: "林尽水源", Then: "先写狭窄的入口，再展开田园与居民。“交通”在这里指交错相通，不是现代交通运输。", Now: "安宁来自具体生活描写；桃源到底在哪里，不是这段文字能够证明的事。", Steps: []string{"山口：初极狭", "行进：复行数十步", "所见：豁然开朗"}, Question: "由狭到阔的空间变化怎样影响阅读感受？请用这段原文解释。"},
			{Title: "停留：双方知道什么", Anchor: "见渔人", Then: "“妻子”指妻子与儿女；“绝境”指与外界隔绝的地方；“无论”在这里是不必说、更不必说。", Now: "盛情款待与不愿对外讲述能否同时成立？不替村民编定一种唯一动机。", Question: "渔人和村民的信息差是什么？为什么既招待又请求保密？请区分事实与推测。"},
			{Title: "离开：标记与迷路", Anchor: "既出", Then: "“志”在此作动词，是做标记；原文写了做标记，也写了后来无法再找到路。", Now: "留下标记并没有消除不确定。我们可以讨论叙事效果，不必选定仙术或阴谋作解释。", Question: "做了标记却再次迷路，给这个故事留下了怎样的空间？不要虚构真实地址。"},
		}
		l.Views = []LessonView{{ID: "fisher", Title: "渔人", Lens: "从刚刚看见的景象与当下所知出发。", Question: "此刻你看见了什么，又有哪些事情还不知道？不要使用后续情节。"}}
		l.Sources = []CompanionSource{
			{ID: "curated-tao-author", Title: "已整理背景 · 陶渊明与诗、记", URL: b.SourceURL, Summary: "这篇散文署陶渊明，常与《桃花源诗》并列。此处展示“记”的全文，不把另篇诗句悄悄加入渔人的见闻。"},
			{ID: "curated-tao-time", Title: "原文线索 · 两条时间", URL: b.SourceURL, Summary: "开篇的晋太元是叙事时间；村民讲先世避秦时乱，是人物对自身来历的叙述。文学作品中的这两条时间线，不等于已经得到考证的桃源历史。"},
		}
	case "quanxue":
		l.Classical = true
		l.Subtitle = "比喻怎样走向结论"
		l.Labels = []string{"论证拆解", "提出反例"}
		l.Moments = []LessonMoment{
			{Title: "改变：青与蓝", Anchor: "青，取之于蓝", Then: "“中绳”是合乎墨线；“𫐓”指使木弯曲；“知”在这里可按智理解。诸喻共同讨论学习与改变。", Now: "材料会被加工改变，并不自动证明所有学习方式都有效；可以追问比喻与结论之间的条件。", Steps: []string{"比喻：青于蓝、冰寒于水", "关系：经过过程而改变原来状态", "结论：博学与自省帮助改善判断、行动"}, Question: "请检查青与蓝的比喻怎样支持论点，并给出一个能帮助明确适用条件的反例。"},
			{Title: "借助：登高与舟楫", Anchor: "吾尝终日", Then: "“假”是借助；“绝”是横渡；“生”通性。强调借助条件，而不是单靠本领自然增长。", Now: "使用工具何时促进理解，何时可能替代思考？把现代例子标为我们的类比。", Steps: []string{"比喻：登高、舆马、舟楫", "不变：臂长、脚力、游泳能力", "变化：借助条件扩大行动范围"}, Question: "用现代学习工具类比善假于物，有哪些相似和不同？别把类比说成荀子的原话。"},
			{Title: "积累、坚持与专一", Anchor: "积土成山", Then: "这一段不只说时间久，还用积累、不舍、用心一组织不同条件。“跬步”是小步，不是具体课程指标。", Now: "重复错误是否也是积累？可以补充反馈与方法这个条件，不能把质疑直接判为不努力。", Steps: []string{"积累：小流与江海", "坚持：锲而不舍", "专一：用心一与用心躁"}, Question: "这段里的积累、坚持、专一是否足以保证学好？请先忠实解释原文，再讨论我的质疑。"},
		}
		l.Sources = []CompanionSource{
			{ID: "curated-xun-author", Title: "已整理背景 · 荀子与篇章", URL: "https://zh.wikisource.org/zh-hans/荀子/勸學篇", Summary: "《劝学》是《荀子》的首篇。课文所选的比喻只是全篇的一部分；全篇还讨论学习、修养与礼，不能把四段摘录当成荀子思想的全部。"},
			{ID: "curated-xun-scope", Title: "已整理背景 · 选段范围", URL: b.SourceURL, Summary: "本页是四段完整选文，末句为“用心躁也”。不同古籍本与教学选本在字形、通假和断句上有差异；注释与现代例子是辅助理解，不属于原文。"},
		}
	case "mulan":
		l.Classical = true
		l.Subtitle = "诗的篇幅，不等于人生的时长"
		l.Labels = []string{"诗中写下", "诗中留白"}
		l.Moments = []LessonMoment{
			{Title: "出征前：迟疑与决定", Anchor: "问女何所思", Then: "诗从问答写到军帖、家中处境与替父从征的决定。“市”作动词，是买。", Now: "原文没有给出一份完整心理报告。可以演绎当下犹疑，但不把演绎说成历史记载。", Question: "诗如何从叹息推进到替父从征的决定？哪些情绪是原文明说，哪些是读者补充？"},
			{Title: "行军与归来：漫长却简短", Anchor: "万里赴戎机", Then: "六句容纳远行、寒夜、战事与归来；“将军百战死，壮士十年归”可以结合互文理解，不机械地一一分配命运。", Now: "诗没有写出的战役、同伴关系和私人体验保持留白。不能从影视改编补进来当成事实。", Steps: []string{"写下：远行与寒夜", "压缩：百战与十年", "留下：未展开的战事与体验"}, Question: "为什么漫长战争在这里写得很短？请提出两种有原文依据的理解，不替诗补造战役。"},
			{Title: "所愿：官职与故乡", Anchor: "归来见天子", Then: "诗先写功赏，再写木兰辞官还乡的愿望。读者可以关注从公共评价到个人所愿的转折。", Now: "原文没有说明拒绝官职的全部理由；可讨论多种可能，不把其中一种定为真实动机。", Question: "功赏与回乡在这一段如何形成对照？不要把一种心理解释当成唯一答案。"},
			{Title: "回家：动作重新变密", Anchor: "爷娘闻女来", Then: "迎接、开门、换衣、理鬓与见伙伴被展开。与战事的压缩并读，观察叙事速度变化。", Now: "这里的生活细节值得细读，但不等于对所有人的身份或归宿作规定。", Question: "回家的动作为什么写得这么具体？比较之前写战事的篇幅，允许不同理解。"},
		}
		l.Views = []LessonView{{ID: "mulan", Title: "木兰", Lens: "从当前诗段的处境说话，不带入影视设定。", Question: "只在此刻，你面临什么选择，哪些事情仍不确定？不谈后面的经历。"}}
		l.Sources = []CompanionSource{
			{ID: "curated-mulan-source", Title: "已整理背景 · 佚名与编者", URL: "https://zh.wikisource.org/zh-hans/木蘭詩", Summary: "本诗以佚名作品流传，收录于《乐府诗集》。编者不等于创作者；这里不虚构作者姓名、头像或个人传记。"},
			{ID: "curated-mulan-versions", Title: "已整理背景 · 原诗与后来改编", URL: b.SourceURL, Summary: "本页采用简体通行文本；不同传本有唯/惟、火伴/伙伴等异文。后世影视和小说属于另外的改编，不是这首诗中已发生的事。"},
		}
	default:
		return nil
	}
	l.Title = l.Subtitle
	// Ensure unsupported material cannot silently become an empty source anchor.
	for _, m := range l.Moments {
		if !strings.Contains(b.Text, m.Anchor) {
			// A different edition must not receive this edition's teaching material.
			return nil
		}
	}
	return l
}
