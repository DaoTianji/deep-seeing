import ReactMarkdown from "react-markdown";
import {ArrowLeft,ArrowUpRight,BookOpen} from "lucide-react";
import sample from "./reading-example.md?raw";

export function ReadingExample(){return <main className="rs-room rs-example">
 <header className="rs-header"><a href="/reading"><ArrowLeft size={18}/> 返回书架</a><span><BookOpen size={23}/>书中见</span></header>
 <section className="rs-example-notice"><strong>预先生成的完整示例 · 含《项链》原著结局</strong><p>来自 2026 年 9 月的真实模型测试，不是现场生成，不会写入你的记录。测试者选择了 10 档影响力，建议先坦白、再讨论赔偿。</p><p>这是互动改写，不是莫泊桑原作。示例仍有局限：开头对遗失时地点的描述比原著更确定，付款时的资金来源也未细化。我们保留当次输出，供你判断效果。</p><a href="/reading?book=necklace&mode=story">自己尝试一个不同的选择 <ArrowUpRight size={16}/></a></section>
 <article className="rs-example-text"><ReactMarkdown>{sample}</ReactMarkdown></article>
 </main>}
