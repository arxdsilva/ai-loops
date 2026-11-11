// Screenshots a local HTML page (or an SVG frame) with headless Chrome over CDP.
// usage: node shot.mjs <file.html|file.svg> <out.png> [width] [height] [scale] [colorScheme]
// Used for the readme's report screenshot and the social preview image.
import { spawn } from 'node:child_process';
import { writeFileSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';

const [src, out, w = '1440', h = '900', scale = '2', scheme = 'light'] = process.argv.slice(2);
const width = Number(w), height = Number(h), dpr = Number(scale);
const chrome = process.env.CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const port = 9334;
const profile = mkdtempSync(join(tmpdir(), 'loops-shot-'));
const proc = spawn(chrome, ['--headless=new', `--remote-debugging-port=${port}`, '--disable-gpu', '--hide-scrollbars',
  `--window-size=${width},${height}`, `--user-data-dir=${profile}`, 'about:blank'], { stdio: 'ignore' });
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
await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: dpr, mobile: false });
await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: scheme }, { name: 'prefers-reduced-motion', value: 'reduce' }] });
await send('Page.navigate', { url: `file://${resolve(src)}` });
await sleep(1500);
const shot = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
writeFileSync(out, Buffer.from(shot.result.data, 'base64'));
ws.close(); proc.kill();
console.log(out);
