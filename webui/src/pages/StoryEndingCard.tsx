import { BookOpen, Download, Feather, Sparkles } from "lucide-react";
import {useState} from "react";
import {endingMarkdown} from "./ending-markdown";
import {request, type Book, type Branch} from "./reading-api";

export function StoryEndingCard({ book, branch, onSigned }: { book: Book; branch: Branch; onSigned?: (b:Branch)=>void }) {
 const [name,setName]=useState("");const [saving,setSaving]=useState(false);const [error,setError]=useState("");
 async function sign(){if(saving||!name.trim())return;setSaving(true);setError("");try{const b=await request<Branch>(`/branches/${branch.id}/signature`,{name:name.trim(),revision:branch.revision});onSigned?.(b);}catch(e){setError(e instanceof Error?e.message:"署名没有保存");}finally{setSaving(false);}}

 const ending = branch.ending;
 if (!ending) return null;
 function download() {
  if (!ending) return;
  const text = endingMarkdown(book, branch);
  const url = URL.createObjectURL(new Blob([text], { type: "text/markdown;charset=utf-8" }));
  const a = document.createElement("a"); a.href = url; a.download = `${book.title}-故事结局卡-${branch.id.slice(0,6)}.md`; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
 }
 return <article className="rw-ending-card" aria-label="故事结局卡">
  <header><span className="rw-ending-seal"><Feather size={24}/></span><span>故事已结束</span><h2>{ending.title}</h2><p>《{book.title}》的一种新可能</p></header>
  <div className="rw-ending-story">{ending.story.split(/\n+/).filter(Boolean).map((p,i) => <p key={i}>{p}</p>)}</div>
  <p className="rw-ending-resolution">{ending.resolution}</p>
  <div className="rw-ending-paths"><section><h3><BookOpen size={17}/>原著的故事线</h3><small>含原著结局 · 共同起点与原有走向</small><ol>{book.scenes.map(s => <li key={s.id}><strong>{s.title}</strong><p>{s.summary}</p></li>)}</ol></section><section><h3><Feather size={17}/>改写后的故事线</h3><small>从起点到本次结局</small><ol>{ending.new_timeline.map((s,i) => <li key={i}><p>{s}</p></li>)}</ol></section></div>
  <section className="rw-ending-impact"><h3><Sparkles size={18}/>你的参与带来了什么变化</h3>{ending.contributions.length ? ending.contributions.map(c => <div key={c.turn_id}><p>{c.text}</p><blockquote>你曾说：“{branch.turns.find(t => t.id === c.turn_id)?.message}”</blockquote></div>) : <p>本次没有记录到由你的建议直接促成的变化。</p>}</section>
  <section className="rw-signature" aria-label="结局署名">{branch.signature?<><Feather size={19}/><span>来访者 · <span className="rw-signature-name">{branch.signature}</span></span><small>原著作者：{book.author} · 互动生成：安</small></>:<><label>留下你的署名<input value={name} maxLength={40} onChange={e=>setName(e.target.value)} placeholder="你的名字或笔名" disabled={saving}/></label><button className="rw-secondary" disabled={saving||!name.trim()} onClick={sign}>{saving?"正在保存…":"确认署名"}</button><small>可不署名；确认后保留在这张卡片中，不进入人物记忆。署名确认后不可更改。</small></>}{error&&<p role="alert">{error}</p>}</section>
  <footer><span>已完结并存档 · 影响力 {branch.influence || 5}/10<br/><small>互动创作，不覆盖原著；不再自动续写。</small></span><button className="rw-secondary" onClick={download}><Download size={16}/>下载结局卡</button></footer>
 </article>;
}
