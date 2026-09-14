import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import ReadingPage from "./ReadingPage";
import {StoryEndingCard} from "./StoryEndingCard";
import {endingMarkdown} from "./ending-markdown";
import { request, type Book, type Branch } from "./reading-api";
import {kongRewrites} from "./reading-entry";

vi.mock("./reading-api", () => ({request: vi.fn()}));

it("exports a real multiline Markdown card and renders separate story paragraphs",()=>{
 const b:Branch={...branch,signature:"测试来访者",ending:{title:"一封未寄的信",story:"第一段。\n\n第二段。",resolution:"选择已经明确，后果保持未知。",new_timeline:["先前的处境","现在的选择"],contributions:[],revision:2,created_at:"2026-09-09"}};
 const {container}=render(<StoryEndingCard book={book} branch={b}/>);
 expect(container.querySelectorAll(".rw-ending-story p")).toHaveLength(2);
 const markdown=endingMarkdown(book,b);
 expect(markdown).toContain("# 一封未寄的信\n\n《项链》");
 expect(markdown).toContain("\n## 原著故事线（含结局）\n");
 expect(markdown).toContain("\n## 这一次的故事\n");
 expect(markdown).toContain("\n来访者署名：测试来访者\n");
 expect(markdown).not.toContain("\\n");
});
const book: Book = { id:"necklace", title:"项链", author:"莫泊桑", subtitle:"另一种可能", source_url:"https://www.gutenberg.org/ebooks/10483", source_note:"历史原文", version:"v1", characters:[{id:"mathilde",name:"玛蒂尔德",initial:"M",description:"渴望被看见",color:"#987"},{id:"loisel",name:"路瓦栽",initial:"L",description:"寻找办法",color:"#789"}], scenes:Array.from({length:7},(_,i)=>({id:i+1,title:`场景${i+1}`,time:"当夜",place:"家",summary:"项链遗失了",question:"是否坦白？",choice:"坦白",resistance:"高",characters:["mathilde","loisel"],evidence_ids:[]})), evidence:[] };
const branch: Branch={id:"branch1",anchor:4,revision:1,scene:book.scenes[3],memories:[],turns:[],events:[],diverged:false};
beforeEach(()=>{window.history.replaceState({},"","/reading?book=necklace&mode=story");localStorage.clear();vi.mocked(request).mockReset();vi.mocked(request).mockImplementation(async(path)=>{
 if(path==="/books")return [];
 if(path==="/book")return {book,reading:{status:"not_started",completed:0,insights:[]},mode:"agent",model_connected:true};
 if(path==="/branches")return branch;
 if(path.includes("/snapshot"))return {facts:[{id:"f1",text:"朋友还不知道遗失",evidence_id:"e4"}],memories:[]};
 throw Error("测试：网络中断，本轮未完成");
});});
afterEach(cleanup);
it.each(["", "teacher", "letters", "challenge"])("enters the curated Kong scene once for %s, with a draft and no automatic model turn",async(idea)=>{
 window.history.replaceState({},"",`/reading?book=kong&mode=story&entry=kong-rewrite&idea=${idea}`);
 const kongBook={...book,id:"kong",entry_scene:3,characters:[{id:"kong",name:"孔乙己",initial:"孔",description:"读书人",color:"#787"}],scenes:book.scenes.map(s=>({...s,characters:["kong"]}))};
 const kongBranch={...branch,id:"kong-demo",anchor:2,scene:kongBook.scenes[1]};
 vi.mocked(request).mockImplementation(async(path)=>{
  if(path==="/book")return {book:kongBook,reading:{status:"not_started",insights:[]},mode:"agent",model_connected:true};
  if(path==="/books")return [];
  if(path==="/branches")return kongBranch;
  if(path.includes("/snapshot"))return {facts:[],memories:[]};
  throw Error("unexpected call");
 });
 render(<ReadingPage/>);
 expect(await screen.findByRole("textbox",{name:"对人物说的话"})).toHaveValue(kongRewrites[(idea||"letters") as keyof typeof kongRewrites].message);
 expect(vi.mocked(request).mock.calls.filter(([p])=>p==="/branches")).toEqual([["/branches",{scene:2}]]);
 expect(vi.mocked(request).mock.calls.some(([p])=>p.endsWith("/turn"))).toBe(false);
 expect(new URLSearchParams(location.search).get("branch")).toBe("kong-demo");
 expect(new URLSearchParams(location.search).has("entry")).toBe(false);
});
it("locates a homepage companion question without sending it or enabling full-book spoilers",async()=>{
 window.history.replaceState({},"","/reading?book=kong&question=coat");
 const position={paragraph:1,furthest:1,full:false,finished:false,bookmarks:[]};
 const state={revision:0,version:"v1",position,turns:[],notes:[]};
 const kong={id:"kong",name:"孔乙己",initial:"孔",description:"读书人",color:"#787"};
 vi.mocked(request).mockImplementation(async(path,body)=>{
  if(path==="/book")return {book:{...book,id:"kong",characters:[kong]},reading:{status:"not_started"},mode:"agent",model_connected:true};
  if(path==="/books")return [];
  if(path==="/lesson")return {id:"kong",title:"孔乙己",views:[],moments:[],sources:[]};
  if(path==="/companion")return {state,paragraphs:[{id:1,text:"酒店",start_byte:0,end_byte:6},{id:3,text:"站着喝酒而穿长衫",start_byte:6,end_byte:33}],characters:{1:[],3:[kong]},research_available:false};
  if(path==="/companion/position")return {...state,revision:1,position:(body as {position:typeof position}).position};
  if(path.startsWith("/translations"))return {};
  throw Error("unexpected call");
 });
 render(<ReadingPage/>);
 expect(await screen.findByRole("textbox",{name:"伴读问题"})).toHaveValue("你为什么还穿着这件长衫？");
 expect(screen.getByRole("combobox",{name:"伴读对象"})).toHaveValue("kong");
 expect(request).toHaveBeenCalledWith("/companion/position",{position:{...position,paragraph:3},revision:0});
 expect(vi.mocked(request).mock.calls.some(([p])=>p.includes("/chat")||p==="/branches")).toBe(false);
});
it("routes a read-only essay away from a forged story URL without assuming scenes",async()=>{
 window.history.replaceState({},"","/reading?book=beiying&mode=story");
 vi.mocked(request).mockImplementation(async(path)=>{
  if(path==="/book")return {book:{...book,id:"beiying",title:"背影",read_only:true,scenes:[],characters:[]},reading:{status:"not_started"},mode:"agent",model_connected:true};
  if(path==="/books")return [];
  if(path==="/lesson")return {id:"beiying",title:"那一刻，与后来",intro:"",views:[],moments:[{title:"送行",paragraph:3,text:"散文原文",then:"当时",now:"后来",question:"解释"}],sources:[]};
  if(path==="/companion")return {state:{revision:0,notes:[],turns:[],position:{paragraph:1,furthest:0,finished:false,full:false,bookmarks:[]}},paragraphs:[{id:1,text:"散文原文",start_byte:0,end_byte:12}],research_available:false};
  throw Error("unexpected");
 });
 render(<ReadingPage/>);expect(await screen.findByText("散文原文")).toBeInTheDocument();
 expect(screen.queryByRole("button",{name:"走进这个场景"})).not.toBeInTheDocument();expect(request).not.toHaveBeenCalledWith("/branches",expect.anything());
});
it("persists a signature and reports success only after server confirmation",async()=>{
 const ended:Branch={...branch,ending:{title:"另一种礼物",story:"新的故事",resolution:"落幕",new_timeline:[],contributions:[],revision:1,created_at:"2026-09-09"}};
 const updated={...ended,signature:"小雨"};const onSigned=vi.fn();
 vi.mocked(request).mockResolvedValue(updated);
 render(<StoryEndingCard book={book} branch={ended} onSigned={onSigned}/>);
 fireEvent.change(screen.getByRole("textbox",{name:"留下你的署名"}),{target:{value:"小雨"}});
 fireEvent.click(screen.getByRole("button",{name:"确认署名"}));
 await waitFor(()=>expect(onSigned).toHaveBeenCalledWith(updated));
 expect(request).toHaveBeenCalledWith("/branches/branch1/signature",{name:"小雨",revision:1});
});
it("shows shelf as the default entry without generating a story",async()=>{
 window.history.replaceState({},"","/reading");
 render(<ReadingPage/>);
 expect(await screen.findByRole("heading",{name:/选择一篇开始阅读/})).toBeInTheDocument();
 expect(screen.queryByRole("button",{name:"走进这个场景"})).not.toBeInTheDocument();
 expect(request).not.toHaveBeenCalledWith("/branches",expect.anything());
});
it("renders the short-story shelf with independent book links",async()=>{
 const prior=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(path,body)=>{
  if(path==="/books")return [{id:"necklace",title:"项链",theme:"信息差"},{id:"magi",title:"麦琪的礼物",theme:"礼物与关心"},{id:"leaf",title:"最后一片叶子",theme:"照护"},{id:"paw",title:"猴爪",theme:"未知"},{id:"kong",title:"孔乙己",theme:"尊严"}];
  return prior(path,body);
 });
 render(<ReadingPage/>);
 expect(await screen.findByRole("link",{name:/麦琪的礼物/})).toHaveAttribute("href","/reading?book=magi");
 expect(screen.getByRole("link",{name:/项链 信息差/})).toHaveAttribute("aria-current","page");
 expect(screen.getByRole("navigation",{name:"选择短篇"}).querySelectorAll("a")).toHaveLength(5);
});
it("uses another book's entry scene, count and storage key without Necklace UI",async()=>{
 const other:Book={...book,id:"magi",title:"麦琪的礼物",author:"欧·亨利",entry_scene:1,scenes:book.scenes.slice(0,6),characters:[{id:"della",name:"德拉",initial:"D",description:"准备礼物",color:"#987"}]};
 other.scenes=other.scenes.map(s=>({...s,summary:"礼物的选择",characters:["della"]}));
 const otherBranch={...branch,id:"magi-branch",anchor:1,scene:other.scenes[0]};
 vi.mocked(request).mockImplementation(async(path)=>{
  if(path==="/book")return {book:other,reading:{status:"not_started",completed:0,insights:[]},mode:"agent",model_connected:true};
  if(path==="/books")return [];
  if(path==="/branches")return otherBranch;
  if(path.includes("/snapshot"))return {facts:[],memories:[]};
  throw Error("unexpected");
 });
 render(<ReadingPage/>);
 expect(await screen.findByText("短篇小说 · 6 个故事节点")).toBeInTheDocument();
 expect(screen.getByLabelText("麦琪的礼物书封")).toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"走进这个场景"}));
 await screen.findByRole("textbox",{name:"对人物说的话"});
 expect(request).toHaveBeenCalledWith("/branches",{scene:1});
 expect(localStorage.getItem("reading-magi-branch")).toBe("magi-branch");
 expect(localStorage.getItem("reading-necklace-branch")).toBeNull();
 expect(screen.queryByText("试着劝她坦白")).not.toBeInTheDocument();
});
it("sends influence and explicit finish, then displays a locked ending card",async()=>{
 const prior=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(path,body)=>{
  if(path.endsWith("/turn"))return {...branch,influence:9,revision:2,ending:{title:"坦白后的清晨",story:"他们终于把话说开。故事在此落幕。",resolution:"核心冲突已经收束。",new_timeline:["遗失项链","说出真相"],contributions:[],revision:2,created_at:"2026-09-08"}};
  return prior(path,body);
 });
 render(<ReadingPage/>);fireEvent.click(await screen.findByRole("button",{name:"走进这个场景"}));
 fireEvent.change(await screen.findByRole("slider",{name:/来访者影响力/}),{target:{value:"9"}});
 fireEvent.click(screen.getByRole("button",{name:"让故事走向结局"}));
 expect(await screen.findByRole("article",{name:"故事结局卡"})).toBeInTheDocument();
 expect(screen.getByText("坦白后的清晨")).toBeInTheDocument();
 expect(screen.getByRole("button",{name:"继续下一场景"})).toBeDisabled();
 expect(screen.queryByRole("textbox",{name:"对人物说的话"})).not.toBeInTheDocument();
 expect(request).toHaveBeenCalledWith("/branches/branch1/turn",expect.objectContaining({finish:true,advance:false,influence:9}));
 expect(screen.getByText("原著的故事线")).toBeInTheDocument();
 expect(screen.getByText("改写后的故事线")).toBeInTheDocument();
});
it("renders editorial scenes without claiming a completed model read",async()=>{render(<ReadingPage/>);expect(await screen.findByRole("button",{name:"走进这个场景"})).toBeEnabled();expect(screen.getByText(/场景由编者预先整理/)).toBeInTheDocument();expect(screen.queryByText("场景7")).not.toBeInTheDocument();});
it("enters a scene, loads only selected perspective, and preserves failed input",async()=>{render(<ReadingPage/>);fireEvent.click(await screen.findByRole("button",{name:"走进这个场景"}));expect(await screen.findByText("朋友还不知道遗失")).toBeInTheDocument();const input=screen.getByRole("textbox",{name:"对人物说的话"});fireEvent.change(input,{target:{value:"我们去坦白吧"}});fireEvent.click(screen.getByRole("button",{name:"发送"}));expect(await screen.findByText("测试：网络中断，本轮未完成")).toBeInTheDocument();expect(input).toHaveValue("我们去坦白吧");expect(localStorage.getItem("reading-necklace-branch")).toBe("branch1");fireEvent.click(screen.getByRole("button",{name:"L 路瓦栽"}));await waitFor(()=>expect(request).toHaveBeenCalledWith("/branches/branch1/snapshot?character=loisel"));});
it("shows a recoverable error instead of a blank page when boot fails",async()=>{vi.mocked(request).mockRejectedValue(Error("服务未启动"));render(<ReadingPage/>);expect(await screen.findByText("服务未启动")).toBeInTheDocument();expect(screen.getByRole("button",{name:"重新连接"})).toBeEnabled();});
