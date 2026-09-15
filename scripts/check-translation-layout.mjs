import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir} from 'node:fs/promises';
const {chromium}=createRequire(import.meta.url)('playwright');
const browser=await chromium.launch({executablePath:process.env.READING_CHROME_EXECUTABLE,headless:true});
try{
 await mkdir('data/evals/translation-layout',{recursive:true});
 for(const width of [1440,390,320]){
  const context=await browser.newContext({viewport:{width,height:900},isMobile:width<760,hasTouch:width<760});
  const page=await context.newPage();let modelRequests=0;const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{if(r.method()==='POST'&&/\/chat|\/translation|\/turn/.test(r.url()))modelRequests++;});
  await page.goto('http://127.0.0.1:3325/reading?book=magi');
  await page.getByLabel('名字或独特的笔名').fill('译文布局-'+crypto.randomUUID().slice(0,8));
  await page.getByRole('button',{name:'开始阅读',exact:true}).click();
  await page.getByLabel('阅读方式',{exact:true}).selectOption('parallel');
  const first=page.locator('.cr-sentence-pair').first();await first.waitFor();
  await first.scrollIntoViewIfNeeded();
  const style=await first.evaluate(el=>{
   const button=el.querySelector('button'),p=el.querySelector('.cr-sentence-translation');
   const b=button.getBoundingClientRect(),t=p.getBoundingClientRect(),css=getComputedStyle(button);
   return {font:parseFloat(css.fontSize),label:button.textContent,buttonX:b.x,textRight:t.right,buttonTop:b.y,textBottom:t.bottom,width:b.width,height:b.height};
  });
  assert.equal(style.font,12);assert.equal(style.label,'');
  assert.ok(style.buttonX>=style.textRight&&style.buttonTop<style.textBottom,'button must be beside the translation, not a separate row');
  if(width<760)assert.ok(style.width>=44&&style.height>=44,'small icon must retain a usable touch target');
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  assert.equal(await page.getByText('重译此句 · 仅自己可见',{exact:true}).count(),0);
  await page.screenshot({path:`data/evals/translation-layout/magi-${width}.png`});
  assert.equal(modelRequests,0);assert.deepEqual(errors,[]);
  console.log(JSON.stringify({width,style,modelRequests,errors}));await context.close();
 }
}finally{await browser.close();}
