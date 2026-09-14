import {useState} from "react";
import {BookOpen, ArrowUpRight, Feather, GitBranch} from "lucide-react";
import type {BookSummary} from "./reading-api";

const symbols:Record<string,string>={necklace:"◈",magi:"赠",leaf:"叶",paw:"愿",kong:"人",beiying:"影",taohuayuan:"源",quanxue:"学",mulan:"兰"};
const readingTopics:Record<string,string>={necklace:"人物的选择与结局中的反转",magi:"礼物、选择与两个人的心意",leaf:"人物的希望与故事中的伏笔",paw:"愿望、代价与悬念",kong:"人物处境与看客的态度",beiying:"父子关系与回忆中的细节",taohuayuan:"理想社会的描写与叙事中的悬念",quanxue:"比喻、论据与学习主张",mulan:"叙事节奏与人物形象"};
export function ReadingShelf({books,error}:{books:BookSummary[];error:string}){
 const [filter,setFilter]=useState("all");
 const records=books.flatMap(book=>(book.records||[]).map(record=>({...record,book}))).sort((a,b)=>b.updated_at.localeCompare(a.updated_at));
 const visible=records.filter(r=>filter==="all"||r.book.id===filter);
 return <main className="rs-room">
  <header className="rs-header"><span><BookOpen size={23}/>书中见</span><span className="rs-edition">互动阅读体验版</span></header>
  <section className="rs-intro"><h1>选择一篇开始阅读</h1><p>阅读原文，向安提问，或从人物的视角讨论故事。</p><div className="rs-stats"><span><b>{books.length}</b> 篇作品</span><span><b>{books.filter(b=>b.reader?.finished).length}</b> 篇已读完</span><span><b>{records.filter(r=>r.completed).length}</b> 张结局卡</span></div></section>
  {error&&<p className="rs-error" role="alert">{error}<button onClick={()=>window.location.reload()}>重新连接</button></p>}
  {!books.length&&!error&&<p role="status">正在加载书架…</p>}
  {(books.some(b=>b.id==="taohuayuan")||books.some(b=>b.id==="necklace"))&&<section className="rs-featured" aria-label="推荐体验">
   {books.some(b=>b.id==="taohuayuan")&&<a className="rs-experience companion" href="/reading?book=taohuayuan"><span>01 · 阅读时</span><h2>读懂《桃花源记》</h2><p>读原文、问背景，或听听渔人如何理解自己的经历。讨论不会改变原著。</p><strong>开始伴读 <ArrowUpRight size={18}/></strong></a>}
   {books.some(b=>b.id==="necklace")&&<div className="rs-experience story"><span>02 · 读完后</span><h2>试着改变《项链》</h2><p>如果遗失后选择坦白，会发生什么？与人物交谈，再把新结局保存为一张署名卡。</p><a href="/reading?book=necklace&mode=story"><strong>选择故事起点 <ArrowUpRight size={18}/></strong></a><a className="rs-example-link" href="/reading?view=example">先看完整示例（含结局）</a></div>}
  </section>}
  <section className="rs-lessons" aria-label="课文精读专题">{books.filter(b=>b.lesson_id||b.id==="kong"||b.id==="beiying").map(b=><a className="rs-lesson" key={b.id} href={`/reading?book=${b.id}`}><span>课文精读 · {b.author}</span><strong>《{b.title}》</strong><p>{readingTopics[b.id]||b.subtitle}</p><small>{b.text_scope||"完整原文"} · 作者与背景 · 你的理解　↗</small></a>)}</section>
  <section className="rs-books" aria-label="短篇书架">{books.map((book,index)=>{
   const finished=(book.records||[]).filter(r=>r.completed).length;
   return <article className="rs-book" key={book.id}><a className="rs-cover" data-book={book.id} href={`/reading?book=${book.id}`} aria-label={`翻开《${book.title}》`}><span className="rs-cover-top">书中见 · {String(index+1).padStart(2,"0")}</span><span className="rs-book-author">{book.author}</span><span className="rs-emblem" aria-hidden="true">{symbols[book.id]}</span><h2>{book.title}</h2><span className="rs-book-theme">{book.theme}</span><span className="rs-open">{book.reader?.furthest?"继续阅读":"开始阅读"} <ArrowUpRight size={15}/></span></a><div className="rs-book-caption"><p>{readingTopics[book.id]||book.subtitle}</p><small>{book.reader?.finished?"已读完 · 可以重读":book.reader?.furthest?`读到第 ${book.reader.paragraph} 段`:"尚未阅读"}{finished>0&&` · ${finished}个结局`}</small></div></article>
  })}</section>
  <section className="rs-journal" aria-label="我的故事记录"><div className="rs-journal-heading"><div><h2>我的故事记录</h2></div><label>按书籍查看<select value={filter} onChange={e=>setFilter(e.target.value)}><option value="all">全部作品</option>{books.map(b=><option value={b.id} key={b.id}>{b.title}</option>)}</select></label></div>
   {!visible.length?<div className="rs-empty"><Feather size={28}/><p>暂无故事记录</p><span>尝试改写故事后，可以在这里继续，或查看已保存的结局。</span></div>:<div className="rs-records">{visible.map(r=><a key={r.book.id+r.id} className="rs-record" href={`/reading?book=${r.book.id}&branch=${encodeURIComponent(r.id)}`}><span className={r.completed?"rs-record-status done":"rs-record-status"}>{r.completed?<Feather size={15}/>:<GitBranch size={15}/>} {r.completed?"已结束":"进行中"}</span><small>《{r.book.title}》 · {new Date(r.updated_at).toLocaleDateString("zh-CN")}</small><h3>{r.title}</h3><p>{r.turns}次互动{r.signature?` · 来访者 ${r.signature}`:""}</p><span className="rs-record-action">{r.completed?"打开结局卡":"继续这段故事"} <ArrowUpRight size={16}/></span></a>)}</div>}
  </section><footer className="rs-footer">只展示当前名字的记录，每本书最近50条。连接同一服务时，输入同名可以继续。<br/>名字不是密码，知道同名的人也能进入；请勿保存敏感资料。<br/>场景为编者导读，新的互动故事由你、人物与安共同推进。</footer>
 </main>;
}
