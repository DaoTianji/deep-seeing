import{c as a,u as o,a as u,b as j,t as p,j as s,L as i,C as y,T as m,d as k}from"./index-Fikgd6vI.js";/**
 * @license lucide-react v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const v=a("ArrowLeft",[["path",{d:"m12 19-7-7 7-7",key:"1l729n"}],["path",{d:"M19 12H5",key:"x3x0zl"}]]);/**
 * @license lucide-react v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const g=a("Cpu",[["rect",{width:"16",height:"16",x:"4",y:"4",rx:"2",key:"14l7u7"}],["rect",{width:"6",height:"6",x:"9",y:"9",rx:"1",key:"5aljv4"}],["path",{d:"M15 2v2",key:"13l42r"}],["path",{d:"M15 20v2",key:"15mkzm"}],["path",{d:"M2 15h2",key:"1gxd5l"}],["path",{d:"M2 9h2",key:"1bbxkp"}],["path",{d:"M20 15h2",key:"19e6y8"}],["path",{d:"M20 9h2",key:"19tzq7"}],["path",{d:"M9 2v2",key:"165o2o"}],["path",{d:"M9 20v2",key:"i2bqo8"}]]);/**
 * @license lucide-react v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const M=a("HeartPulse",[["path",{d:"M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z",key:"c3ymky"}],["path",{d:"M3.22 12H9.5l.5-1 2 4.5 2-7 1.5 3.5h5.27",key:"1uw2ng"}]]);function T(){var c,l;const{turnId:r=""}=o(),{getTurn:d}=u(),n=d(r),t=j({queryKey:["turn",r],queryFn:()=>k.turn(r),enabled:!n&&!!r,retry:!1}),e=n||((c=t.data)!=null&&c.turn?p(t.data.turn):void 0);if(t.isLoading&&!e)return s.jsx("div",{className:"page-state",children:"正在找到这一轮留下的痕迹…"});if(!e)return s.jsxs("div",{className:"page-state error",children:[s.jsx("h1",{children:"没有找到这一轮"}),s.jsx("p",{children:"它可能来自旧版本，尚未拥有稳定的 Turn ID。"}),s.jsx(i,{to:"/",children:"回到对话"})]});const h=Math.max(...e.events.map(x=>x.offset),0);return s.jsxs("div",{className:"turn-page",children:[s.jsxs("header",{className:"turn-hero",children:[s.jsxs(i,{to:"/",className:"back-link",children:[s.jsx(v,{size:16}),"回到对话"]}),s.jsxs("div",{children:[s.jsxs("span",{className:"eyebrow",children:["TURN · ",e.id.slice(0,12)]}),s.jsx("h1",{children:e.userText||"一次没有标题的交谈"}),s.jsx("p",{children:"从公开事件中回看：哪些背景被看见，哪些记忆成为证据，注意力如何移动。"})]}),s.jsxs("div",{className:"turn-metrics",children:[s.jsxs("article",{children:[s.jsx(y,{}),s.jsxs("span",{children:[s.jsxs("strong",{children:[(h/1e9).toFixed(1),"s"]}),s.jsx("small",{children:"公开过程"})]})]}),s.jsxs("article",{children:[s.jsx(g,{}),s.jsxs("span",{children:[s.jsx("strong",{children:e.events.length}),s.jsx("small",{children:"结构事件"})]})]}),s.jsxs("article",{children:[s.jsx(M,{}),s.jsxs("span",{children:[s.jsx("strong",{children:((l=e.health)==null?void 0:l.status)||"完成"}),s.jsx("small",{children:"运行状态"})]})]})]})]}),s.jsx(m,{turn:e}),s.jsxs("section",{className:"turn-answer",children:[s.jsx("span",{className:"eyebrow",children:"ANSWER"}),s.jsx("h2",{children:"最终回答"}),s.jsx("div",{children:e.answer||"回答正文没有写入公开轨迹。请从当前会话中查看。"})]})]})}export{T as TurnPage};
