import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir} from 'node:fs/promises';
const {chromium}=createRequire(import.meta.url)('playwright');
const base='http://127.0.0.1:3325'; // isolated, no model configuration
const browser=await chromium.launch({executablePath:process.env.READING_CHROME_EXECUTABLE,headless:true});
try {
 await mkdir('data/evals/reading-mobile',{recursive:true});
 for(const viewport of [{width:1440,height:1000},{width:390,height:844},{width:320,height:740}]){
  const context=await browser.newContext({viewport,isMobile:viewport.width<760,hasTouch:viewport.width<760});const page=await context.newPage();
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
  await move(4);
  const paragraph=page.locator('[data-paragraph="4"] p').first();
  if(viewport.width<760)await paragraph.tap({position:{x:24,y:24}});
  else await paragraph.click({position:{x:24,y:24}});
  await page.waitForFunction(()=>document.querySelector('[aria-label="定位原文段落"]').value==='4');
  const resumed=Number(await select.inputValue());
  const beforeY=await page.evaluate(()=>scrollY);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'horizontal overflow');
  if(viewport.width<760){
   await page.screenshot({path:`data/evals/reading-mobile/reading-${viewport.width}.png`});
   await page.getByRole('button',{name:'讨论这段',exact:true}).click();
   await page.getByRole('dialog',{name:'伴读面板'}).waitFor();
   assert.equal(await page.evaluate(()=>document.body.style.overflow),'hidden');
   const panelBox=await page.getByRole('dialog',{name:'伴读面板'}).boundingBox();
   assert.ok(panelBox&&Math.abs(panelBox.y)<2&&Math.abs(panelBox.height-viewport.height)<2,'mobile panel must cover the viewport');
  }
  await page.getByRole('textbox',{name:'伴读问题',exact:true}).fill('请解释当前这一段。');
  if(viewport.width<760){
   await page.screenshot({path:`data/evals/reading-mobile/panel-${viewport.width}.png`});
   const inputBox=await page.getByRole('textbox',{name:'伴读问题',exact:true}).boundingBox();
   assert.ok(inputBox&&inputBox.y>=0&&inputBox.y+inputBox.height<=viewport.height,'input is clipped');
   await page.setViewportSize({width:viewport.width,height:440});
   await page.waitForFunction(()=>Math.abs(document.querySelector('.cr-companion.mobile-open').getBoundingClientRect().height-440)<2);
   const shortInput=await page.getByRole('textbox',{name:'伴读问题',exact:true}).boundingBox();
   assert.ok(shortInput&&shortInput.y>=0&&shortInput.y+shortInput.height<=440,'input is clipped in reduced viewport');
   await page.screenshot({path:`data/evals/reading-mobile/reduced-height-${viewport.width}.png`});
   await page.setViewportSize(viewport);
   await page.waitForFunction(height=>Math.abs(document.querySelector('.cr-companion.mobile-open').getBoundingClientRect().height-height)<2,viewport.height);
   await page.getByRole('button',{name:'关闭伴读',exact:true}).click();
   assert.ok(Math.abs(await page.evaluate(()=>scrollY)-beforeY)<4,'closing panel moved original text');
   await page.getByRole('button',{name:'讨论这段',exact:true}).click();
   assert.equal(await page.getByRole('textbox',{name:'伴读问题',exact:true}).inputValue(),'请解释当前这一段。');
  }
  await move(1);await page.waitForTimeout(700);assert.equal(Number(await select.inputValue()),resumed);
  assert.equal(modelRequests,0);assert.deepEqual(errors,[]);
  assert.ok(positions.length>=3);
  // Persisted selection is restored on reload, not reset to paragraph 1.
  await page.reload();await select.waitFor();assert.equal(Number(await select.inputValue()),resumed);
  console.log(JSON.stringify({viewport,auto,resumed,text_click:true,manual_pin:true,draft_frozen:true,restored:true,no_horizontal_overflow:true,modelRequests,page_errors:errors}));
  await context.close();
 }
}finally{await browser.close();}
