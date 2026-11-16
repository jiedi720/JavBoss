// avdanyu fetch helper — reads public search results from avdanyuwiki.com via a
// locally running Chromium/Brave debugging endpoint. The site is behind
// Cloudflare; a real browser clears the challenge, plain HTTP clients do not.
//
// Usage: node fetch.mjs "<keyword>" [cdpEndpoint]
// Output: JSON on stdout, diagnostics on stderr.
const keyword = process.argv[2] || '';
const cdpBase = (process.argv[3] || process.env.AVDANYU_CDP || 'http://127.0.0.1:9222').replace(/\/+$/, '');
if (!keyword) {
  console.error('avdanyu: missing keyword');
  process.exit(2);
}

const timeoutMs = 45000;
const started = Date.now();

const ver = await (await fetch(`${cdpBase}/json/version`)).json();
const bws = new WebSocket(ver.webSocketDebuggerUrl);
await new Promise((r, j) => { bws.onopen = r; bws.onerror = j; });
let id = 0;
let sessionId = null;
const pending = new Map();
bws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); }
};
const raw = (method, params = {}, sid) => new Promise((r) => {
  const i = ++id;
  pending.set(i, r);
  bws.send(JSON.stringify({ id: i, method, params, ...(sid ? { sessionId: sid } : {}) }));
});
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const ev = async (x) => (await raw('Runtime.evaluate', { expression: x, returnByValue: true, awaitPromise: true }, sessionId)).result?.result?.value;

const created = await raw('Target.createTarget', { url: 'about:blank', background: true, newWindow: false });
const targetId = created.result?.targetId;
const attached = await raw('Target.attachToTarget', { targetId, flatten: true });
sessionId = attached.result?.sessionId;
const cmd = (m, p = {}) => raw(m, p, sessionId);

let out = { keyword, works: [] };
try {
  await cmd('Runtime.enable');
  await cmd('Page.enable');
  await cmd('Page.setWebLifecycleState', { state: 'active' });
  await cmd('Emulation.setFocusEmulationEnabled', { enabled: true });

  const url = `https://avdanyuwiki.com/?s=${encodeURIComponent(keyword)}`;
  await cmd('Page.navigate', { url });

  // Wait for Cloudflare challenge to clear and articles to show up.
  let articleCount = 0;
  for (let i = 0; i < 45; i += 1) {
    await wait(1000);
    articleCount = await ev(`document.querySelectorAll('article').length`) || 0;
    if (articleCount > 0 && Date.now() - started > 3000) break;
    if (Date.now() - started > timeoutMs) break;
  }

  const pageTitle = await ev(`document.title||''`) || '';
  if (/just a moment|请稍候|attention required|checking your browser/i.test(pageTitle) && articleCount === 0) {
    out.error = 'cloudflare_blocked';
    out.title = pageTitle;
  } else {
    out.title = pageTitle;
    out.works = await ev(`(() => {
      const norm = (s) => (s || '').replace(/\\u00a0/g, ' ').replace(/[ \\t]+/g, ' ').trim();
      return [...document.querySelectorAll('article')].map((a) => {
        const text = (a.innerText || '').replace(/\\u00a0/g, ' ');
        const field = (label, next) => {
          const re = new RegExp(label + '\\\\s*[:：]\\\\s*([\\\\s\\\\S]*?)(?=' + next + '|$)');
          const m = text.match(re);
          return m ? norm(m[1]) : '';
        };
        const boundary = '出演[:：]|出演AV男優|出演男優|メーカー|収録時間|品番|配信開始日|商品発売日|シリーズ|レーベル|ジャンル|リンク先削除報告';
        const maleBlock = field('出演AV男優', boundary);
        const males = maleBlock.split(/[\\n\\r]+/).map(norm).filter((x) => x && !/^出演/.test(x));
        const maleAlt = field('出演男優', boundary);
        if (maleAlt && !males.some((m) => maleAlt.includes(m) || m.includes(maleAlt))) {
          maleAlt.split(/[\\n\\r,\\s]+/).map(norm).filter(Boolean).forEach((x) => { if (!males.includes(x)) males.push(x); });
        }
        const rawCode = field('品番', '配信開始日|商品発売日|シリーズ|ジャンル|メーカー|収録時間|$');
        const dur = field('収録時間', '品番|配信開始日|$').match(/(\\d+)/);
        const date = field('配信開始日', '商品発売日|シリーズ|$').match(/(\\d{4})\\/(\\d{1,2})\\/(\\d{1,2})/);
        const titleEl = a.querySelector('h1,h2,h3');
        return {
          title: norm(titleEl ? titleEl.innerText : ''),
          code: norm(rawCode),
          studio: field('メーカー', '収録時間|品番|$'),
          series: field('シリーズ', 'レーベル|ジャンル|$'),
          duration_min: dur ? Number(dur[1]) : 0,
          release_date: date ? (date[1] + '-' + String(date[2]).padStart(2, '0') + '-' + String(date[3]).padStart(2, '0')) : '',
          actors: males,
        };
      }).filter((w) => w.actors.length > 0 || w.code);
    })()`) || [];
  }
} catch (e) {
  out.error = 'fetch_failed';
  out.message = String(e);
} finally {
  try { await raw('Target.closeTarget', { targetId }); } catch (_) {}
  bws.close();
}
console.log(JSON.stringify(out));
