import {cleanup,render,screen} from "@testing-library/react";
import {afterEach,expect,it} from "vitest";
import {ReadingShelf} from "./ReadingShelf";
import type {BookSummary} from "./reading-api";
afterEach(cleanup);
it("shows physical book covers and links to saved journeys",()=>{
 const books:BookSummary[]=[{id:"magi",title:"麦琪的礼物",author:"欧·亨利",subtitle:"先听见彼此",theme:"爱",scene_count:6,entry_scene:1,records:[{id:"one",title:"迟到的礼物",scene:"家",turns:3,completed:true,signature:"小雨",updated_at:"2026-09-09T01:00:00Z"},{id:"two",title:"等待",scene:"家",turns:1,completed:false,updated_at:"2026-09-08T01:00:00Z"}]}];
 render(<ReadingShelf books={books} error=""/>);
 expect(screen.getByRole("link",{name:"翻开《麦琪的礼物》"})).toHaveAttribute("href","/reading?book=magi");
 expect(screen.getByRole("link",{name:/迟到的礼物/})).toHaveAttribute("href","/reading?book=magi&branch=one");
 expect(screen.getByText(/来访者 小雨/)).toBeInTheDocument();
 expect(screen.getByText("进行中")).toBeInTheDocument();
});
it("does not pretend new visitors already have records",()=>{
 render(<ReadingShelf books={[]} error=""/>);
 expect(screen.getByText("暂无故事记录")).toBeInTheDocument();
 expect(screen.getByText("书中见")).toBeInTheDocument();
 expect(screen.getByRole("heading",{name:"选择一篇开始阅读"})).toBeInTheDocument();
 expect(screen.getByRole("heading",{name:"我的故事记录"})).toBeInTheDocument();
});
