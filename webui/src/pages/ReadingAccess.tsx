import {useCallback,useEffect,useRef,useState,type ReactNode} from "react";
import {ArrowRight,BookOpen,LoaderCircle,LogOut} from "lucide-react";
import {announceReaderChange,readerHeaders,READER_CHANGED,READER_STORAGE_KEY,setActiveReader} from "./reading-session";
import "./reading-access.css";

type Reader={id:string;name:string};
type Session={reader:Reader|null;legacy_available?:boolean};
async function sessionRequest(path:string,body?:unknown):Promise<Session>{
 const response=await fetch(`/api/story/session${path}`,{credentials:"same-origin",method:body===undefined?"GET":"POST",headers:{...readerHeaders(),...(body===undefined?{}:{"Content-Type":"application/json"})},body:body===undefined?undefined:JSON.stringify(body)});
 let result;try{result=await response.json();}catch{throw Error("暂时无法连接服务，请稍后重试。");}
 if(!response.ok)throw Error(result.error||"暂时无法进入，请重试。");
 return result;
}

export function ReadingAccess({children}:{children:ReactNode}){
 const [session,setSession]=useState<Session>();const [checking,setChecking]=useState(true);
 const [name,setName]=useState("");const [claim,setClaim]=useState(false);const [busy,setBusy]=useState(false);const [error,setError]=useState("");
 const generation=useRef(0);
 const accept=useCallback((s:Session)=>{setActiveReader(s.reader?.id||"");setSession(s);setChecking(false);},[]);
 const refresh=useCallback(async(quiet=false)=>{
  const ticket=++generation.current;if(!quiet){setActiveReader("");setChecking(true);}setError("");
  try{const s=await sessionRequest("");if(ticket===generation.current)accept(s);}catch(e){if(ticket===generation.current){accept({reader:null});setError((e as Error).message);}}
 },[accept]);
 useEffect(()=>{
  void refresh();
  const onStorage=(e:StorageEvent)=>{if(e.key===READER_STORAGE_KEY)void refresh();};
  const onVisible=()=>{if(document.visibilityState==="visible")void refresh(true);};
  const onInvalid=()=>{void refresh();};
  const onRestore=(e:PageTransitionEvent)=>{if(e.persisted)void refresh();};
  window.addEventListener("storage",onStorage);window.addEventListener(READER_CHANGED,onInvalid);document.addEventListener("visibilitychange",onVisible);window.addEventListener("pageshow",onRestore);
  return()=>{generation.current++;setActiveReader("");window.removeEventListener("storage",onStorage);window.removeEventListener(READER_CHANGED,onInvalid);document.removeEventListener("visibilitychange",onVisible);window.removeEventListener("pageshow",onRestore);};
 },[refresh]);
 const enter=async()=>{
  if(busy)return;setBusy(true);setError("");const ticket=++generation.current;
  try{const s=await sessionRequest("",{name:name.trim(),claim_legacy:claim});if(ticket===generation.current){accept(s);setName("");setClaim(false);announceReaderChange();}}
  catch(e){if(ticket===generation.current)setError((e as Error).message);}finally{setBusy(false);}
 };
 const logout=async()=>{
  if(busy)return;setBusy(true);setError("");
  try{await sessionRequest("/logout",{});announceReaderChange();setName("");setClaim(false);window.history.replaceState({},"","/reading");await refresh();window.dispatchEvent(new PopStateEvent("popstate"));}
  catch(e){setError((e as Error).message);}finally{setBusy(false);}
 };
 if(checking)return <main className="ra-wait" role="status"><BookOpen size={32}/><p>正在加载阅读记录…</p></main>;
 if(session?.reader)return <div className="ra-signed-in"><header className="ra-reader-bar"><span>当前读者：<strong>{session.reader.name}</strong></span><span>{error&&<span role="alert">{error}</span>}<button disabled={busy} onClick={logout}><LogOut size={14}/>{busy?"正在退出":"退出 / 换个名字"}</button></span></header><div className="ra-content" key={session.reader.id}>{children}</div></div>;
 return <main className="ra-entry"><header className="ra-brand"><BookOpen size={23}/><span>书中见</span></header><section className="ra-entry-body"><div className="ra-invitation"><h1>阅读、提问，<br/><em>也试试改写故事。</em></h1><p className="ra-intro">输入名字，保存阅读进度、笔记和故事记录。<br/>下次使用同一个名字继续。</p></div><form className="ra-card" onSubmit={e=>{e.preventDefault();void enter();}}><h2>输入读者名字</h2><label htmlFor="reader-name">名字或独特的笔名</label><input id="reader-name" name="reader-name" autoComplete="nickname" autoFocus placeholder="例如：小林0911" value={name} onChange={e=>setName(e.target.value)} maxLength={80} disabled={busy} required/><p className="ra-name-help">同一个名字会打开同一份记录。首尾空格忽略，英文字母不区分大小写。</p>{session?.legacy_available&&<label className="ra-legacy"><input type="checkbox" checked={claim} onChange={e=>setClaim(e.target.checked)} disabled={busy}/><span>把此浏览器以前的匿名记录归到这个新名字下<small>自愿选择；已有名字不会被合并或覆盖。</small></span></label>}{error&&<p className="ra-error" role="alert">{error}</p>}<button className="ra-enter" disabled={busy||!name.trim()||Array.from(name.trim()).length>40}>{busy?<LoaderCircle size={18}/>:<>开始阅读<ArrowRight size={18}/></>}</button><div className="ra-boundary"><strong>只是名字，不是密码。</strong><p>不同名字的记录分开保存；但知道同一个名字的人也能进入。请用独特笔名，不要存放敏感资料。</p></div></form></section><footer className="ra-footer">不用注册，不必设置密码。退出不会删除你的记录。<br/>在连接同一服务的设备上，输入同名即可继续。</footer></main>;
}
