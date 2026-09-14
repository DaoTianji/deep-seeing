import {useEffect, useRef, type RefObject} from "react";

// The paragraph around the upper third of the visible paper is the reading
// anchor. A long paragraph remains selected until the anchor passes its end.
export function visibleReadingParagraph(paper: HTMLElement): number | undefined {
 let top=0,bottom=window.innerHeight;
 for(let parent=paper.parentElement;parent;parent=parent.parentElement){
  if(/auto|scroll|hidden|clip/.test(getComputedStyle(parent).overflowY)){
   const rect=parent.getBoundingClientRect();top=Math.max(top,rect.top);bottom=Math.min(bottom,rect.bottom);
  }
 }
 if(bottom<=top)return;
 const toolbar=paper.querySelector(".cr-selection-bar")?.getBoundingClientRect();
 if(toolbar&&toolbar.top<=top+4&&toolbar.bottom>top)top=toolbar.bottom;
 const anchor=top+(bottom-top)*.3;
 let candidate:number|undefined,distance=Infinity;
 for(const paragraph of paper.querySelectorAll<HTMLElement>("[data-paragraph]")){
  const rect=paragraph.getBoundingClientRect();
  if(rect.height<=0||rect.bottom<=top||rect.top>=bottom)continue;
  const next=anchor<rect.top?rect.top-anchor:anchor>rect.bottom?anchor-rect.bottom:0;
  if(next<distance){distance=next;candidate=Number(paragraph.dataset.paragraph);}
 }
 return candidate;
}

export function useReadingFollow(paper:RefObject<HTMLElement|null>,enabled:boolean,bookID:string,onParagraph:(id:number)=>void){
 const callback=useRef(onParagraph);callback.current=onParagraph;
 useEffect(()=>{
  if(!enabled)return;
  let timer:ReturnType<typeof setTimeout>;
  const schedule=()=>{
   clearTimeout(timer);
   timer=setTimeout(()=>{
    if(document.visibilityState==="hidden"||!paper.current)return;
    const id=visibleReadingParagraph(paper.current);
    if(id)callback.current(id);
   },450);
  };
  const scroll=(event:Event)=>{
   const target=event.target;
   // Ignore scrolling in chat, notes, selects and translation dialogs.
   if(target===document||target===window||(target instanceof Window)||(target instanceof HTMLElement&&paper.current&&target.contains(paper.current)))schedule();
  };
  window.addEventListener("scroll",scroll,true);
  window.addEventListener("resize",schedule);
  schedule();
  return()=>{clearTimeout(timer);window.removeEventListener("scroll",scroll,true);window.removeEventListener("resize",schedule);};
 },[enabled,bookID,paper]);
}
