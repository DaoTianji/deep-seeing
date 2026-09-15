import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {BookTranslationBar,SentenceTranslation,useBookTranslation,type TranslationEdition} from "./BookTranslation";
import {request} from "./reading-api";
vi.mock("./reading-api",()=>({request:vi.fn()}));
let saved:TranslationEdition;
function Harness({book="necklace",style="fluent"}:{book?:string;style?:string}) {
 const t=useBookTranslation(book,style);
 return <><BookTranslationBar translation={t} onShow={()=>{}}/>{t.edition&&<SentenceTranslation sentences={t.edition.sentences} display="parallel" busy={t.running} onRetranslate={id=>void t.start(true,id)}/>}</>;
}
beforeEach(()=>{
 saved={version:"v1",style:"fluent",target:"zh",completed:0,total:2,sentences:[{id:"p1-s1",paragraph:1,original:"Hello."},{id:"p1-s2",paragraph:1,original:"Goodbye."}]};
 vi.mocked(request).mockReset().mockImplementation(async(_path,body)=>{
  if(body){const count=saved.completed+1;saved={...saved,completed:count,sentences:saved.sentences.map((s,i)=>i<count?{...s,translation:i===0?"你好。":"再见。"}:s)};}
  return saved;
 });
});
it("retranslates only the selected sentence personally and can display the shared default",async()=>{
 const base:TranslationEdition={...saved,scope:"default",revision:"r1",completed:2,sentences:saved.sentences.map((s,i)=>({...s,translation:i?"再见。":"你好。"}))};saved=base;
 vi.mocked(request).mockImplementation(async(path,body)=>{
  if(body)saved={...base,scope:"personal",revision:"r2",sentences:[base.sentences[0],{...base.sentences[1],translation:"回头见。"}]};
  return path.includes("edition=default")?base:saved;
 });
 render(<Harness/>);await screen.findByText("你好。");
 expect(vi.mocked(request).mock.calls.filter(c=>c[1])).toHaveLength(0);
 fireEvent.click(screen.getByRole("button",{name:"重新翻译句子 p1-s2"}));
 await screen.findByText("回头见。");await waitFor(()=>expect(screen.getByRole("button",{name:"查看默认译文"})).toBeEnabled());
 expect(vi.mocked(request).mock.calls.filter(c=>c[1]).map(c=>c[1])).toEqual([{style:"fluent",action:"retranslate",sentence_id:"p1-s2",revision:"r1",run_id:"",completed:0}]);
 expect(screen.getByText("你好。")).toBeInTheDocument();expect(screen.getByText("我的译文")).toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"查看默认译文"}));await screen.findByText("再见。");
 expect(screen.getByText("站内默认译文")).toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"查看我的译文"}));await screen.findByText("回头见。");
 expect(vi.mocked(request).mock.calls.filter(c=>c[1])).toHaveLength(1);
});
it("requires confirmation for a whole-book rewrite and resumes its private checkpoint",async()=>{
 saved={...saved,scope:"default",revision:"r1",completed:2,sentences:saved.sentences.map(s=>({...s,translation:"旧译文"}))};
 let batches=0;
 vi.mocked(request).mockImplementation(async(_path,body)=>{
  if(body){batches++;saved=batches===1?{...saved,pending:{id:"job1",completed:1,total:2}}:{...saved,scope:"personal",revision:"r2",pending:undefined,sentences:saved.sentences.map(s=>({...s,translation:"新译文"}))};}
  return saved;
 });
 render(<Harness/>);await screen.findByRole("button",{name:"重新翻译全文"});
 fireEvent.click(screen.getByRole("button",{name:"重新翻译全文"}));
 expect(screen.getByRole("dialog",{name:"确认整篇重译"})).toBeInTheDocument();expect(batches).toBe(0);
 fireEvent.click(screen.getByRole("button",{name:"开始个人整篇重译"}));
 await screen.findByText("我的译文");await waitFor(()=>expect(screen.getByRole("button",{name:"重新翻译全文"})).toBeEnabled());
 expect(vi.mocked(request).mock.calls.filter(c=>c[1]).map(c=>c[1])).toEqual([
  {style:"fluent",action:"retranslate",sentence_id:"",revision:"r1",run_id:"",completed:0},
  {style:"fluent",action:"retranslate",sentence_id:"",revision:"r1",run_id:"job1",completed:1},
 ]);
});
it.each(["taohuayuan","quanxue","mulan"])("offers the shared modern Chinese edition for %s",async(book)=>{
 render(<Harness book={book}/>);await screen.findByText("站内默认译文");
 expect(request).toHaveBeenCalledWith(expect.stringContaining(`book=${book}`),undefined,expect.any(AbortSignal));
});
afterEach(cleanup);
it("keeps sentence actions beside the translation as accessible icon buttons",()=>{
 const retry=vi.fn();const {container}=render(<SentenceTranslation sentences={[{id:"p1-s1",paragraph:1,original:"That was all.",translation:"这就是全部了。"}]} display="parallel" onRetranslate={retry}/>);
 const button=screen.getByRole("button",{name:"重新翻译句子 p1-s1"});
 expect(button).toHaveAttribute("title","重译此句 · 仅保存到我的译文");expect(button).toHaveTextContent("");
 expect(container.querySelector(".cr-sentence-translation-line")).toContainElement(button);
 expect(button.parentElement).toContainElement(screen.getByText("这就是全部了。"));
 expect(screen.queryByText("AI 辅助译文 · 原文保持不变")).not.toBeInTheDocument();
 fireEvent.click(button);expect(retry).toHaveBeenCalledExactlyOnceWith("p1-s1");
});
it("loads saved edition without translating, then batches and reuses on reopen",async()=>{
 const view=render(<Harness/>);
 await screen.findByText("已保存 0 / 2 句");
 expect(vi.mocked(request).mock.calls.filter(c=>c[1]!==undefined)).toHaveLength(0);
 fireEvent.click(screen.getByRole("button",{name:"全书逐句翻译"}));
 await screen.findByText("已保存 2 / 2 句 · 全书完成");
 expect(screen.getByText("你好。")).toBeInTheDocument();expect(screen.getByText("Hello.")).toBeInTheDocument();
 await waitFor(()=>expect(screen.getByRole("button",{name:"查看逐句对照"})).toBeEnabled());
 expect(vi.mocked(request).mock.calls.filter(c=>c[1]!==undefined).map(c=>c[1])).toEqual([{style:"fluent",completed:0},{style:"fluent",completed:1}]);
 view.unmount();render(<Harness/>);await screen.findByRole("button",{name:"查看逐句对照"});
 fireEvent.click(screen.getByRole("button",{name:"查看逐句对照"}));
 expect(vi.mocked(request).mock.calls.filter(c=>c[1]!==undefined)).toHaveLength(2);
});
it("keeps successful batches on failure and resumes from the saved checkpoint",async()=>{
 const original=vi.mocked(request).getMockImplementation()!;let fail=true;
 vi.mocked(request).mockImplementation(async(...args)=>{if(args[1]&&saved.completed===1&&fail){fail=false;throw Error("模型暂时不可用");}return original(...args);});
 render(<Harness/>);await screen.findByText("已保存 0 / 2 句");fireEvent.click(screen.getByRole("button",{name:"全书逐句翻译"}));
 await screen.findByRole("alert");await screen.findByRole("button",{name:"继续翻译全书"});
 expect(screen.getByText("你好。")).toBeInTheDocument();expect(saved.completed).toBe(1);
 fireEvent.click(screen.getByRole("button",{name:"继续翻译全书"}));await screen.findByText("已保存 2 / 2 句 · 全书完成");
});
it("pauses a pending batch without automatically retrying",async()=>{
 vi.mocked(request).mockImplementation(async(_path,body,signal)=>body?await new Promise((_resolve,reject)=>signal?.addEventListener("abort",()=>reject(new DOMException("aborted","AbortError")))):saved);
 render(<Harness/>);await screen.findByText("已保存 0 / 2 句");fireEvent.click(screen.getByRole("button",{name:"全书逐句翻译"}));
 fireEvent.click(await screen.findByRole("button",{name:"暂停翻译"}));
 await screen.findByText("已暂停，已保存的句子不会丢失。");await screen.findByRole("button",{name:"全书逐句翻译"});
 expect(vi.mocked(request).mock.calls.filter(c=>c[1]!==undefined)).toHaveLength(1);
});
it("does not offer English translation for the Chinese book",()=>{
 render(<Harness book="kong"/>);expect(screen.queryByRole("region",{name:"全书逐句译本"})).not.toBeInTheDocument();expect(request).not.toHaveBeenCalled();
});
it("switches edition by style and never silently generates a new style",async()=>{
 const view=render(<Harness/>);await screen.findByText("已保存 0 / 2 句");
 view.rerender(<Harness style="literal"/>);await waitFor(()=>expect(request).toHaveBeenCalledWith(expect.stringContaining("style=literal"),undefined,expect.any(AbortSignal)));
 expect(vi.mocked(request).mock.calls.filter(c=>c[1]!==undefined)).toHaveLength(0);
});
