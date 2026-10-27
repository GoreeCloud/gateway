import http from 'node:http';
import { readFile, mkdir, writeFile, chmod } from 'node:fs/promises';
import { randomBytes, createHash, timingSafeEqual } from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { demoConfig, demoUpstreams, demoMetrics } from './demo.js';
import { fingerprint, summarize, validateDraft, validateAdminUrl } from './caddy.js';

const projectDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const distDir = path.join(projectDir, 'dist');
const sourceDir = path.join(projectDir, 'src');
const MAX_BODY = 1024 * 1024 + 4096;
const TTL = 8 * 60 * 60 * 1000;
const MIME = { '.html': 'text/html; charset=utf-8', '.css': 'text/css; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.svg': 'image/svg+xml', '.json': 'application/json' };

function json(res, status, data, extra = {}) {
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8', 'Cache-Control': 'no-store', ...extra });
  res.end(JSON.stringify(data));
}
function error(res, status, message) { json(res, status, { error: message }); }
function digest(value) { return createHash('sha256').update(value).digest(); }
function safeEqual(a, b) { return timingSafeEqual(digest(a), digest(b)); }
function cookies(req) {
  return Object.fromEntries((req.headers.cookie ?? '').split(';').map(entry => {
    const i = entry.indexOf('=');
    return i < 0 ? ['', ''] : [entry.slice(0, i).trim(), entry.slice(i + 1).trim()];
  }));
}
async function body(req) {
  const chunks = [];
  let size = 0;
  for await (const chunk of req) {
    size += chunk.length;
    if (size > MAX_BODY) throw Object.assign(new Error('Request exceeds the 1 MiB configuration limit'), { status: 413 });
    chunks.push(chunk);
  }
  const content = Buffer.concat(chunks).toString('utf8');
  if (!req.headers['content-type']?.startsWith('application/json')) throw Object.assign(new Error('Expected application/json'), { status: 415 });
  try { return JSON.parse(content); } catch { throw Object.assign(new Error('Invalid request JSON'), { status: 400 }); }
}

export function createGatewayServer(env = process.env) {
  const demo = env.GATEWAY_DEMO === '1';
  const allowApply = !demo && env.GATEWAY_ALLOW_APPLY === '1';
  const token = env.GATEWAY_ACCESS_TOKEN ?? '';
  if (!demo && token.length < 32) throw new Error('Set GATEWAY_ACCESS_TOKEN to a random secret of at least 32 characters');
  const publicOrigin = env.GATEWAY_PUBLIC_ORIGIN ? new URL(env.GATEWAY_PUBLIC_ORIGIN) : null;
  if (publicOrigin && (publicOrigin.protocol !== 'https:' || publicOrigin.username || publicOrigin.password || publicOrigin.pathname !== '/' || publicOrigin.search || publicOrigin.hash)) throw new Error('GATEWAY_PUBLIC_ORIGIN must be a bare HTTPS origin');
  const adminOrigin = demo ? null : validateAdminUrl(env.CADDY_ADMIN_URL ?? 'http://127.0.0.1:2019');
  const secure = Boolean(publicOrigin);
  const sessions = new Map();
  const attempts = new Map();
  const events = [];
  let applying = false;
  const audit = (type, detail) => {
    events.unshift({ id: randomBytes(8).toString('hex'), type, detail, time: new Date().toISOString() });
    if (events.length > 60) events.length = 60;
  };
  if (demo) audit('demo', 'Synthetic preview enabled; Caddy is not connected');

  const sessionFor = req => {
    if (demo) return { csrf: 'demo', expires: Date.now() + TTL };
    const sid = cookies(req).gw_session;
    const session = sid && sessions.get(sid);
    if (!session) return null;
    if (session.expires <= Date.now()) { sessions.delete(sid); return null; }
    return session;
  };
  const originAllowed = req => {
    const host = req.headers.host ?? '';
    const allowedHost = publicOrigin ? publicOrigin.host : null;
    const local = /^(localhost|127\.0\.0\.1|\[::1\])(?::\d{1,5})?$/.test(host);
    if (allowedHost ? host !== allowedHost : !local) return false;
    if (!['POST', 'PUT', 'PATCH', 'DELETE'].includes(req.method)) return true;
    const origin = req.headers.origin;
    if (!origin) return false;
    if (publicOrigin) return origin === publicOrigin.origin;
    try { return new URL(origin).host === host && ['http:', 'https:'].includes(new URL(origin).protocol); }
    catch { return false; }
  };
  const caddy = async (pathname, options = {}) => {
    const response = await fetch(`${adminOrigin}${pathname}`, { ...options, signal: AbortSignal.timeout(6000), redirect: 'error' });
    const text = await response.text();
    if (!response.ok) throw Object.assign(new Error(`Caddy rejected request (HTTP ${response.status}); inspect the private Caddy logs`), { status: 502 });
    return text;
  };
  const activeConfig = async () => demo ? structuredClone(demoConfig) : JSON.parse(await caddy('/config/'));
  const activity = async () => {
    const config = await activeConfig();
    const [up, mt] = demo ? [JSON.stringify(demoUpstreams), demoMetrics] : await Promise.all([
      caddy('/reverse_proxy/upstreams').catch(() => '{}'),
      caddy('/metrics').catch(() => ''),
    ]);
    const summary = summarize(config, JSON.parse(up), mt);
    return { ...summary, demo, readOnly: !allowApply, connected: !demo };
  };
  const handler = async (req, res) => {
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer');
    res.setHeader('X-Frame-Options', 'DENY');
    res.setHeader('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');
    res.setHeader('Content-Security-Policy', "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'; object-src 'none'");
    if (secure) res.setHeader('Strict-Transport-Security', 'max-age=31536000');
    try {
      if (!originAllowed(req)) return error(res, 403, 'Request origin is not permitted');
      const pathname = new URL(req.url, 'http://localhost').pathname;
      if (pathname.startsWith('/api/')) {
        const session = sessionFor(req);
        if (pathname === '/api/session' && req.method === 'GET') {
          return json(res, 200, { authenticated: Boolean(session), demo, readOnly: !allowApply, csrf: session?.csrf ?? null });
        }
        if (pathname === '/api/login' && req.method === 'POST') {
          if (demo) return json(res, 200, { authenticated: true, demo, readOnly: true, csrf: 'demo' });
          const ip = req.socket.remoteAddress || 'unknown';
          const now = Date.now();
          const window = (attempts.get(ip) ?? []).filter(t => now - t < 15 * 60 * 1000);
          if (window.length >= 5) return error(res, 429, 'Too many attempts; retry in 15 minutes');
          const payload = await body(req);
          if (typeof payload?.token !== 'string' || !safeEqual(payload.token, token)) {
            window.push(now); attempts.set(ip, window); return error(res, 401, 'Invalid access token');
          }
          attempts.delete(ip);
          const id = randomBytes(32).toString('base64url');
          const value = { csrf: randomBytes(32).toString('base64url'), expires: now + TTL };
          sessions.set(id, value);
          audit('login', 'Operator authenticated');
          return json(res, 200, { authenticated: true, demo, readOnly: !allowApply, csrf: value.csrf }, {
            'Set-Cookie': `gw_session=${id}; HttpOnly; SameSite=Strict; Path=/; Max-Age=${TTL / 1000}${secure ? '; Secure' : ''}`,
          });
        }
        if (!session) return error(res, 401, 'Authentication required');
        if (req.method === 'POST' && req.headers['x-csrf-token'] !== session.csrf) return error(res, 403, 'Invalid CSRF token');
        if (pathname === '/api/logout' && req.method === 'POST') {
          sessions.delete(cookies(req).gw_session);
          audit('logout', 'Operator signed out');
          return json(res, 200, { ok: true }, { 'Set-Cookie': `gw_session=; HttpOnly; SameSite=Strict; Path=/; Max-Age=0${secure ? '; Secure' : ''}` });
        }
        if (pathname === '/api/overview' && req.method === 'GET') return json(res, 200, await activity());
        if (pathname === '/api/config' && req.method === 'GET') {
          const config = await activeConfig();
          return json(res, 200, { content: JSON.stringify(config, null, 2), fingerprint: fingerprint(config) });
        }
        if (pathname === '/api/events' && req.method === 'GET') return json(res, 200, { events });
        if (pathname === '/api/validate' && req.method === 'POST') {
          const payload = await body(req);
          const config = validateDraft(payload?.content);
          return json(res, 200, { valid: true, scope: 'JSON syntax and basic structure only (not Caddy runtime validation)', fingerprint: fingerprint(config), summary: summarize(config, {}, '').routeCount });
        }
        if (pathname === '/api/apply' && req.method === 'POST') {
          if (!allowApply) return error(res, 403, 'Applying changes is disabled by the server operator');
          if (applying) return error(res, 409, 'Another configuration change is in progress');
          const payload = await body(req);
          if (payload?.confirm !== 'APPLY' || typeof payload?.baseFingerprint !== 'string') return error(res, 400, 'Explicit APPLY confirmation and base revision are required');
          const newConfig = validateDraft(payload.content);
          applying = true;
          try {
            const original = await activeConfig();
            if (fingerprint(original) !== payload.baseFingerprint) return error(res, 409, 'The live configuration has changed. Reload the editor before applying.');
            if (fingerprint(original) === fingerprint(newConfig)) return json(res, 200, { changed: false, fingerprint: fingerprint(original) });
            const backupDir = path.resolve(env.GATEWAY_BACKUP_DIR ?? path.join(projectDir, 'data', 'backups'));
            await mkdir(backupDir, { recursive: true, mode: 0o700 });
            await chmod(backupDir, 0o700);
            const backupName = `${new Date().toISOString().replaceAll(':', '-')}-${randomBytes(5).toString('hex')}.json`;
            await writeFile(path.join(backupDir, backupName), JSON.stringify(original, null, 2), { flag: 'wx', mode: 0o600 });
            await caddy('/load', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(newConfig) });
            const after = await activeConfig();
            audit('apply', `Configuration reloaded; prior state snapshotted as ${backupName}`);
            return json(res, 200, { changed: true, verified: true, fingerprint: fingerprint(after), backup: backupName });
          } finally { applying = false; }
        }
        return error(res, 404, 'API endpoint not found');
      }
      if (req.method !== 'GET' && req.method !== 'HEAD') return error(res, 405, 'Method not allowed');
      const file = pathname === '/' ? 'index.html' : pathname.slice(1);
      if (!['index.html', 'styles.css', 'app.js', 'model.js', 'mark.svg'].includes(file)) return error(res, 404, 'File not found');
      let contents;
      try { contents = await readFile(path.join(distDir, file)); }
      catch { contents = await readFile(path.join(sourceDir, file)); }
      res.writeHead(200, { 'Content-Type': MIME[path.extname(file)] ?? 'application/octet-stream', 'Cache-Control': 'no-store' });
      res.end(req.method === 'HEAD' ? undefined : contents);
    } catch (err) {
      // Avoid exposing raw config or secret values via error responses.
      const safeMessage = err.status ? err.message : (err instanceof SyntaxError ? 'Caddy returned invalid JSON' : 'Caddy is unreachable or the request failed');
      error(res, err.status ?? 503, safeMessage);
    }
  };
  return http.createServer((req, res) => { void handler(req, res); });
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const host = process.env.GATEWAY_HOST ?? '127.0.0.1';
  if (!['127.0.0.1', 'localhost', '::1'].includes(host)) throw new Error('GATEWAY_HOST must be loopback; expose through a trusted HTTPS reverse proxy instead');
  const port = Number(process.env.GATEWAY_PORT ?? 3088);
  const server = createGatewayServer();
  server.listen(port, host, () => console.info(`GoreeCloud Gateway listening on http://${host}:${port}`));
}
