const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');

const root = path.resolve('web');
const port = 4173;
const contentTypes = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.svg': 'image/svg+xml',
};

function fileFor(requestPath) {
  const decoded = decodeURIComponent(requestPath).replace(/^\/+/, '');
  let candidate = path.resolve(root, decoded || 'index.html');
  if (candidate !== root && !candidate.startsWith(`${root}${path.sep}`)) return null;
  try {
    if (fs.statSync(candidate).isDirectory()) candidate = path.join(candidate, 'index.html');
  } catch {
    return null;
  }
  return candidate;
}

const server = http.createServer((request, response) => {
  const target = fileFor(new URL(request.url, `http://${request.headers.host}`).pathname);
  if (!target) {
    response.writeHead(404, {'content-type': 'text/plain; charset=utf-8'});
    response.end('Not found');
    return;
  }
  response.writeHead(200, {
    'cache-control': 'no-store',
    'content-type': contentTypes[path.extname(target)] || 'application/octet-stream',
  });
  fs.createReadStream(target).pipe(response);
});

server.listen(port, '127.0.0.1', () => {
  console.log(`Serving embedded Web UI at http://127.0.0.1:${port}`);
});
