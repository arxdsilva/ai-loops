// Records the animated loops workflow SVG to PNG frames via headless Chrome (CDP).
// usage: node record.mjs <svg> <outdir> <seconds> <fps>
import { spawn } from 'node:child_process';
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs';
const [svgPath, outDir, secs = '15', fpsArg = '30'] = process.argv.slice(2);
const fps = Number(fpsArg), frames = Math.round(Number(secs) * fps);
mkdirSync(outDir, { recursive: true });
// scale the SVG to fit 1760x1023 inside the 1920x1080 frame, keeping its aspect ratio
const raw = readFileSync(svgPath, 'utf8');
const [, sw, sh] = raw.match(/width="(\d+)" height="(\d+)"/).map(Number);
const k = Math.min(1760 / sw, 1023 / sh);
const svg = raw.replace(/width="\d+" height="\d+"/, `width="${Math.round(sw * k)}" height="${Math.round(sh * k)}"`);
const html = `<!doctype html><html><head><meta charset="utf-8"><style>
html,body{margin:0;width:1920px;height:1080px;background:#fff;display:flex;align-items:center;justify-content:center;overflow:hidden}
</style></head><body>${svg}</body></html>`;
writeFileSync(`${outDir}/page.html`, html);
const chrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const port = 9333;
const proc = spawn(chrome, ['--headless=new', `--remote-debugging-port=${port}`, '--disable-gpu', '--hide-scrollbars',
  '--window-size=1920,1080', '--force-device-scale-factor=1', `--user-data-dir=${outDir}/profile`, 'about:blank'], { stdio: 'ignore' });
const sleep = ms => new Promise(r => setTimeout(r, ms));
let target;
for (let i = 0; i < 50 && !target; i++) {
  try { const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json(); target = list.find(t => t.type === 'page'); } catch {}
  if (!target) await sleep(200);
}
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise(r => ws.addEventListener('open', r, { once: true }));
let id = 0; const pending = new Map();
ws.addEventListener('message', e => { const m = JSON.parse(e.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); } });
const send = (method, params = {}) => new Promise(r => { const i = ++id; pending.set(i, r); ws.send(JSON.stringify({ id: i, method, params })); });
await send('Page.enable');
await send('Emulation.setDeviceMetricsOverride', { width: 1920, height: 1080, deviceScaleFactor: 1, mobile: false });
await send('Page.navigate', { url: `file://${outDir}/page.html` });
await sleep(800);
for (let f = 0; f < frames; f++) {
  const t = (f / fps) * 1000;
  await send('Runtime.evaluate', { expression: `document.getAnimations().forEach(a=>{a.pause();a.currentTime=${t}})` });
  const shot = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
  writeFileSync(`${outDir}/f${String(f).padStart(4, '0')}.png`, Buffer.from(shot.result.data, 'base64'));
}
ws.close(); proc.kill();
console.log(`frames: ${frames}`);
