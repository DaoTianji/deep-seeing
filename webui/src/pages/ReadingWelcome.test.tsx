import {cleanup,render,screen} from "@testing-library/react";
import {afterEach,expect,it,vi} from "vitest";
import {MemoryRouter} from "react-router-dom";
import {ReadingWelcome} from "./ReadingWelcome";
import {readingEntry,kongRewriteLink,kongRewrites,rewriteEntry} from "./reading-entry";
import {App} from "../App";

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
it("explains the product before login without loading personal records or calling models",()=>{
 const fetcher=vi.fn();vi.stubGlobal("fetch",fetcher);
 render(<MemoryRouter initialEntries={["/reading"]}><App/></MemoryRouter>);
 expect(screen.getByRole("heading",{level:1})).toHaveTextContent("读懂故事");
 expect(screen.getByText("孔乙己是站着喝酒而穿长衫的唯一的人。")).toBeInTheDocument();
 expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
 expect(fetcher).not.toHaveBeenCalled();
});
it("links each question directly to its own curated companion intent",()=>{
 render(<ReadingWelcome/>);
 for(const [key,title] of [["coat","你为什么还穿着这件长衫"],["background","长衫和短衣"],["laughter","为什么大家都在笑"]]){
  const link=screen.getByRole("link",{name:new RegExp(title)});
  expect(link).toHaveAttribute("href",`/reading?book=kong&question=${key}`);
  expect(readingEntry(link.getAttribute("href")!.split("?")[1])?.paragraph).toBe(3);
 }
 expect(readingEntry("book=kong&question=coat")?.speaker).toBe("kong");
 expect(readingEntry("book=kong&question=background")?.speaker).toBe("an");
});
it("offers a specific rewrite scene, not a fabricated finished demonstration",()=>{
 render(<ReadingWelcome/>);
 for(const [key,idea] of Object.entries(kongRewrites)){
  const link=screen.getByRole("link",{name:new RegExp(idea.title)});
  expect(link).toHaveAttribute("href",`${kongRewriteLink}&idea=${key}`);
  expect(rewriteEntry(link.getAttribute("href")!.split("?")[1])?.message).toBe(idea.message);
 }
 expect(screen.getByText(/这是玩法说明，不是生成示例/)).toBeInTheDocument();
 expect(screen.getByText(/联网查询状态/)).toBeInTheDocument();
 expect(screen.getByRole("link",{name:/浏览其他作品/})).toHaveAttribute("href","/reading?view=shelf");
});
it("keeps rewrite starters curated and only in the intended story entry",()=>{
 expect(rewriteEntry("book=kong&mode=story&entry=kong-rewrite")?.message).toBe(kongRewrites.letters.message);
 expect(rewriteEntry("book=kong&mode=story&entry=kong-rewrite&idea=__proto__")).toBe(kongRewrites.letters);
 expect(rewriteEntry("book=kong&mode=story&entry=kong-rewrite&idea=arbitrary-prompt")).toBe(kongRewrites.letters);
 expect(rewriteEntry("book=necklace&mode=story&entry=kong-rewrite&idea=teacher")).toBeUndefined();
 expect(rewriteEntry("book=kong&entry=kong-rewrite&idea=teacher")).toBeUndefined();
});
it("does not accept arbitrary prompts or transfer Kong intents to other works",()=>{
 expect(readingEntry("book=kong&question=unknown")).toBeUndefined();
 expect(readingEntry("book=kong&question=__proto__")).toBeUndefined();
 expect(readingEntry("book=necklace&question=coat")).toBeUndefined();
});
