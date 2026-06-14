// Minimal static dev server for the audit app (no dependencies). The app is pure static
// files; this just serves them with the right MIME types (notably application/wasm).
//   bash build.sh && node serve.mjs   ->  http://localhost:8799
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { extname, join, normalize } from 'node:path';

const ROOT = process.argv[2] || new URL('.', import.meta.url).pathname;
const PORT = Number(process.env.PORT) || 8799;
const MIME = {
  '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript',
  '.css': 'text/css', '.wasm': 'application/wasm', '.xml': 'application/xml',
  '.json': 'application/json', '.png': 'image/png',
};

createServer(async (req, res) => {
  try {
    let p = decodeURIComponent(req.url.split('?')[0]);
    if (p === '/') p = '/index.html';
    const safe = normalize(p).split(/[\\/]/).filter((s) => s && s !== '..').join('/');
    const data = await readFile(join(ROOT, safe));
    res.writeHead(200, { 'Content-Type': MIME[extname(safe)] || 'application/octet-stream' });
    res.end(data);
  } catch {
    res.writeHead(404);
    res.end('not found');
  }
}).listen(PORT, () => console.log('audit app -> http://localhost:' + PORT));
