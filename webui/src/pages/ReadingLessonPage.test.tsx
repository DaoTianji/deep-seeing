import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {ReadingLessonPage} from "./ReadingLessonPage";
import {request,type Book} from "./reading-api";
import type {ReaderState} from "./CompanionReader";
vi.mock("./reading-api",()=>({request:vi.fn()}));
const book:Book={id:"kong",title:"孔乙己",author:"鲁迅",subtitle:"",source_url:"https://example.org",source_note:"公版原文",version:"1",scenes:[],characters:[{id:"boy",name:"小伙计",initial:"小",description:"",color:"#123"},{id:"keeper",name:"掌柜",initial:"掌",description:"",color:"#123"}],evidence:[]};
const lesson={id:"kong",title:"同一阵笑声，三种处境。",subtitle:"",intro:"试着换个位置。",views:[{id:"boy",title:"小伙计",lens:"站在柜台内",question:"为什么会笑？"},{id:"keeper",title:"掌柜",lens:"生意的位置",question:"你怎样看客人？"}],moments:[{title:"站着喝酒",paragraph:3,text:"原文三",then:"现场",now:"回望",question:"请解释这一段"},{title:"另一个细节",paragraph:4,text:"原文四",then:"当时的我",now:"后来的声音",question:"解释声调"}],sources:[{id:"curated-test",title:"已整理背景 · 作者",summary:"预先整理的公开资料",url:"https://example.org"}]};
let state:ReaderState;
beforeEach(()=>{
 window.history.replaceState({},"","/reading?book=kong");
 state={revision:0,version:"1",position:{paragraph:1,furthest:0,finished:false,full:false,bookmarks:[]},turns:[],notes:[]};
 vi.mocked(request).mockReset().mockImplementation(async(path,body)=>{
  if(path==="/lesson")return lesson;
  if(path==="/companion")return {state,paragraphs:[1,2,3,4].map(id=>({id,text:`原文${id}`,start_byte:0,end_byte:10})),research_available:false};
  if(path==="/companion/position"){state={...state,revision:state.revision+1,position:(body as {position:ReaderState["position"]}).position};return state;}
  if(path==="/companion/notes"){const b=body as {paragraph:number;text:string};state={...state,revision:state.revision+1,notes:[...state.notes,{...b,id:"n1"}]};return state;}
  throw Error("unexpected request "+path);
 });
 vi.stubGlobal("fetch",vi.fn());
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
it("labels the complete selected text honestly and offers argument exploration, not a story",async()=>{
 const prior=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(p,b)=>p==="/lesson"?{...lesson,views:[],classical:true,labels:["论证拆解","提出反例"],moments:[{...lesson.moments[0],steps:["比喻","关系","结论"]}]}:prior(p,b));
 render(<ReadingLessonPage book={{...book,id:"quanxue",title:"劝学（节选）",language:"zh",read_only:true,text_scope:"课文节选",characters:[]}} onStory={vi.fn()}/>);
 expect(await screen.findByText("课文节选")).toBeInTheDocument();
 expect(screen.getByLabelText("原文结尾")).toHaveTextContent("选文结束");
 expect(screen.queryByRole("button",{name:"人物与场景"})).not.toBeInTheDocument();
 expect(screen.queryByRole("button",{name:"翻译这段"})).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"专题导读"}));
 fireEvent.click(screen.getByRole("button",{name:/§ 3/}));
 expect(await screen.findByLabelText("原文关系线索")).toHaveTextContent("比喻关系结论");
 fireEvent.click(screen.getByRole("button",{name:"提出反例"}));
 fireEvent.click(screen.getByRole("button",{name:"请安解释字词与句意"}));
 await waitFor(()=>expect((screen.getByLabelText("伴读问题") as HTMLTextAreaElement).value).toContain("古义、现代释义和解读"));
 expect(fetch).not.toHaveBeenCalled();
});
it("does not reveal unavailable perspectives or later scene titles",async()=>{
 const prior=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(p,b)=>p==="/companion"?{state,paragraphs:[1,2,3,4].map(id=>({id,text:`原文${id}`,start_byte:0,end_byte:10})),characters:{1:[],3:[]}}:prior(p,b));
 render(<ReadingLessonPage book={book} onStory={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"专题导读"}));
 expect(screen.queryByText("站着喝酒")).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:/§ 3/}));
 await waitFor(()=>expect(state.position.paragraph).toBe(3));
 expect(screen.queryByLabelText("专题人物")).not.toBeInTheDocument();
 expect(fetch).not.toHaveBeenCalled();
});

it("renders every original paragraph by default without replacing it with excerpts",async()=>{
 render(<ReadingLessonPage book={book} onStory={vi.fn()}/>);
 expect(await screen.findByText("原文1")).toBeInTheDocument();
 expect(screen.getByText("原文4")).toBeInTheDocument();
 expect(screen.getByRole("article",{name:"作品正文"}).querySelectorAll("[data-paragraph]")).toHaveLength(4);
 expect(screen.getByLabelText("原文结尾")).toHaveTextContent("全文结束");
 expect(screen.queryByText("原文三")).not.toBeInTheDocument();
 expect(fetch).not.toHaveBeenCalled();
 expect(request).not.toHaveBeenCalledWith("/companion/position",expect.anything());
 expect(state.position.full).toBe(false);expect(state.position.finished).toBe(false);
});
it("keeps the full original while opening a source-linked perspective without sending",async()=>{
 render(<ReadingLessonPage book={book} onStory={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"专题导读"}));
 fireEvent.click(screen.getByRole("button",{name:/§ 3/}));
 await waitFor(()=>expect(state.position.paragraph).toBe(3));
 fireEvent.change(screen.getByLabelText("专题人物"),{target:{value:"keeper"}});
 fireEvent.click(screen.getByRole("button",{name:"从掌柜的视角聊聊"}));
 await waitFor(()=>expect(screen.getByLabelText("伴读对象")).toHaveValue("keeper"));
 expect(screen.getByLabelText("伴读问题")).toHaveValue("你怎样看客人？");
 expect(screen.getByLabelText("伴读面板")).toHaveClass("mobile-open");
 expect(screen.getByText("原文1")).toBeInTheDocument();expect(screen.getByText("原文4")).toBeInTheDocument();
 expect(state.position.full).toBe(false);expect(fetch).not.toHaveBeenCalled();
});
it("retains saved understanding cards in the original reader",async()=>{
 state.notes=[{id:"old",paragraph:3,text:"[专题理解] 笑声也可能是一种排斥。"}];
 render(<ReadingLessonPage book={book} onStory={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"写笔记"}));
 expect(screen.getByText("[专题理解] 笑声也可能是一种排斥。")).toBeInTheDocument();
 expect(screen.getByText("原文1")).toBeInTheDocument();expect(screen.getByText("原文4")).toBeInTheDocument();
});
it("switches time lenses beside the full essay and prepares an honest disagreement",async()=>{
 const prior=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(p,b)=>p==="/lesson"?{...lesson,views:[]}:prior(p,b));
 render(<ReadingLessonPage book={{...book,id:"beiying",language:"zh",title:"背影",read_only:true,characters:[]}} onStory={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"专题导读"}));
 fireEvent.click(screen.getByRole("button",{name:/§ 4.*另一个细节/}));
 await waitFor(()=>expect(state.position.paragraph).toBe(4));
 fireEvent.click(screen.getByRole("button",{name:"后来回望"}));
 expect(screen.getByText("后来的声音")).toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"我有不同的理解"}));
 await waitFor(()=>expect((screen.getByLabelText("伴读问题") as HTMLTextAreaElement).value).toContain("后来的声音"));
 expect((screen.getByLabelText("伴读问题") as HTMLTextAreaElement).value).toContain("这是一种解读，不是原文");
 expect(screen.getByText("原文1")).toBeInTheDocument();expect(fetch).not.toHaveBeenCalled();
});
it("does not lose the original if the optional lesson fails",async()=>{
 const prior=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(p,b)=>{if(p==="/lesson")throw Error("offline");return prior(p,b);});
 render(<ReadingLessonPage book={book} onStory={vi.fn()}/>);
 expect(await screen.findByText("原文1")).toBeInTheDocument();expect(screen.getByText("原文4")).toBeInTheDocument();
 expect(await screen.findByRole("status")).toHaveTextContent("完整原文与伴读仍可使用");
});
it("does not claim a new discussion position when saving fails",async()=>{
 const prior=vi.mocked(request).getMockImplementation()!;
 vi.mocked(request).mockImplementation(async(p,b)=>{if(p==="/companion/position")throw Error("位置未保存");return prior(p,b);});
 render(<ReadingLessonPage book={book} onStory={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"专题导读"}));
 fireEvent.click(screen.getByRole("button",{name:/§ 3/}));
 expect(await screen.findByRole("alert")).toHaveTextContent("位置未保存");
 expect(state.position.paragraph).toBe(1);expect(fetch).not.toHaveBeenCalled();
});
