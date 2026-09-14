// Real-model smoke test against the isolated local reading service.
// Only fictional prompts; each invocation obtains its own visitor cookie.
import assert from 'node:assert/strict';
const base = process.argv[2] || 'http://127.0.0.1:3320';
assert.equal(new URL(base).hostname, '127.0.0.1', 'local prototype only');
let cookie = '', readerID = '';
async function call(path, body) {
 const response = await fetch(base + '/api/story' + path, {method:body ? 'POST':'GET',headers:{'Content-Type':'application/json',...(readerID ? {'X-Reading-Profile':readerID}:{}),...(cookie ? {Cookie:cookie}:{})},body:body ? JSON.stringify(body):undefined,signal:AbortSignal.timeout(330000)});
 const setCookie=response.headers.get('set-cookie');if(setCookie)cookie=setCookie.split(';')[0];
 const result=await response.json();if(!response.ok)throw Error(result.error);return result;
}
readerID = (await call('/session',{name:'结局验收-'+crypto.randomUUID().slice(0,12)})).reader.id;
function request(branch, opts){return {character:'mathilde',message:'',request_id:crypto.randomUUID(),revision:branch.revision,influence:5,...opts};}
const past = await call('/branches',{scene:7});
const autoInput=request(past,{message:'真相已经说开了。那些年无法取回，你可以不急着原谅，也不用向我再解释。今天就让自己安静地走一段路吧。'});
let automatic=await call('/branches/'+past.id+'/turn',autoInput);
if(!automatic.ending)automatic=await call('/branches/'+past.id+'/turn',request(automatic,{advance:true}));
assert.ok(automatic.ending, 'resolved final scene should naturally end');
assert.ok(automatic.ending.contributions.every(c=>automatic.turns.some(t=>t.id===c.turn_id&&t.message)));
console.log(JSON.stringify({case:'automatic',title:automatic.ending.title,resolution:automatic.ending.resolution,story:automatic.ending.story,contributions:automatic.ending.contributions,revision:automatic.revision}));
const early=await call('/branches',{scene:4});
const input=request(early,{finish:true,influence:10});
const finished=await call('/branches/'+early.id+'/turn',input);
assert.ok(finished.ending);assert.equal(finished.influence,10);assert.deepEqual(finished.ending.contributions,[],'no invented contribution before any visitor dialogue');
const retry=await call('/branches/'+early.id+'/turn',input);assert.equal(retry.revision,finished.revision);
await assert.rejects(()=>call('/branches/'+early.id+'/turn',request(finished,{advance:true})),/已经完结/);
console.log(JSON.stringify({case:'manual',title:finished.ending.title,resolution:finished.ending.resolution,story:finished.ending.story,new_timeline:finished.ending.new_timeline,revision:finished.revision}));
console.log('PASS: automatic ending, manual ending, traceable contribution, completed lock, idempotent retry.');
