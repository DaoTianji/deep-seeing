import {useState} from "react";
import type {ReadingLesson} from "./ReadingLessonPage";
import type {Character} from "./reading-api";

export function LessonGuide({lesson,paragraph,busy,people,full,onSelect,onAsk}:{lesson:ReadingLesson;paragraph:number;busy:boolean;people:Character[];full:boolean;onSelect:(paragraph:number)=>void;onAsk:(paragraph:number,speaker:string,message:string)=>void}) {
 const [time,setTime]=useState("then"),[person,setPerson]=useState("");
 const moment=lesson.moments.find(m=>m.paragraph===paragraph)||lesson.moments[0];
 const inPlace=moment?.paragraph===paragraph;
 const views=lesson.views.filter(v=>people.some(c=>c.id===v.id));
 const view=views.find(v=>v.id===person)||views[0];
 const labels=lesson.labels||["那一刻","后来回望"];
 const interpretation=time==="then"?moment?.then:moment?.now;
 return <div className="cr-lesson-guide">
  <h2>{lesson.subtitle}</h2><p>选一处文字，带着自己的理解读。专题只作辅助，不替你下结论。</p>
  <nav aria-label="专题片段">{lesson.moments.map(m=><button disabled={busy} aria-pressed={m.paragraph===paragraph} onClick={()=>onSelect(m.paragraph)} key={m.paragraph}><span>§ {m.paragraph}</span>{!full&&m.paragraph>paragraph?"后续片段 · 点击读到这里":m.title}</button>)}</nav>
  {moment&&<section aria-label="片段解读"><h3>{!full&&moment.paragraph>paragraph?"走到下一处文字":moment.title}</h3>{!inPlace?<button className="cr-guide-action" disabled={busy} onClick={()=>onSelect(moment.paragraph)}>到第 {moment.paragraph} 段一起读</button>:<>
   {(moment.then||moment.now)&&<><div className="cr-guide-time"><button aria-pressed={time==="then"} onClick={()=>setTime("then")}>{labels[0]}</button><button aria-pressed={time==="now"} onClick={()=>setTime("now")}>{labels[1]}</button></div><p className="cr-guide-interpretation">{interpretation}</p><small>编者的一种解读，不是唯一答案。</small>
    {moment.steps&&<ol className="cr-guide-steps" aria-label="原文关系线索">{moment.steps.map(step=><li key={step}>{step}</li>)}</ol>}
    <button className="cr-guide-action" disabled={busy} onClick={()=>onAsk(moment.paragraph,"an",`这段编者导读提出：“${interpretation}”。这是一种解读，不是原文。我有不同理解，请和我讨论这一段，先不要替我下结论。`)}>我有不同的理解</button>
   </>}
   {view&&<><label>换一个立场<select aria-label="专题人物" value={view.id} onChange={e=>setPerson(e.target.value)}>{views.map(v=><option key={v.id} value={v.id}>{v.title}</option>)}</select></label><p>{view.lens}</p><button className="cr-guide-action" disabled={busy} onClick={()=>onAsk(moment.paragraph,view.id,view.question)}>从{view.title}的视角聊聊</button><small>人物视角演绎，不是原文引语；问题由你确认发送。</small></>}
   {lesson.classical&&<button className="cr-guide-action" disabled={busy} onClick={()=>onAsk(moment.paragraph,"an","请解释这一段的关键字词与句意。区分古义、现代释义和解读；释义不是原文，也不需要改写故事。")}>请安解释字词与句意</button>}
   <button className="cr-guide-action" disabled={busy} onClick={()=>onAsk(moment.paragraph,"an",moment.question)}>和安细读这段</button>
  </>}</section>}
  <section aria-label="作者与背景"><h3>作者与背景</h3><small>预先整理的公开资料，不是本轮实时搜索。</small>{lesson.sources.map(s=><details key={s.id}><summary>{s.title.replace(/^(已整理背景|原文线索) · /,"")}</summary><p>{s.summary}</p><a href={s.url} target="_blank" rel="noreferrer">查看来源 ↗</a></details>)}</section>
 </div>;
}
