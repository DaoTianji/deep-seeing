import {useState} from "react";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {ReadingAccess} from "./ReadingAccess";
import {readerHeaders,readerCacheKey,READER_STORAGE_KEY,setActiveReader} from "./reading-session";

let current:{reader:{id:string;name:string}|null;legacy_available?:boolean};
const fetcher=vi.fn();
beforeEach(()=>{
 current={reader:null};setActiveReader("");localStorage.clear();window.history.replaceState({},"","/reading");
 fetcher.mockReset().mockImplementation(async(url:string,options:RequestInit)=>{
  if(url.endsWith("/logout")){current={reader:null};return new Response(JSON.stringify(current));}
  if(options.method==="POST"){const body=JSON.parse(options.body as string);current={reader:{id:"reader-a",name:body.name}};}
  return new Response(JSON.stringify(current));
 });vi.stubGlobal("fetch",fetcher);
});
afterEach(()=>{cleanup();setActiveReader("");vi.unstubAllGlobals();});
it("does not mount private pages before entering a name",async()=>{
 const probe=vi.fn(()=> <p>已有的阅读记录</p>);
 const Probe=probe;
 render(<ReadingAccess><Probe/></ReadingAccess>);
 expect(await screen.findByRole("textbox",{name:"名字或独特的笔名"})).toBeInTheDocument();
 expect(probe).not.toHaveBeenCalled();expect(screen.queryByText("已有的阅读记录")).not.toBeInTheDocument();
 expect(screen.getByText(/知道同一个名字的人也能进入/)).toBeInTheDocument();
 expect(screen.getByRole("button",{name:"开始阅读"})).toBeDisabled();
 expect(screen.getByText("书中见")).toBeInTheDocument();
 expect(screen.getByRole("heading",{name:"输入读者名字"})).toBeInTheDocument();
});
it("enters using only a name and makes profile-scoped requests possible",async()=>{
 render(<ReadingAccess><p>自己的书架</p></ReadingAccess>);
 fireEvent.change(await screen.findByRole("textbox",{name:"名字或独特的笔名"}),{target:{value:" 小林 "}});
 fireEvent.click(screen.getByRole("button",{name:"开始阅读"}));
 expect(await screen.findByText("自己的书架")).toBeInTheDocument();expect(screen.getByText("小林")).toBeInTheDocument();
 const sent=JSON.parse(fetcher.mock.calls.find(c=>c[1].method==="POST")![1].body);
 expect(sent).toEqual({name:"小林",claim_legacy:false});expect(readerHeaders()).toEqual({"X-Reading-Profile":"reader-a"});
 expect(readerCacheKey("magi")).toBe("reading-reader-a-magi-branch");
});
it("offers legacy record claim only with explicit opt-in",async()=>{
 current={reader:null,legacy_available:true};render(<ReadingAccess><p>书架</p></ReadingAccess>);
 const checkbox=await screen.findByRole("checkbox");expect(checkbox).not.toBeChecked();
 fireEvent.click(checkbox);fireEvent.change(screen.getByRole("textbox"),{target:{value:"旧书房"}});fireEvent.click(screen.getByRole("button",{name:"开始阅读"}));
 await screen.findByText("书架");expect(JSON.parse(fetcher.mock.calls[1][1].body).claim_legacy).toBe(true);
});
it("does not pretend a failed login worked",async()=>{
 render(<ReadingAccess><p>不能显示的记录</p></ReadingAccess>);await screen.findByRole("textbox");
 fetcher.mockResolvedValueOnce(new Response(JSON.stringify({error:"存档暂不可用"}),{status:503}));
 fireEvent.change(screen.getByRole("textbox"),{target:{value:"小林"}});fireEvent.click(screen.getByRole("button",{name:"开始阅读"}));
 expect(await screen.findByRole("alert")).toHaveTextContent("存档暂不可用");expect(screen.getByRole("textbox")).toHaveValue("小林");expect(screen.queryByText("不能显示的记录")).not.toBeInTheDocument();
});
it("logout unmounts records and clears the previous deep link",async()=>{
 current={reader:{id:"reader-a",name:"小林"}};window.history.replaceState({},"","/reading?book=magi&branch=old");
 render(<ReadingAccess><p>小林的手记</p></ReadingAccess>);await screen.findByText("小林的手记");
 fireEvent.click(screen.getByRole("button",{name:"退出 / 换个名字"}));await screen.findByRole("textbox");
 expect(screen.queryByText("小林的手记")).not.toBeInTheDocument();expect(window.location.search).toBe("");expect(readerHeaders()).toEqual({});
});
it("a different tab changing names replaces the entire private subtree",async()=>{
 current={reader:{id:"reader-a",name:"小林"}};
 function Private(){const [id]=useState(()=>readerHeaders()["X-Reading-Profile"]);return <p>记录属于 {id}</p>;}
 render(<ReadingAccess><Private/></ReadingAccess>);await screen.findByText("记录属于 reader-a");
 current={reader:{id:"reader-b",name:"小雨"}};fireEvent(window,new StorageEvent("storage",{key:READER_STORAGE_KEY,newValue:"changed"}));
 await waitFor(()=>expect(screen.getByText("记录属于 reader-b")).toBeInTheDocument());expect(screen.queryByText("记录属于 reader-a")).not.toBeInTheDocument();
 expect(readerCacheKey("magi")).toBe("reading-reader-b-magi-branch");
});
