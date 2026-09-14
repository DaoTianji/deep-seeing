import type {Book,Branch} from "./reading-api";

export function endingMarkdown(book:Book, branch:Branch):string {
 const ending=branch.ending;
 if(!ending)return "";
 return [
  "# "+ending.title,"","《"+book.title+"》的一种新可能 · 分支 "+branch.id,"",
  ending.story,"","## 结局说明","",ending.resolution,"",
  "## 原著故事线（含结局）","",...book.scenes.map(s=>"- "+s.title+"："+s.summary),"",
  "## 这一次的故事","",...ending.new_timeline.map(s=>"- "+s),"",
  "## 你的参与带来了什么变化","",...(ending.contributions.length?ending.contributions.map(c=>"- "+c.text+"\n  你曾说："+(branch.turns.find(t=>t.id===c.turn_id)?.message||"")):["本次没有记录到由你的建议直接促成的变化。"]),"",
  "---","模型生成的互动分支，不是原著。影响力 "+(branch.influence||5)+"/10。",
  ...(branch.signature?["","来访者署名："+branch.signature,"原著作者："+book.author+" · 互动生成：安"]:[]),""
 ].join("\n");
}
