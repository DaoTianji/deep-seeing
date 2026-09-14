import {ArrowRight,BookOpen,MessageCircle,GitBranch,Quote} from "lucide-react";
import {kongRewriteLink,kongRewrites} from "./reading-entry";
import "./reading-welcome.css";

export function ReadingWelcome(){
 return <main className="welcome-room">
  <header className="welcome-nav"><a className="welcome-brand" href="/reading"><BookOpen size={24}/>书中见</a><a href="/reading?view=shelf">书架与我的记录 <ArrowRight size={16}/></a></header>
  <section className="welcome-hero">
   <div><p className="welcome-eyebrow">书中见 · AI 互动阅读</p><h1>读懂故事，<br/><em>也能改写故事。</em></h1><p className="welcome-lead">向人物提问，请安讲解原文和背景。<br/>也可以进入故事，用你的行动试出另一个结局。</p><div className="welcome-actions"><a className="welcome-primary" href="#companion">体验伴读 <ArrowRight size={18}/></a><a className="welcome-secondary" href="#rewrite">试试改写</a></div><p className="welcome-hint">先用《孔乙己》试一试。开始体验时输入笔名，保存自己的记录。</p></div>
   <aside className="welcome-excerpt"><span>鲁迅 ·《孔乙己》原文节选</span><Quote size={30} aria-hidden="true"/><blockquote>孔乙己是站着喝酒而穿长衫的唯一的人。</blockquote><div className="welcome-contrast"><span>站着喝酒</span><span>穿着长衫</span></div><p>短衣帮站着喝酒，穿长衫的坐进里间。<br/>孔乙己为什么偏偏不属于任何一边？</p><small>从这段原文开始，不提前揭示结局。</small></aside>
  </section>
  <section id="companion" className="welcome-experience">
   <div className="welcome-section-head"><span><MessageCircle size={20}/>01 · 伴读</span><h2>他为什么不肯脱下长衫？</h2><p>带着一个问题回到原文。你可以听人物如何回应，也可以请安解释文本和背景。</p></div>
   <div className="welcome-questions">
    <a href="/reading?book=kong&question=coat"><span className="welcome-person">孔</span><small>问孔乙己 · 人物视角</small><h3>“你为什么还穿着这件长衫？”</h3><p>从他当时的处境展开对话，而不是从结局倒推。</p><strong>与孔乙己交谈 <ArrowRight size={16}/></strong></a>
    <a href="/reading?book=kong&question=background"><span className="welcome-person an">安</span><small>问安 · 原文与背景</small><h3>长衫和短衣，意味着什么？</h3><p>解释文中的对比；背景资料与原文依据分别标明。</p><strong>请安解释 <ArrowRight size={16}/></strong></a>
    <a href="/reading?book=kong&question=laughter"><span className="welcome-person text"><Quote size={22}/></span><small>讨论文本 · 看客与叙述</small><h3>为什么大家都在笑？</h3><p>看看同一阵笑声，对不同的人意味着什么。</p><strong>从笑声读起 <ArrowRight size={16}/></strong></a>
   </div><p className="welcome-boundary">伴读不改变原著。人物对话由 AI 模拟；安的解读与原文分开展示。联网查询状态可在伴读页查看。</p>
  </section>
  <section id="rewrite" className="welcome-rewrite">
   <div><span className="welcome-eyebrow"><GitBranch size={20}/>02 · 改写</span><h2>孔乙己的本事，<br/>能不能换条出路？</h2><p>他识字，写得一手好字，却在酒店里受人取笑。<br/>你走进咸亨酒店，打算做一件原作里没有发生的事。</p><div className="welcome-rewrite-ideas">{Object.entries(kongRewrites).map(([key,idea],i)=><a key={key} href={`${kongRewriteLink}&idea=${key}`}><span aria-hidden="true">0{i+1}</span><div><h3>{idea.title}</h3><p>{idea.description}</p></div><ArrowRight size={18} aria-hidden="true"/></a>)}</div><a className="welcome-secondary" href="/reading?book=kong&mode=story">我有别的想法，自己选个起点 <ArrowRight size={18}/></a><small>这些是开场提议，不是预设剧情。点击后可以修改开场白，再发送给人物。</small></div>
   <div className="welcome-outcome"><span>改写怎么玩</span><h3>你出主意，人物作出回应</h3><p>孔乙己可能答应，也可能嫌你的办法有失体面。沿着他的回应继续行动，看看故事会变成什么样。</p><ol><li><strong>进入一个场景</strong><p>人物只知道发展到此刻的经历。</p></li><li><strong>试着改变一件事</strong><p>提出建议、与人物交谈，再推进故事。</p></li><li><strong>收尾，保存结局卡</strong><p>可以手动结束，让故事按当前走向收尾。对照原著与新故事，并署上你的名字。</p></li></ol><p className="welcome-boundary">新故事独立保存，不覆盖原著或旧存档。这是玩法说明，不是生成示例；结局卡会涉及原著后续情节。</p></div>
  </section>
  <section className="welcome-library"><div><h2>想从另一篇开始？</h2><p>书架里还有《项链》《桃花源记》《背影》等作品。支持的阅读与互动方式因作品而异。</p></div><a className="welcome-secondary" href="/reading?view=shelf">浏览其他作品与我的记录 <ArrowRight size={18}/></a></section>
  <footer className="welcome-footer">书中见 · 互动阅读体验版<span>原文、解读与虚构分支，始终分开。</span></footer>
 </main>;
}
