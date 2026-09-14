import assert from 'node:assert/strict';
const base=process.argv[2];assert.ok(base?.startsWith('https://')||base?.startsWith('http://127.0.0.1:'),'explicit HTTPS or loopback target required');
const suffix=crypto.randomUUID().slice(0,8);let cookie='',id='';
async function call(path,body){const r=await fetch(base+'/api/story'+path,{method:body?'POST':'GET',headers:{'Content-Type':'application/json',...(cookie?{Cookie:cookie}:{}),...(id?{'X-Reading-Profile':id}:{})},body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(360000)});const set=r.headers.get('set-cookie');if(set)cookie=set.split(';')[0];const raw=await r.text();assert.ok(r.ok,`HTTP ${r.status}: ${raw.slice(0,200)}`);return r.headers.get('content-type')?.includes('ndjson')?raw.trim().split('\n').map(x=>JSON.parse(x)):JSON.parse(raw)}
async function login(name){id='';const s=await call('/session',{name});assert.equal(s.reader.name,name);id=s.reader.id;return id}
const name='发布验收-'+suffix;await login(name);const first=id;
const books=await call('/books');assert.equal(books.length,9);assert.equal(books.filter(b=>b.lesson_id).length,6);
// Read-only checks: bundled defaults must be complete without model calls.
const translationCounts={necklace:240,magi:153,leaf:174,paw:290,taohuayuan:29,quanxue:15,mulan:21};
for(const [book,total] of Object.entries(translationCounts)){
 const edition=await call('/companion/translation?book='+book+'&edition=default');
 assert.equal(edition.scope,'default',book);assert.equal(edition.total,total,book);assert.equal(edition.completed,total,book);
 assert.equal(edition.sentences.length,total,book);assert.ok(edition.sentences.every(s=>s.original&&s.translation?.trim()),book);
}
console.log(JSON.stringify({case:'bundled-default-translations',books:7,sentences:922,model_requested:false}));
const state=(await call('/companion?book=taohuayuan')).state;
const note='发布验收：桃花源的描写包含哪些生活细节？';await call('/companion/notes?book=taohuayuan',{paragraph:1,text:note,revision:state.revision});
await login('隔离验收-'+suffix);assert.notEqual(id,first);assert.equal((await call('/companion?book=taohuayuan')).state.notes.length,0);
await login(name);assert.equal(id,first);assert.ok((await call('/companion?book=taohuayuan')).state.notes.some(n=>n.text===note));
const model=process.env.READING_RELEASE_MODEL==='1';
if(model){const s=(await call('/companion?book=taohuayuan')).state;const events=await call('/companion/chat?book=taohuayuan',{request_id:crypto.randomUUID(),revision:s.revision,speaker:'an',paragraph:1,action:'chat',message:'请解释第一段中“缘溪行”的意思，引用原文，不要提前讲后续情节。',research:false,style:'fluent'});const e=events.find(e=>e.type==='error');assert.ok(!e,JSON.stringify(e));const result=events.find(e=>e.type==='result');assert.ok(result);assert.ok(result.data.turns.some(t=>t.reply&&t.evidence?.includes(1)));console.log(JSON.stringify({case:'live-companion',turns:result.data.turns.length,metrics:result.data.turns.at(-1).metrics}));}
if(process.env.READING_RELEASE_STORY==='1'){
 let b=await call('/branches?book=necklace',{scene:4});
 const turn=(opts)=>call('/branches/'+b.id+'/turn?book=necklace',{character:'loisel',message:'',request_id:crypto.randomUUID(),revision:b.revision,influence:10,...opts});
 b=await turn({message:'先不要急着举债买替代品。请先与妻子商量，一起告诉朋友项链遗失，问清实际价值，再商量如何弥补。这只是建议，并不表示这些事情已经发生。'});
 if(!b.ending)b=await turn({finish:true});assert.ok(b.ending?.story);assert.ok(b.ending.new_timeline.length>0);
 const signed=await call('/branches/'+b.id+'/signature?book=necklace',{name:'比赛发布验收',revision:b.revision});assert.equal(signed.signature,'比赛发布验收');
 const restored=await call('/branches/'+b.id+'?book=necklace');assert.equal(restored.signature,signed.signature);assert.equal(restored.ending.story,b.ending.story);
 console.log(JSON.stringify({case:'live-ending',branch:b.id,title:b.ending.title,story:b.ending.story,resolution:b.ending.resolution,contributions:b.ending.contributions,signature_restored:true}));
}
console.log(JSON.stringify({passed:true,books:9,lessons:6,note_restored:true,profile_isolation:true,model_requested:model,fixture:name}));
