import {act,cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {CompanionReader,type ReaderState} from "./CompanionReader";
import {request,type Book} from "./reading-api";
vi.mock("./reading-api",()=>({request:vi.fn()}));
const book:Book={id:"necklace",title:"项链",author:"莫泊桑",subtitle:"",source_url:"https://example.org",source_note:"历史原文",version:"1",entry_scene:1,scenes:[],evidence:[],characters:[{id:"mathilde",name:"玛蒂尔德",initial:"M",description:"",color:"#aaa"}]};
let state:ReaderState;
beforeEach(()=>{
 window.history.replaceState({},"","/reading?book=necklace");
 state={revision:0,version:"1",position:{paragraph:1,furthest:0,full:false,finished:false,bookmarks:[]},turns:[],notes:[]};
 vi.mocked(request).mockReset().mockImplementation(async(path,body)=>{
  if(path.startsWith("/companion/translation"))return {version:"1",style:"fluent",target:"zh",completed:0,total:7,sentences:[]};
  if(path==="/companion")return {state,paragraphs:Array.from({length:7},(_,i)=>({id:i+1,text:`原文段落 ${i+1}`,start_byte:i*20,end_byte:i*20+19})),research_available:true,research_provider:"bing"};
  if(path==="/companion/position"){state={...state,revision:state.revision+1,position:(body as {position:ReaderState["position"]}).position};return state;}
  throw Error("保存失败，稍后重试");
 });
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.restoreAllMocks();});

function moveReadingAnchor(container:HTMLElement,id:number){
 for(const node of container.querySelectorAll<HTMLElement>("[data-paragraph]")){
  const top=(Number(node.dataset.paragraph)-id)*500+100;
  vi.spyOn(node,"getBoundingClientRect").mockReturnValue({top,bottom:top+490,height:490} as DOMRect);
 }
 fireEvent.scroll(window);
}
it("selects a paragraph by clicking its text without moving the page",async()=>{
 const scroll=vi.fn();vi.stubGlobal("scrollIntoView",scroll);
 const {container}=render(<CompanionReader book={book} onStory={vi.fn()} continuous/>);await screen.findByText("原文段落 1");
 for(const node of container.querySelectorAll("section"))node.scrollIntoView=scroll;
 fireEvent.click(screen.getByText("原文段落 4"));
 await waitFor(()=>expect(state.position.paragraph).toBe(4));
 expect(screen.getByText("已固定第 4 段")).toBeInTheDocument();expect(scroll).not.toHaveBeenCalled();
 fireEvent.click(screen.getByText("原文段落 4"));
 expect(vi.mocked(request).mock.calls.filter(([path])=>path==="/companion/position")).toHaveLength(1);
});
it("ignores text selection, swipe gestures and nested controls",async()=>{
 vi.stubGlobal("PointerEvent",MouseEvent);
 const {container}=render(<CompanionReader book={book} onStory={vi.fn()} continuous/>);await screen.findByText("原文段落 1");
 const text=screen.getByText("原文段落 4");
 const selection=vi.spyOn(window,"getSelection").mockReturnValue({isCollapsed:false} as Selection);
 fireEvent.click(text);expect(state.position.paragraph).toBe(1);selection.mockRestore();
 fireEvent.pointerDown(text,{clientX:50,clientY:100});fireEvent.pointerMove(text,{clientX:50,clientY:160});fireEvent.click(text);
 expect(state.position.paragraph).toBe(1);
 const control=document.createElement("button");control.textContent="句子工具";container.querySelector('[data-paragraph="4"]')!.append(control);
 fireEvent.click(control);expect(state.position.paragraph).toBe(1);
 expect(vi.mocked(request).mock.calls.filter(([path])=>path==="/companion/position")).toHaveLength(0);
});
it("offers mobile reading actions and returns from the panel with the draft intact",async()=>{
 vi.stubGlobal("innerWidth",390);
 render(<CompanionReader book={book} onStory={vi.fn()} continuous/>);await screen.findByText("原文段落 1");
 expect(screen.getByRole("navigation",{name:"移动阅读工具"})).toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"讨论这段"}));
 expect(screen.getByRole("dialog",{name:"伴读面板"})).toBeInTheDocument();
 expect(screen.getByRole("button",{name:"关闭伴读"})).toHaveFocus();expect(document.body.style.overflow).toBe("hidden");
 fireEvent.change(screen.getByRole("textbox",{name:"伴读问题"}),{target:{value:"先记下一个疑问"}});
 fireEvent.click(screen.getByRole("button",{name:"关闭伴读"}));
 expect(document.body.style.overflow).not.toBe("hidden");expect(screen.queryByRole("dialog",{name:"伴读面板"})).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"讨论这段"}));
 expect(screen.getByLabelText("伴读问题")).toHaveValue("先记下一个疑问");
 fireEvent.keyDown(document,{key:"Escape"});expect(screen.queryByRole("dialog",{name:"伴读面板"})).not.toBeInTheDocument();
});
it("tracks the mobile visual viewport and releases the panel lock on desktop",async()=>{
 vi.stubGlobal("innerWidth",390);
 const visual=Object.assign(new EventTarget(),{height:800,offsetTop:0});vi.stubGlobal("visualViewport",visual);
 const {container}=render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.click(screen.getByRole("button",{name:"讨论这段"}));visual.height=420;visual.offsetTop=12;act(()=>{visual.dispatchEvent(new Event("resize"));});
 expect(container.querySelector(".cr-room")).toHaveStyle({"--reader-viewport-height":"420px","--reader-viewport-top":"12px"});
 vi.stubGlobal("innerWidth",1200);fireEvent.resize(window);
 expect(screen.queryByRole("dialog",{name:"伴读面板"})).not.toBeInTheDocument();expect(document.body.style.overflow).not.toBe("hidden");
});
it("follows scrolling, preserves manual selection, and resumes without a model call",async()=>{
 const {container}=render(<CompanionReader book={book} onStory={vi.fn()} continuous/>);await screen.findByText("原文段落 1");
 moveReadingAnchor(container,3);
 await waitFor(()=>expect(screen.getByLabelText("定位原文段落")).toHaveValue("3"));
 expect(state.position.paragraph).toBe(3);
 fireEvent.change(screen.getByLabelText("定位原文段落"),{target:{value:"2"}});
 await waitFor(()=>expect(state.position.paragraph).toBe(2));
 expect(screen.getByText("已固定第 2 段")).toBeInTheDocument();
 moveReadingAnchor(container,5);
 expect(screen.getByRole("button",{name:"恢复自动跟随"})).toHaveAttribute("aria-pressed","false");
 fireEvent.click(screen.getByRole("button",{name:"恢复自动跟随"}));
 await waitFor(()=>expect(state.position.paragraph).toBe(5));
 expect(vi.mocked(request).mock.calls.every(([path])=>!path.includes("/chat")&&!path.includes("/branches"))).toBe(true);
});
it("freezes the discussion paragraph while composing and sends that paragraph",async()=>{
 const {container}=render(<CompanionReader book={book} onStory={vi.fn()} continuous/>);await screen.findByText("原文段落 1");
 moveReadingAnchor(container,2);await waitFor(()=>expect(state.position.paragraph).toBe(2));
 fireEvent.change(screen.getByLabelText("伴读问题"),{target:{value:"解释这一段"}});
 moveReadingAnchor(container,5);
 const answer={...state,revision:state.revision+1,turns:[]};
 const fetch=vi.fn().mockResolvedValue(new Response(JSON.stringify({type:"result",data:answer})+"\n"));vi.stubGlobal("fetch",fetch);
 fireEvent.click(screen.getByRole("button",{name:"发送伴读问题"}));
 await waitFor(()=>expect(fetch).toHaveBeenCalledOnce());
 expect(JSON.parse(fetch.mock.calls[0][1].body)).toMatchObject({paragraph:2,revision:1});
});
it("does not claim the position changed when automatic saving fails",async()=>{
 const {container}=render(<CompanionReader book={book} onStory={vi.fn()} continuous/>);await screen.findByText("原文段落 1");
 vi.mocked(request).mockRejectedValueOnce(Error("位置保存失败"));moveReadingAnchor(container,4);
 expect(await screen.findByRole("alert")).toHaveTextContent("位置保存失败");
 expect(screen.getByLabelText("定位原文段落")).toHaveValue("1");
 expect(screen.getByRole("button",{name:"恢复自动跟随"})).toBeInTheDocument();
});
it("identifies the no-key search channel without promising free model calls",async()=>{
 render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 expect(screen.getByRole("note")).toHaveTextContent("Bing RSS · 无需新增 Key");
 expect(screen.getByRole("note")).toHaveTextContent("消耗模型用量");
 fireEvent.change(screen.getByLabelText("伴读对象"),{target:{value:"mathilde"}});
 expect(screen.queryByRole("note")).not.toBeInTheDocument();
});
it("keeps failed-request usage visible without pretending an answer was saved",async()=>{
 const metrics={duration_ms:1250,model_calls:1,reported_calls:1,usage:{prompt_tokens:30,completion_tokens:10,total_tokens:40}};
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(new Response([
  JSON.stringify({type:"metrics",data:metrics}),
  JSON.stringify({type:"error",data:{message:"本轮审核失败，未保存"}}),
 ].join("\n")+"\n")));
 render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.change(screen.getByLabelText("伴读问题"),{target:{value:"请解释"}});
 fireEvent.click(screen.getByRole("button",{name:"发送伴读问题"}));
 expect(await screen.findByRole("alert")).toHaveTextContent("本轮审核失败，未保存");
 expect(screen.getByText(/最近一次请求 · 1.3 秒 · 1 次模型调用/)).toBeInTheDocument();
 expect(screen.getByText(/合计 40 Token/)).toBeInTheDocument();
 expect(state.turns).toHaveLength(0);
 expect(screen.getByLabelText("伴读问题")).toHaveValue("请解释");
});

it("renders stored per-turn cost after reopening without a model call",async()=>{
 state.turns=[{id:"t",speaker:"an",paragraph:1,full:false,action:"chat",message:"请解释",reply:"一种理解",needs_spoiler:false,sources:[],evidence:[1],metrics:{duration_ms:2500,model_calls:2,reported_calls:0}}];
 const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 render(<CompanionReader book={book} onStory={vi.fn()}/>);
 expect(await screen.findByText(/此条生成记录 · 2.5 秒 · 2 次模型调用/)).toBeInTheDocument();
 expect(screen.getByText(/Token 未完整返回（0\/2 次）/)).toBeInTheDocument();
 expect(fetch).not.toHaveBeenCalled();
});
it("opens original text and never generates a story or chat on page selection",async()=>{
 render(<CompanionReader book={book} onStory={vi.fn()}/>);
 expect(await screen.findByText("原文段落 1")).toBeInTheDocument();
 expect(screen.queryByText("原文段落 6")).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"下一页"}));
 expect(await screen.findByText("原文段落 6")).toBeInTheDocument();
 expect(request).toHaveBeenCalledWith("/companion/position",expect.objectContaining({position:expect.objectContaining({paragraph:6})}));
 expect(vi.mocked(request).mock.calls.every(([path])=>!path.includes("/branches")&&!path.includes("/chat"))).toBe(true);
});

it("adjusts reading layout and exposes completion without needing the sidebar",async()=>{
 const {container}=render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.change(screen.getByLabelText("阅读行距"),{target:{value:"2.2"}});
 expect(container.querySelector(".cr-paper")).toHaveStyle({"--reader-line":"2.2"});
 fireEvent.click(screen.getByRole("button",{name:"收起伴读"}));expect(container.querySelector(".cr-room")).toHaveClass("cr-panel-hidden");
 fireEvent.click(screen.getByRole("button",{name:"读完之后"}));expect(screen.getByRole("dialog",{name:"读完之后"})).toBeInTheDocument();
 expect(state.position.finished).toBe(false);
});

it("does not disclose future scene titles in the optional context drawer",async()=>{
 const detailed:Book={...book,excerpts:{e1:{text:"原文段落 1",start_byte:0,end_byte:19},e2:{text:"未读",start_byte:20,end_byte:39}},scenes:[
 {id:1,title:"门前",evidence_ids:["e1"],characters:["mathilde"],time:"",place:"",summary:"",question:"",choice:"",resistance:""},
 {id:2,title:"不能提前展示的结局",evidence_ids:["e2"],characters:["mathilde"],time:"",place:"",summary:"",question:"",choice:"",resistance:""}]};
 render(<CompanionReader book={detailed} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.click(screen.getByRole("button",{name:"人物与场景"}));
 const dialog=screen.getByRole("dialog",{name:"已读人物与场景"});expect(dialog).toHaveTextContent("门前");expect(dialog).not.toHaveTextContent("不能提前展示的结局");
 fireEvent.click(screen.getByRole("button",{name:"玛蒂尔德"}));
 expect(screen.queryByRole("dialog")).not.toBeInTheDocument();expect(screen.getByLabelText("伴读对象")).toHaveValue("mathilde");
});
it("offers An and character perspective, requiring confirmation before full-book discussion",async()=>{
 render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.change(screen.getByLabelText("伴读对象"),{target:{value:"mathilde"}});
 expect(screen.getByText("人物视角演绎 · 不改变剧情")).toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"展开全书"}));
 expect(screen.getByRole("dialog",{name:"展开全书讨论"})).toBeInTheDocument();expect(request).not.toHaveBeenCalledWith("/companion/position",expect.anything());
 fireEvent.click(screen.getByRole("button",{name:"允许全书讨论"}));
 await waitFor(()=>expect(screen.getByLabelText("伴读对象")).toHaveValue("an"));expect(state.position.full).toBe(true);
});
it("enters another story only after explicit completion action",async()=>{
 const enter=vi.fn();render(<CompanionReader book={book} onStory={enter}/>);await screen.findByText("原文段落 1");
 fireEvent.click(screen.getByRole("button",{name:"我已经读完了"}));expect(enter).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole("button",{name:"尝试改写故事"}));
 await waitFor(()=>expect(enter).toHaveBeenCalledOnce());expect(state.position.finished).toBe(true);
});
it("does not discard a note or claim saved when persistence fails",async()=>{
 render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.click(screen.getByRole("button",{name:"写笔记"}));
 fireEvent.change(screen.getByLabelText("阅读笔记"),{target:{value:"这是我的理解"}});
 fireEvent.click(screen.getByRole("button",{name:"保存笔记"}));
 expect(await screen.findByRole("alert")).toHaveTextContent("保存失败");expect(screen.getByLabelText("阅读笔记")).toHaveValue("这是我的理解");
});
it("waits for a confirmed streamed result and renders real source cards",async()=>{
 const answer={...state,revision:1,turns:[{id:"t",speaker:"an",paragraph:1,full:false,action:"chat",message:"解释一下",reply:"一种可能的理解。",needs_spoiler:false,evidence:[1],sources:[{id:"s",title:"历史资料",url:"https://example.org/history",summary:"核实过的背景"}]}]};
 const fetch=vi.fn().mockResolvedValue(new Response([JSON.stringify({type:"status",data:{message:"正在阅读背景来源"}}),JSON.stringify({type:"result",data:answer})].join("\n")));vi.stubGlobal("fetch",fetch);
 render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.change(screen.getByLabelText("伴读问题"),{target:{value:"解释一下"}});fireEvent.click(screen.getByRole("button",{name:"发送伴读问题"}));
 expect(await screen.findByText("一种可能的理解。")).toBeInTheDocument();expect(screen.getByText("历史资料")).toBeInTheDocument();expect(screen.getByLabelText("伴读问题")).toHaveValue("");
 const body=JSON.parse(fetch.mock.calls[0][1].body);expect(body).toMatchObject({speaker:"an",paragraph:1,action:"chat"});
});

it("labels unsuccessful research without inventing a source card",async()=>{
 const answer={...state,revision:1,turns:[{id:"t",speaker:"an",paragraph:1,full:false,action:"chat",message:"查证背景",reply:"本次没有查到可用的背景资料，只能先从原文理解。",needs_spoiler:false,research_status:"no_usable_sources",evidence:[1],sources:[]}]};
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(new Response(JSON.stringify({type:"result",data:answer})+"\n")));
 render(<CompanionReader book={book} onStory={vi.fn()}/>);await screen.findByText("原文段落 1");
 fireEvent.change(screen.getByLabelText("伴读问题"),{target:{value:"查证背景"}});fireEvent.click(screen.getByRole("button",{name:"发送伴读问题"}));
 expect(await screen.findByText(/本次联网未取得可用来源/)).toBeInTheDocument();
 expect(screen.queryByText("打开实际读取的来源 ↗")).not.toBeInTheDocument();
});

it("keeps an essay read-only after completion and has no translation batch",async()=>{
 const enter=vi.fn();render(<CompanionReader book={{...book,id:"beiying",title:"背影",language:"zh",read_only:true,characters:[]}} onStory={enter}/>);await screen.findByText("原文段落 1");
 expect(screen.queryByRole("option",{name:/人物视角/})).not.toBeInTheDocument();
 expect(vi.mocked(request).mock.calls.some(([p])=>p.startsWith("/companion/translation"))).toBe(false);
 fireEvent.click(screen.getByRole("button",{name:"读完之后"}));expect(screen.queryByRole("button",{name:"尝试改写故事"})).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"与安讨论全文"}));await waitFor(()=>expect(state.position.finished).toBe(true));expect(enter).not.toHaveBeenCalled();
});
