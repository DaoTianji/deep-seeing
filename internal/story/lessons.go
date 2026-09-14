package story

import (
	"net/http"
	"strings"
)

// ReadingCatalog adds essays without inventing scenes or character memories.
// Catalog remains the original, playable story collection.
func ReadingCatalog() []Book {
	text, err := libraryFiles.ReadFile("texts/beiying.txt")
	if err != nil {
		panic(err)
	}
	books := append(Catalog(), Book{ID: "beiying", Title: "背影", Author: "朱自清", Subtitle: "那一刻，与后来", Theme: "细节与回望", ReadOnly: true, Language: "zh", Version: "beiying-wikisource-2570094-v1", Text: string(text), SourceURL: "https://zh.wikisource.org/zh-hans/背影", SourceNote: "朱自清《背影》公版原文，据维基文库修订版 2570094 简体转录；去除排版空白并按自然段分隔，保留该版本措辞。不是教材注释或现代改写。", Characters: []Character{}, Scenes: []Scene{}, Evidence: []Evidence{}, Excerpts: map[string]Excerpt{}})
	books = append(books, classicalLessons()...)
	for i := range books {
		switch books[i].ID {
		case "kong", "beiying", "necklace", "taohuayuan", "quanxue", "mulan":
			books[i].LessonID = books[i].ID
		}
	}
	return books
}

type LessonView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Lens     string `json:"lens"`
	Question string `json:"question"`
}
type LessonMoment struct {
	Steps     []string `json:"steps,omitempty"`
	Title     string   `json:"title"`
	Anchor    string   `json:"-"`
	Paragraph int      `json:"paragraph"`
	Text      string   `json:"text"`
	Then      string   `json:"then"`
	Now       string   `json:"now"`
	Question  string   `json:"question"`
}
type ReadingLesson struct {
	Labels    []string          `json:"labels,omitempty"`
	Classical bool              `json:"classical,omitempty"`
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Subtitle  string            `json:"subtitle"`
	Intro     string            `json:"intro"`
	Views     []LessonView      `json:"views"`
	Moments   []LessonMoment    `json:"moments"`
	Sources   []CompanionSource `json:"sources"`
}

// Lesson is curated public teaching material, never private author-studio data
// or a claim that an AI has researched the work during this visit.
func (e *Engine) Lesson() *ReadingLesson {
	var l ReadingLesson
	switch e.Book.ID {
	case "kong":
		l = ReadingLesson{ID: "kong", Title: "同一阵笑声，\n三种处境。", Subtitle: "咸亨酒店里的笑声", Intro: "先停在柜台边。一个人被大家记住，是否意味着他被认真看见？", Views: []LessonView{
			{ID: "boy", Title: "小伙计", Lens: "从柜台里面看：什么时候可以跟着笑？", Question: "只根据这一段，你为什么会把孔乙己到店和笑声联系起来？不谈后来的事。"},
			{ID: "keeper", Title: "掌柜", Lens: "从做生意的位置看：规矩和客人，谁更重要？", Question: "只根据这一段，你怎样看待店里的客人和孔乙己？请从此刻的处境解释，不补写未来。"},
			{ID: "kong", Title: "孔乙己", Lens: "从被注视的位置看：笑声里有什么？", Question: "只根据这一段，面对众人的取笑，你最想让他们听懂什么？不要补写未发生的经历。"},
		}, Moments: []LessonMoment{{Title: "站着喝酒，穿着长衫", Anchor: "孔乙己是站着喝酒而穿长衫的唯一的人", Question: "站着与长衫放在一起，有什么张力？请区分原文和解读。"}}, Sources: []CompanionSource{
			{ID: "curated-kong-author", Title: "已整理背景 · 小说之外的鲁迅", URL: "https://www.xsg.pku.edu.cn/heros/scholar/detail/826.html", Summary: "鲁迅（1881—1936）来自浙江绍兴，也曾在北京大学讲授中国小说史，并著有《中国小说史略》。他既创作小说，也研究小说；了解这一点可以打开阅读问题，但不能据此断言他某一句话的唯一用意。"},
			{ID: "curated-kong-publication", Title: "已整理背景 · 作品的发表", URL: "https://news.pku.edu.cn/ztwz111/bnws/tsws/index9.htm", Summary: "北京大学档案馆的刊物记录将《孔乙己》列于1919年4月《新青年》第六卷第四号。发表时间不等于故事发生的时间。"},
			{ID: "curated-kong-text", Title: "原文线索 · 叙述者不是作者", URL: e.Book.SourceURL, Summary: "酒店开篇的叙述者回忆自己十二岁起当伙计的经历，并比较过去与现在的酒价。这是作品叙述者的视角，不自动等同于作者鲁迅本人的经历。"},
		}}
	case "beiying":
		l = ReadingLesson{ID: "beiying", Title: "那时不懂的，\n后来才看见。", Subtitle: "那一刻，与后来", Intro: "先不用给这篇文章一个标准答案。从一个动作、一句感叹，看看叙述中的时间怎样改变了理解。", Views: []LessonView{}, Moments: []LessonMoment{
			{Title: "父亲决定送行", Anchor: "到南京时", Then: "文中的我认为自己已经能料理旅程，多次劝父亲不必送行。", Now: "回忆把父亲的踌躇与再三嘱咐保留下来。可以留意叙述选择了哪些动作，而不替父亲编造内心独白。", Question: "这一段里父子各自如何理解送行？哪些是原文明说，哪些只是我们的解释？"},
			{Title: "两次“聪明”", Anchor: "我们过了江", Then: "当时的我想插话，暗笑父亲的迂，自认足以照顾自己。", Now: "同一段里的回望声调，让聪明不再只是称赞。叙述者正在重新评价自己的判断。", Question: "这段两次提到聪明，声调有什么不同？我也可以不完全赞同叙述者后来的判断吗？"},
			{Title: "把目光放在动作上", Anchor: "我说道", Then: "文中连续写探身、攀、缩和微倾，把一个日常动作展开。", Now: "泪水先出现在现场，随后又在回忆中被写下；不要把全部情绪都推迟到成年之后。", Question: "请从这段的具体动词解释表达效果，不只用一句父爱概括，也不要要求读者必须感动。"},
		}, Sources: []CompanionSource{
			{ID: "curated-beiying-text", Title: "已整理背景 · 原文与写作时间", URL: e.Book.SourceURL, Summary: "本篇署写作时间为1925年10月在北京。原文记录的是回望，不能把写作时间直接当成文中送行当天，也不能把本篇与1928年同名散文集混为一谈。"},
			{ID: "curated-beiying-author", Title: "已整理背景 · 写作之外的朱自清", URL: "https://www.tsinghua.edu.cn/info/1365/81248.htm", Summary: "朱自清（1898—1948）曾在江浙学校任国文教员，1925年秋到清华任教授，1932年9月起长期担任中文系主任。他不仅是作者，也是教师与学者；这些经历不能代替原文证明某个具体写作动机。"},
		}}
	default:
		extra := extraLesson(e.Book)
		if extra == nil {
			return nil
		}
		l = *extra
	}
	for i := range l.Moments {
		for _, p := range e.Paragraphs() {
			if strings.Contains(p.Text, l.Moments[i].Anchor) {
				l.Moments[i].Paragraph = p.ID
				l.Moments[i].Text = p.Text
				break
			}
		}
	}
	return &l
}

func (e *Engine) registerLesson(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/story/lesson", func(w http.ResponseWriter, r *http.Request) {
		l := e.Lesson()
		if l == nil {
			jsonResponse(w, 404, map[string]string{"error": "本篇暂未设置专题"})
			return
		}
		jsonResponse(w, 200, l)
	})
}
