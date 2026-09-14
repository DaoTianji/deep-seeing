import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {AuthorStudio} from "./AuthorStudio";
import {request} from "./reading-api";
vi.mock("./reading-api",()=>({request:vi.fn()}));
const work={id:"w1",title:"旧日随笔",text:"一段真实原文。",written:"",published:"",revised:"",kind:"original",private:true};
const observation={id:"o1",kind:"style",text:"通过具体物象展开表达。",period:"这篇作品",evidence:["w1:c1"],counter_evidence:[],uncertainty:"不代表全部作品"};
const author={id:"a1",name:"虚构作者",aliases:"测试",revision:2,works:[work],analyses:[{version:1,work_ids:["w1"],observations:[observation],changes:"初次认识",warnings:[]}],drafts:[]};
beforeEach(()=>{window.history.replaceState({},"","/reading?view=authors");vi.mocked(request).mockReset().mockImplementation(async(path)=>{
 if(path==="/authors")return [author];if(path==="/books")return [];
 if(path==="/authors/a1")return {author,chunks:{"w1:c1":{id:"w1:c1",work:"w1",text:"一段真实原文。"}}};throw Error("保存失败");
});});
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
async function open(){render(<AuthorStudio/>);fireEvent.click(await screen.findByRole("button",{name:/虚构作者/}));await screen.findByText("什么时间，写下了什么");}
it("opens the labelled pre-generated demo without model analysis",async()=>{
 const fallback=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(path,...args)=>path==="/authors/demo"?author:fallback(path,...args));
 render(<AuthorStudio/>);
 fireEvent.click(await screen.findByRole("button",{name:/已完成的虚构演示/}));
 await screen.findByText("在这些文字里，我们看到了什么");
 expect(vi.mocked(request).mock.calls.some(([path])=>path==="/authors/demo")).toBe(true);
 expect(vi.mocked(request).mock.calls.some(([path])=>path.includes("/analyze"))).toBe(false);
});
it("keeps unknown dates and requires gateway consent before analysis",async()=>{
 await open();expect(screen.getByText("写作时间未知")).toBeInTheDocument();const analyze=screen.getByRole("button",{name:/让安逐篇阅读/});expect(analyze).toBeDisabled();
 fireEvent.click(screen.getByRole("checkbox",{name:/同意将本档案作品发送/}));expect(analyze).toBeEnabled();expect(vi.mocked(request).mock.calls.some(([p])=>p.includes("analyze"))).toBe(false);
});
it("opens original evidence rather than a fabricated generated summary",async()=>{
 await open();fireEvent.click(screen.getByRole("button",{name:"安的作者认识"}));
 expect(screen.getByText("通过具体物象展开表达。")).toBeInTheDocument();const citation=screen.getByRole("button",{name:/1 · 旧日随笔/});citation.focus();fireEvent.click(citation);
 expect(screen.getByRole("dialog",{name:"作者认识的原文依据"})).toHaveTextContent("一段真实原文。");
 expect(screen.getByRole("button",{name:"关闭原文依据"})).toHaveFocus();
 fireEvent.keyDown(document,{key:"Escape"});
 expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
 expect(screen.getByRole("button",{name:/1 · 旧日随笔/})).toHaveFocus();
});
it("separates style selections, new facts, and user consent",async()=>{
 await open();fireEvent.click(screen.getByRole("button",{name:"风格创作"}));
 const generate=screen.getByRole("button",{name:"生成新的草稿"});expect(generate).toBeDisabled();
 fireEvent.click(screen.getByRole("checkbox",{name:/通过具体物象/}));fireEvent.change(screen.getByLabelText("创作任务"),{target:{value:"写一篇明确虚构的短文"}});expect(generate).toBeDisabled();
 fireEvent.click(screen.getByRole("checkbox",{name:/同意将以上输入/}));expect(generate).toBeEnabled();expect(screen.getByText(/生成内容不会作为作者认识的证据/)).toBeInTheDocument();
});
it("does not claim an imported work was saved after an error",async()=>{
 await open();fireEvent.click(screen.getByRole("button",{name:"加入作品"}));fireEvent.change(screen.getByLabelText("标题",{exact:true}),{target:{value:"新文章"}});fireEvent.change(screen.getByLabelText("作品正文"),{target:{value:"尚未保存的正文"}});
 fireEvent.click(screen.getByRole("button",{name:"保存到作品库"}));await waitFor(()=>expect(screen.getByRole("alert")).toHaveTextContent("保存失败"));expect(screen.getByLabelText("作品正文")).toHaveValue("尚未保存的正文");
});
it("requires confirmation, then clears the deleted dossier and deep link",async()=>{
 const original=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(path,...args)=>path==="/authors/a1/delete"?{deleted:true,recoverable:true}:original(path,...args));
 await open();fireEvent.click(screen.getByRole("button",{name:"删除作者"}));
 expect(screen.getByRole("alertdialog")).toHaveTextContent("不会删除书架自带原文或其他名字的记录");
 expect(screen.getByRole("button",{name:"取消，保留档案"})).toHaveFocus();
 fireEvent.click(screen.getByRole("button",{name:"取消，保留档案"}));
 expect(vi.mocked(request).mock.calls.some(([p])=>p.endsWith("/delete"))).toBe(false);
 fireEvent.click(screen.getByRole("button",{name:"删除作者"}));fireEvent.click(screen.getByRole("button",{name:"确认删除作者"}));
 await waitFor(()=>expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
 expect(request).toHaveBeenCalledWith("/authors/a1/delete",{revision:2});
 expect(screen.queryByRole("button",{name:/虚构作者/})).not.toBeInTheDocument();
 expect(new URLSearchParams(location.search).has("author")).toBe(false);
});
it("keeps the author when deletion fails and permits cancelling",async()=>{
 await open();fireEvent.click(screen.getByRole("button",{name:"删除作者"}));fireEvent.click(screen.getByRole("button",{name:"确认删除作者"}));
 await waitFor(()=>expect(screen.getByRole("alertdialog")).toHaveTextContent("保存失败"));
 expect(screen.getByRole("button",{name:/虚构作者/})).toBeInTheDocument();
 fireEvent.keyDown(document,{key:"Escape"});expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
});
