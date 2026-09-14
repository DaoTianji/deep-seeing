import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
const {chromium}=createRequire(import.meta.url)('playwright');
const base='http://127.0.0.1:3325'; // isolated, no model configuration
const browser=await chromium.launch({executablePath:process.env.READING_CHROME_EXECUTABLE,headless:true});
try {
 for(const viewport of [{width:1440,height:1000},{width:390,height:844}]){
  const context=await browser.newContext({viewport});const page=await context.newPage();
  const errors=[],positions=[];let modelRequests=0;
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{
   if(r.method()==='POST'&&r.url().includes('/companion/position'))positions.push(r.postDataJSON());
   if(r.method()==='POST'&&/\/chat|\/translation|\/turn/.test(r.url()))modelRequests++;
  });
  await page.goto(base+'/reading?book=kong');
  await page.getByLabel('名字或独特的笔名').fill('定位验收-'+crypto.randomUUID().slice(0,8));
  await page.getByRole('button',{name:'开始阅读',exact:true}).click();
  const select=page.getByLabel('定位原文段落');await select.waitFor();
  const move=async id=>page.locator(`[data-paragraph="${id}"]`).evaluate(el=>el.scrollIntoView({block:'start',behavior:'instant'}));
  await move(3);
  await page.waitForFunction(()=>Number(document.querySelector('[aria-label="定位原文段落"]').value)>=3);
  const auto=Number(await select.inputValue());assert.ok(auto>=3);
  // Offscreen controls are operated without scrolling the paper underneath them.
  await select.evaluate(el=>{el.value='2';el.dispatchEvent(new Event('change',{bubbles:true}));});
  await page.waitForFunction(()=>document.querySelector('[aria-label="定位原文段落"]').value==='2');
  await move(6);await page.waitForTimeout(700);assert.equal(await select.inputValue(),'2');
  await page.getByRole('button',{name:'恢复自动跟随'}).evaluate(el=>el.click());
  await page.waitForFunction(()=>Number(document.querySelector('[aria-label="定位原文段落"]').value)>=6);
  const resumed=Number(await select.inputValue());
  if(viewport.width<760)await page.getByRole('button',{name:'打开伴读',exact:true}).evaluate(el=>el.click());
  await page.getByRole('textbox',{name:'伴读问题',exact:true}).fill('请解释当前这一段。');
  await move(1);await page.waitForTimeout(700);assert.equal(Number(await select.inputValue()),resumed);
  assert.equal(modelRequests,0);assert.deepEqual(errors,[]);
  assert.ok(positions.length>=3);
  // Persisted selection is restored on reload, not reset to paragraph 1.
  await page.reload();await select.waitFor();assert.equal(Number(await select.inputValue()),resumed);
  console.log(JSON.stringify({viewport,auto,resumed,manual_pin:true,draft_frozen:true,restored:true,modelRequests,page_errors:errors}));
  await context.close();
 }
}finally{await browser.close();}
