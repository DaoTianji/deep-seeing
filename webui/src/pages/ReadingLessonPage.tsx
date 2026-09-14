import {useEffect,useState} from "react";
import {request,type Book} from "./reading-api";
import {CompanionReader} from "./CompanionReader";

export type ReadingLesson={labels?:string[];classical?:boolean;id:string;title:string;subtitle:string;intro:string;views:{id:string;title:string;lens:string;question:string}[];moments:{steps?:string[];title:string;paragraph:number;text:string;then:string;now:string;question:string}[];sources:{id:string;title:string;summary:string;url:string}[]};

// Optional editorial material must not replace or block the complete original.
export function ReadingLessonPage({book,onStory,initialIntent}:{book:Book;onStory:()=>void;initialIntent?:{speaker:string;message:string;paragraph?:number}}) {
 const [lesson,setLesson]=useState<ReadingLesson>();
 const [error,setError]=useState("");
 useEffect(()=>{let live=true;request<ReadingLesson>("/lesson").then(l=>{if(live)setLesson(l);}).catch(()=>{if(live)setError("专题导读暂未加载，完整原文与伴读仍可使用。");});return()=>{live=false;};},[book.id]);
 return <CompanionReader book={book} onStory={onStory} initialIntent={initialIntent} continuous lesson={lesson} lessonError={error}/>;
}
