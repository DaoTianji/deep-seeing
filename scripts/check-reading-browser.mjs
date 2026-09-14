// Opt-in real-browser / real-model acceptance, with a fresh browser context.
// Never connects to the user's Chrome profile or modifies an existing story.
// NODE_PATH may point to a supplied runtime's Playwright installation.
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir, writeFile, readFile} from 'node:fs/promises';
import {resolve, relative, isAbsolute, join} from 'node:path';
const {chromium} = createRequire(import.meta.url)('playwright');

const base = process.env.READING_BROWSER_BASE || 'http://127.0.0.1:3321';
const target = new URL(base);
assert.ok(['http://127.0.0.1:3321', 'http://127.0.0.1:3322'].includes(target.origin), 'use an isolated acceptance service, never the reader preview');
const preflight = process.argv.includes('--preflight');
assert.ok(preflight || process.env.READING_BROWSER_LIVE === '1', 'explicit real-model opt-in required');
assert.ok(process.env.READING_CHROME_EXECUTABLE, 'provide a Chrome executable, never a user profile');
const root = resolve('data/evals');
const out = resolve(process.env.READING_BROWSER_REPORT || 'data/evals/browser-ending-v7');
const rel = relative(root, out);
assert.ok(rel && !rel.startsWith('..') && !isAbsolute(rel), 'report must be a child of ignored data/evals');
await mkdir(out, {recursive:true, mode:0o700});

const report = {started_at:new Date().toISOString(), preflight, status:'running', steps:[], page_errors:[], turn_requests:0};
const save = () => writeFile(join(out, 'report.json'), JSON.stringify(report, null, 2), {mode:0o600});
let phase = 'launch';
const heartbeat = setInterval(() => console.log(JSON.stringify({phase, status:'waiting', time:new Date().toISOString()})), 15000);
const browser = await chromium.launch({executablePath:process.env.READING_CHROME_EXECUTABLE, headless:true});
const context = await browser.newContext({viewport:{width:1440,height:1000}, acceptDownloads:true});
const page = await context.newPage();
page.setDefaultTimeout(15000);
page.on('pageerror', e => report.page_errors.push(e.message));
async function step(name, fn) {
  phase = name;
  const started = Date.now();
  console.log(JSON.stringify({phase, status:'started'}));
  try {
    await fn();
    report.steps.push({name, passed:true, elapsed_ms:Date.now()-started});
    console.log(JSON.stringify({phase, status:'passed'}));
  } catch (e) {
    report.steps.push({name, passed:false, elapsed_ms:Date.now()-started, error:e.message});
    throw e;
  } finally { await save(); }
}
async function clickResponse(button, endpoint, method='POST', timeout=15000) {
  const waiting = page.waitForResponse(r => new URL(r.url()).pathname === endpoint && r.request().method() === method, {timeout});
  await button.click();
  const response = await waiting;
  const body = await response.json();
  assert.ok(response.ok(), `HTTP ${response.status()}: ${body.error || 'request rejected'}`);
  return body;
}
let branch;
let original;
const advice = '不要为了体面立刻举债买替代品。请先和妻子商量，明早一同去向朋友坦白遗失，问清她的项链是什么，再讨论实际应该怎样弥补。这是我的建议，不代表你们已经这样做了。';
try {
  await step('read_original', async () => {
    await page.goto(base+'/reading?book=necklace');
    await page.getByRole('textbox',{name:'名字或独特的笔名'}).fill('浏览器验收-'+crypto.randomUUID().slice(0,12));
    await page.getByRole('button',{name:'走进我的书房',exact:true}).click();
    const paper = page.locator('[aria-label="作品正文"]');
    await paper.waitFor();
    original = await paper.innerText();
    assert.ok(original.includes('She was one of those pretty and charming girls'));
    await page.screenshot({path:join(out,'reading.png'), fullPage:false});
  });
  if (!preflight) {
    await step('choose_new_branch', async () => {
      await page.getByRole('button', {name:'读完之后',exact:true}).click();
      await page.getByRole('dialog',{name:'读完之后'}).getByRole('button',{name:'探索另一种可能',exact:true}).click();
      await page.getByRole('button', {name:/^L 路瓦栽先生/}).click();
      branch = await clickResponse(page.getByRole('button',{name:'走进这个场景',exact:true}), '/api/story/branches');
      assert.equal(branch.revision,1);
      assert.equal(branch.anchor,4);
      assert.equal(branch.turns.length,0);
      report.branch_id=branch.id;
      report.resume_url=base+'/reading?book=necklace&mode=story&branch='+branch.id;
      const slider = page.getByRole('slider',{name:/来访者影响力/});
      await slider.focus();
      await slider.press('End');
      assert.equal(await slider.inputValue(),'10');
    });
    await step('give_advice', async () => {
      await page.getByRole('textbox',{name:'对人物说的话',exact:true}).fill(advice);
      report.turn_requests++;
      branch = await clickResponse(page.getByRole('button',{name:'发送',exact:true}), '/api/story/branches/'+branch.id+'/turn', 'POST', 900000);
      report.advice_result=branch;
      assert.equal(branch.influence,10);
      assert.equal(branch.turns[0].message,advice);
      assert.equal(branch.turns[0].character_id,'loisel');
      await page.screenshot({path:join(out,'advice.png'),fullPage:false});
    });
    if (!branch.ending) {
      await step('manual_ending', async () => {
        report.turn_requests++;
        branch = await clickResponse(page.getByRole('button',{name:'让故事走向结局',exact:true}), '/api/story/branches/'+branch.id+'/turn', 'POST', 900000);
        report.ending_result=branch;
        assert.ok(branch.ending, 'manual finish did not produce an ending');
      });
    } else {
      report.ending_result=branch;
      report.ended_automatically=true;
    }
    await step('inspect_and_sign_card', async () => {
      const card = page.getByRole('article',{name:'故事完结纪念卡'});
      await card.waitFor();
      for (const title of ['原著的故事线','你走过的故事线','你留下的回声']) assert.ok(await card.getByRole('heading',{name:title,exact:true}).isVisible());
      assert.ok(branch.ending.new_timeline.length > 0);
      assert.ok(branch.ending.contributions.length > 0, 'advice has no attributed contribution');
      for (const c of branch.ending.contributions) assert.ok(branch.turns.some(t=>t.id===c.turn_id && t.message===advice));
      await card.getByRole('textbox',{name:'留下你的署名'}).fill('独立浏览器验收访客');
      branch = await clickResponse(card.getByRole('button',{name:'签下这一页',exact:true}), '/api/story/branches/'+branch.id+'/signature');
      assert.equal(branch.signature,'独立浏览器验收访客');
      assert.equal(await page.getByRole('textbox',{name:'对人物说的话',exact:true}).count(),0);
      report.signed_result=branch;
      await card.screenshot({path:join(out,'signed-card.png')});
    });
    await step('download_and_reload', async () => {
      const downloadEvent = page.waitForEvent('download');
      await page.getByRole('button',{name:'保存纪念卡',exact:true}).click();
      const download=await downloadEvent;
      const path=join(out,'ending-card.md');
      await download.saveAs(path);
      const text=await readFile(path,'utf8');
      assert.ok(text.includes(branch.signature));
      assert.ok(text.includes('\n## 原著故事线（含结局）\n'));
      assert.ok(text.includes('\n## 这一次的故事\n'));
      assert.ok(!text.includes('\\n'), 'literal backslash-n in Markdown export');
      await page.goto(report.resume_url);
      await page.locator('.rw-signature-name').waitFor();
      assert.equal(await page.locator('.rw-signature-name').innerText(),branch.signature);
    });
    await step('return_to_original', async () => {
      await page.getByRole('link',{name:'回到原著伴读',exact:true}).click();
      const paper=page.locator('[aria-label="作品正文"]');
      await paper.waitFor();
      assert.equal(await paper.innerText(),original);
      assert.equal(await page.getByRole('article',{name:'故事完结纪念卡'}).count(),0);
      assert.equal(await page.getByText(branch.ending.title,{exact:true}).count(),0);
      await page.screenshot({path:join(out,'returned-original.png'),fullPage:false});
    });
  }
  assert.deepEqual(report.page_errors,[],'browser runtime errors occurred');
  report.status='passed';
  report.semantic_review=preflight?'not_applicable':'pending_review_of_events_and_ending';
} catch (e) {
  report.status='failed';
  report.error=e.message;
  await page.screenshot({path:join(out,'failure.png'),fullPage:false}).catch(()=>{});
  process.exitCode=1;
} finally {
  report.finished_at=new Date().toISOString();
  // This fresh context contains only the fictional localhost test visitor.
  // Preserve it privately so a UI-only failure need not repeat model calls.
  await writeFile(join(out,'browser-state.json'), JSON.stringify(await context.storageState()), {mode:0o600});
  await save();
  clearInterval(heartbeat);
  await browser.close();
  console.log(JSON.stringify({status:report.status,phase,report:join(out,'report.json'),error:report.error}));
}
