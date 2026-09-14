import {useRef} from "react";
import {act,cleanup,fireEvent,render} from "@testing-library/react";
import {afterEach,expect,it,vi} from "vitest";
import {useReadingFollow,visibleReadingParagraph} from "./useReadingFollow";

function Fixture({enabled=true,onParagraph}:{enabled?:boolean;onParagraph:(id:number)=>void}){
 const paper=useRef<HTMLElement>(null);useReadingFollow(paper,enabled,"test",onParagraph);
 return <><main ref={paper}><section data-paragraph="1"/><section data-paragraph="2"/></main><aside data-testid="chat"/></>;
}
function geometry(container:HTMLElement){
 vi.stubGlobal("innerHeight",900);
 const sections=container.querySelectorAll<HTMLElement>("section");
 vi.spyOn(sections[0],"getBoundingClientRect").mockReturnValue({top:-300,bottom:100,height:400} as DOMRect);
 vi.spyOn(sections[1],"getBoundingClientRect").mockReturnValue({top:110,bottom:1100,height:990} as DOMRect);
}
afterEach(()=>{cleanup();vi.useRealTimers();vi.restoreAllMocks();vi.unstubAllGlobals();});
it("keeps a long paragraph under the reading anchor and ignores offscreen text",()=>{
 const {container}=render(<Fixture enabled={false} onParagraph={vi.fn()}/>);geometry(container);
 expect(visibleReadingParagraph(container.querySelector("main")!)).toBe(2);
 vi.mocked(container.querySelectorAll("section")[1].getBoundingClientRect).mockReturnValue({top:950,bottom:1100,height:150} as DOMRect);
 expect(visibleReadingParagraph(container.querySelector("main")!)).toBe(1);
});
it("debounces real paper scrolling and ignores chat scrolling",()=>{
 vi.useFakeTimers();const onParagraph=vi.fn();const {container,getByTestId}=render(<Fixture onParagraph={onParagraph}/>);geometry(container);
 act(()=>vi.advanceTimersByTime(450));onParagraph.mockClear();
 fireEvent.scroll(getByTestId("chat"));act(()=>vi.advanceTimersByTime(500));expect(onParagraph).not.toHaveBeenCalled();
 fireEvent.scroll(window);act(()=>vi.advanceTimersByTime(300));fireEvent.scroll(window);
 act(()=>vi.advanceTimersByTime(300));expect(onParagraph).not.toHaveBeenCalled();
 act(()=>vi.advanceTimersByTime(150));expect(onParagraph).toHaveBeenCalledExactlyOnceWith(2);
});
it("cancels pending position changes while pinned or composing and on unmount",()=>{
 vi.useFakeTimers();const onParagraph=vi.fn();const {container,rerender,unmount}=render(<Fixture onParagraph={onParagraph}/>);geometry(container);
 fireEvent.scroll(window);rerender(<Fixture enabled={false} onParagraph={onParagraph}/>);
 act(()=>vi.advanceTimersByTime(600));expect(onParagraph).not.toHaveBeenCalled();
 rerender(<Fixture enabled onParagraph={onParagraph}/>);act(()=>vi.advanceTimersByTime(450));expect(onParagraph).toHaveBeenCalledWith(2);
 onParagraph.mockClear();fireEvent.scroll(window);unmount();act(()=>vi.advanceTimersByTime(600));expect(onParagraph).not.toHaveBeenCalled();
});
