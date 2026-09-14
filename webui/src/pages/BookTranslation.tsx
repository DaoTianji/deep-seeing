import {useEffect, useRef, useState} from "react";
import {Languages, Pause} from "lucide-react";
import {request} from "./reading-api";

export type TranslatedSentence = {id:string;paragraph:number;original:string;translation?:string};
export type TranslationEdition = {version:string;style:string;target:string;completed:number;total:number;sentences:TranslatedSentence[];scope?:"default"|"personal";revision?:string;pending?:{id:string;completed:number;total:number}};

export function useBookTranslation(book:string,style:string,language?:string) {
 const [saved,setSaved]=useState<{key:string;edition:TranslationEdition}>();
 const [running,setRunning]=useState(false),[error,setError]=useState("");
 const [defaultView,setDefaultView]=useState(false),[hasPersonal,setHasPersonal]=useState(false);
 const generation=useRef(0),controller=useRef<AbortController|null>(null);
 const key=book+":"+style,enabled=["taohuayuan","quanxue","mulan"].includes(book)||(language!=="zh"&&!["kong","beiying"].includes(book));
 const path=`/companion/translation?book=${encodeURIComponent(book)}&style=${style}`;
 const edition=saved?.key===key?saved.edition:undefined;
 useEffect(()=>{
  const gen=++generation.current,load=new AbortController();
  controller.current?.abort();controller.current=null;setRunning(false);setError("");setDefaultView(false);setHasPersonal(false);
  if(enabled)request<TranslationEdition>(path,undefined,load.signal).then(edition=>{if(generation.current===gen){setSaved({key,edition});setHasPersonal(edition.scope==="personal");}}).catch(e=>{if(!load.signal.aborted&&generation.current===gen)setError(e.message);});
  return()=>{++generation.current;load.abort();controller.current?.abort();};
 },[key,path,enabled]);
 async function showDefault(value:boolean){
  if(controller.current)return;
  const gen=generation.current;setError("");
  try{const next=await request<TranslationEdition>(path+(value?"&edition=default":""));if(gen===generation.current){setSaved({key,edition:next});setDefaultView(value);}}
  catch(e){if(gen===generation.current)setError(e instanceof Error?e.message:"无法读取译本");}
 }
 async function start(retranslate=false,sentence?:string) {
  if(!edition||controller.current)return;
  const gen=generation.current,abort=new AbortController();controller.current=abort;setRunning(true);setError("");
  let current=edition;
  try {
   if(retranslate){
    current=await request<TranslationEdition>(path,undefined,abort.signal);
    do {
     const next=await request<TranslationEdition>(path,{style,action:"retranslate",sentence_id:sentence||"",revision:current.revision,run_id:sentence?"":current.pending?.id||"",completed:sentence?0:current.pending?.completed||0},abort.signal);
     if(generation.current!==gen||abort.signal.aborted)break;
     if(next.pending&&current.pending&&next.pending.id===current.pending.id&&next.pending.completed<=current.pending.completed)throw Error("重译没有新增进度，已暂停；原有译文不受影响。");
     current=next;setSaved({key,edition:current});setDefaultView(false);setHasPersonal(current.scope==="personal");
    }while(!sentence&&current.pending&&!abort.signal.aborted&&generation.current===gen);
   }else{
    while(current.completed<current.total&&!abort.signal.aborted&&generation.current===gen){
     const next=await request<TranslationEdition>(path,{style,completed:current.completed},abort.signal);
     if(generation.current!==gen||abort.signal.aborted)break;
     if(next.completed<=current.completed)throw Error("译文没有新增进度，已暂停；请稍后继续。");
     current=next;setSaved({key,edition:current});
    }
   }
  } catch(e) {
   if(generation.current===gen)setError(abort.signal.aborted?"已暂停，已保存的句子不会丢失。":e instanceof Error?e.message:"翻译未完成，可以继续。");
  } finally {
   if(generation.current===gen){
    try{const latest=await request<TranslationEdition>(path);if(generation.current===gen){setSaved({key,edition:latest});setDefaultView(false);setHasPersonal(latest.scope==="personal");}}catch{/* retain confirmed checkpoint */}
    if(generation.current===gen){setRunning(false);controller.current=null;}
   }
  }
 }
 return {enabled,edition,running,error,start,defaultView,hasPersonal,showDefault,pause:()=>controller.current?.abort()};
}

export function BookTranslationBar({translation,onShow}:{translation:ReturnType<typeof useBookTranslation>;onShow:()=>void}) {
 const [confirm,setConfirm]=useState(false);
 const {enabled,edition,running,error,start,pause,defaultView,hasPersonal,showDefault}=translation;
 if(!enabled)return null;
 const complete=!!edition&&edition.completed===edition.total;
 return <section className="cr-book-translation" aria-label="全书逐句译本">
  <div><Languages size={18}/><strong>{edition?.scope==="personal"?"我的译文":"站内默认译文"}</strong><span role="status">{edition?`已保存 ${edition.completed} / ${edition.total} 句${complete?" · 全书完成":""}`:"正在读取译本…"}</span></div>
  <p>默认译文全站共用，直接读取不消耗模型额度。重新翻译只保存到你的个人版本，不影响其他读者。当前样例为 AI 辅助译文，并非出版社译本。</p>
  {edition&&<progress aria-label="全书翻译进度" value={edition.completed} max={Math.max(1,edition.total)}/>}
  {edition?.pending&&<p role="status">个人整篇重译：{edition.pending.completed} / {edition.pending.total} 句。全部完成后替换，期间仍可阅读原有译文。</p>}
  <div>{running?<button onClick={pause}><Pause size={14}/>暂停翻译</button>:<><button disabled={!edition} onClick={()=>{onShow();if(!complete)void start();}}><Languages size={14}/>{complete?"查看逐句对照":edition?.completed?"继续翻译全书":"全书逐句翻译"}</button>{complete&&<button onClick={()=>edition.pending?void start(true):setConfirm(true)}>{edition.pending?"继续个人整篇重译":"重新翻译全文"}</button>}</>}{hasPersonal&&<button disabled={running} onClick={()=>void showDefault(!defaultView)}>{defaultView?"查看我的译文":"查看默认译文"}</button>}{!complete&&!!edition?.completed&&<button onClick={onShow}>查看已保存译文</button>}</div>
  {confirm&&<div role="dialog" aria-label="确认整篇重译"><p>整篇重译会消耗模型额度，完成后替换你的个人译本。默认译文不变。</p><button disabled={running} onClick={()=>{setConfirm(false);onShow();void start(true);}}>开始个人整篇重译</button><button onClick={()=>setConfirm(false)}>取消</button></div>}
  {error&&<p role="alert" className="cr-error">{error}</p>}
 </section>;
}

export function SentenceTranslation({sentences,display,onRetranslate,busy=false}:{sentences:TranslatedSentence[];display:string;onRetranslate?:(id:string)=>void;busy?:boolean}) {
 return <div className="cr-sentence-pairs">{sentences.map(s=><div className="cr-sentence-pair" key={s.id}>
  {display!=="translation"&&<p>{s.original}</p>}
  <p lang="zh" className={s.translation?"cr-sentence-translation":"cr-untranslated"}>{s.translation||"本句尚未翻译"}</p>
  {s.translation&&onRetranslate&&<button className="cr-retranslate-sentence" aria-label={`重新翻译句子 ${s.id}`} disabled={busy} onClick={()=>onRetranslate(s.id)}>重译此句 · 仅自己可见</button>}
 </div>)}<small className="cr-translation-credit">AI 辅助译文 · 原文保持不变</small></div>;
}
