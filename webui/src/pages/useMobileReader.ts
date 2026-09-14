import {useEffect,useRef,useState,type CSSProperties,type RefObject} from "react";

export function useReaderViewport(){
 const measure=()=>({small:window.innerWidth<=760,height:window.visualViewport?.height??window.innerHeight,top:window.visualViewport?.offsetTop??0});
 const [viewport,setViewport]=useState(measure);
 useEffect(()=>{
  const update=()=>setViewport(measure());
  window.addEventListener("resize",update);
  window.visualViewport?.addEventListener("resize",update);
  window.visualViewport?.addEventListener("scroll",update);
  return()=>{window.removeEventListener("resize",update);window.visualViewport?.removeEventListener("resize",update);window.visualViewport?.removeEventListener("scroll",update);};
 },[]);
 return {small:viewport.small,style:{"--reader-viewport-height":`${viewport.height}px`,"--reader-viewport-top":`${viewport.top}px`} as CSSProperties};
}

export function useMobileReaderPanel(ref:RefObject<HTMLElement|null>,active:boolean,close:()=>void){
 const closeRef=useRef(close);closeRef.current=close;
 useEffect(()=>{
  if(!active||!ref.current)return;
  const previous=document.activeElement as HTMLElement|null,overflow=document.body.style.overflow;
  document.body.style.overflow="hidden";
  ref.current.querySelector<HTMLButtonElement>(".cr-mobile-close")?.focus({preventScroll:true});
  const keys=(event:KeyboardEvent)=>{
   if(document.querySelector(".cr-backdrop"))return;
   if(event.key==="Escape"){event.preventDefault();closeRef.current();return;}
   if(event.key!=="Tab")return;
   const controls=Array.from(ref.current?.querySelectorAll<HTMLElement>('button:not(:disabled),a[href],select:not(:disabled),textarea:not(:disabled),input:not(:disabled),[tabindex="0"]')??[]).filter(el=>el.getClientRects().length>0);
   const first=controls[0],last=controls.at(-1);
   if(event.shiftKey&&(document.activeElement===first||!ref.current?.contains(document.activeElement))){event.preventDefault();last?.focus();}
   else if(!event.shiftKey&&(document.activeElement===last||!ref.current?.contains(document.activeElement))){event.preventDefault();first?.focus();}
  };
  document.addEventListener("keydown",keys);
  return()=>{
   document.body.style.overflow=overflow;document.removeEventListener("keydown",keys);
   const returnTo=previous?.isConnected?previous:document.querySelector<HTMLElement>('[aria-label="打开伴读"]');
   returnTo?.focus({preventScroll:true});
  };
 },[active,ref]);
}
