// Only curated entry points are accepted; URL text never becomes a model prompt.
export const kongQuestions = {
 coat: {speaker:"kong", message:"你为什么还穿着这件长衫？", paragraph:3},
 background: {speaker:"an", message:"长衫和短衣在这里意味着什么？请区分原文依据与背景资料。", paragraph:3},
 laughter: {speaker:"an", message:"为什么大家都在笑？这些笑声让我们看见了怎样的人物处境？", paragraph:3},
};
export function readingEntry(search:string) {
 const params=new URLSearchParams(search);
 if(params.get("book")!=="kong")return undefined;
 const key=params.get("question");
 return key&&Object.hasOwn(kongQuestions,key)?kongQuestions[key as keyof typeof kongQuestions]:undefined;
}
export const kongRewriteLink="/reading?book=kong&mode=story&entry=kong-rewrite";
export const kongRewrites = {
 teacher:{title:"请孔乙己当先生",description:"他总想教人识字。这回真有人请他教，他会怎么做？",message:"孔先生，我想请你教几个还不识字的孩子，束脩可以商量。不过他们连名字都写不全，咱们能不能先从名字教起？"},
 letters:{title:"和他合伙摆个代笔摊",description:"他写字，你揽客。靠这门手艺，能不能换一种活法？",message:"孔先生，你字写得好，不如咱们合伙摆个代笔摊？你写家书，我去揽客，挣了钱分账。第一封信是替人报平安，可得让家里人听得懂，不能满篇之乎者也。你愿不愿意试试？"},
 challenge:{title:"这次，让他来出题",description:"酒客拿功名取笑他。要是换他考考酒客呢？",message:"孔先生，他们总拿没考中秀才笑你。不如今天换你出一道题，考考在场的人？别考茴字的写法，就考一道识字能派上用场的题。我来请掌柜做个见证。"},
};
export function rewriteEntry(search:string){
 const params=new URLSearchParams(search);
 if(params.get("book")!=="kong"||params.get("mode")!=="story"||params.get("entry")!=="kong-rewrite")return undefined;
 const key=params.get("idea");
 return key&&Object.hasOwn(kongRewrites,key)?kongRewrites[key as keyof typeof kongRewrites]:kongRewrites.letters;
}
