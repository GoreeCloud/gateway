import test from 'node:test';
import assert from 'node:assert/strict';
import { createGatewayServer } from '../server/index.js';

async function withServer(env, callback) {
  const server = createGatewayServer(env);
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  try { await callback(origin); } finally { await new Promise(resolve => server.close(resolve)); }
}

test('fails closed without operator secret', () => assert.throws(() => createGatewayServer({}), /GATEWAY_ACCESS_TOKEN/));
test('fails closed for public origins without HTTPS', () => assert.throws(() => createGatewayServer({ GATEWAY_ACCESS_TOKEN: 'a'.repeat(32), GATEWAY_PUBLIC_ORIGIN: 'http://example.com' }), /HTTPS/));
test('demo is explicitly identified and cannot apply changes', async () => {
  await withServer({ GATEWAY_DEMO: '1' }, async origin => {
    const session = await (await fetch(`${origin}/api/session`)).json();
    assert.equal(session.authenticated, true); assert.equal(session.readOnly, true); assert.equal(session.demo, true);
    const o = await (await fetch(`${origin}/api/overview`)).json();
    assert.equal(o.sites, 6); assert.equal(o.demo, true); assert.equal(o.connected, false);
    const apply = await fetch(`${origin}/api/apply`, { method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json', 'X-CSRF-Token': 'demo' }, body: JSON.stringify({ content: '{}', confirm: 'APPLY', baseFingerprint: o.fingerprint }) });
    assert.equal(apply.status, 403);
  });
});
test('security origin rejects cross-origin mutations', async () => {
  await withServer({ GATEWAY_DEMO: '1' }, async origin => {
    const response = await fetch(`${origin}/api/validate`, { method: 'POST', headers: { Origin: 'https://evil.invalid', 'Content-Type': 'application/json', 'X-CSRF-Token': 'demo' }, body: JSON.stringify({ content: '{}' }) });
    assert.equal(response.status, 403);
  });
});
test('demo JSON syntax endpoint clarifies limited verification', async () => {
  await withServer({ GATEWAY_DEMO: '1' }, async origin => {
    const response = await fetch(`${origin}/api/validate`, { method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json', 'X-CSRF-Token': 'demo' }, body: JSON.stringify({ content: '{"apps":{}}' }) });
    const result = await response.json(); assert.equal(result.valid, true); assert.match(result.scope, /not Caddy runtime/);
  });
});
test('live mode requires session for API data', async () => {
  await withServer({ GATEWAY_ACCESS_TOKEN: 'a'.repeat(36) }, async origin => {
    const response = await fetch(`${origin}/api/overview`);
    assert.equal(response.status, 401);
  });
});
test('secure cookie login session and logout invalidation', async () => {
  await withServer({ GATEWAY_ACCESS_TOKEN: 'a'.repeat(36) }, async origin => {
    const login = await fetch(`${origin}/api/login`, { method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json' }, body: JSON.stringify({ token: 'a'.repeat(36) }) });
    const session = await login.json();
    assert.equal(session.authenticated, true);
    const cookie = login.headers.get('set-cookie').split(';')[0];
    assert.match(login.headers.get('set-cookie'), /HttpOnly; SameSite=Strict/);
    const active = await (await fetch(`${origin}/api/session`, { headers: { Cookie: cookie } })).json();
    assert.equal(active.authenticated, true);
    const logout = await fetch(`${origin}/api/logout`, { method: 'POST', headers: { Origin: origin, Cookie: cookie, 'Content-Type': 'application/json', 'X-CSRF-Token': active.csrf }, body: '{}' });
    assert.equal(logout.status, 200);
    const inactive = await (await fetch(`${origin}/api/session`, { headers: { Cookie: cookie } })).json();
    assert.equal(inactive.authenticated, false);
  });
});

test('live read and guarded reload work against an isolated Caddy API simulator', async () => {
  const http = await import('node:http');
  const { mkdtemp, readdir, stat, rm } = await import('node:fs/promises');
  const { tmpdir } = await import('node:os');
  const { join } = await import('node:path');
  let active = { apps: { http: { servers: { demo: { listen: [':443'], routes: [] } } } } };
  let loadCount = 0;
  const stub = http.createServer(async (req, res) => {
    if (req.url === '/config/') { res.setHeader('content-type', 'application/json'); return res.end(JSON.stringify(active)); }
    if (req.url === '/reverse_proxy/upstreams') return res.end(JSON.stringify({ upstreams: [] }));
    if (req.url === '/metrics') return res.end('caddy_http_requests_total 2\n');
    if (req.url === '/load' && req.method === 'POST') {
      let contents = ''; for await (const chunk of req) contents += chunk;
      active = JSON.parse(contents); loadCount++; return res.end();
    }
    res.statusCode = 404; res.end();
  });
  await new Promise(resolve => stub.listen(0, '127.0.0.1', resolve));
  const backup = await mkdtemp(join(tmpdir(), 'gateway-test-'));
  try {
    await withServer({ GATEWAY_ACCESS_TOKEN: 'x'.repeat(40), GATEWAY_ALLOW_APPLY: '1', CADDY_ADMIN_URL: `http://127.0.0.1:${stub.address().port}`, GATEWAY_BACKUP_DIR: backup }, async origin => {
      const login = await fetch(`${origin}/api/login`, { method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json' }, body: JSON.stringify({ token: 'x'.repeat(40) }) });
      const session = await login.json(), cookie = login.headers.get('set-cookie').split(';')[0];
      const headers = { Origin: origin, Cookie: cookie, 'Content-Type': 'application/json', 'X-CSRF-Token': session.csrf };
      const config = await (await fetch(`${origin}/api/config`, { headers: { Cookie: cookie } })).json();
      const desired = structuredClone(active);
      desired.apps.http.servers.demo.routes = [{ match: [{ host: ['new.example.com'] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:3030' }] }] }];
      const call = () => fetch(`${origin}/api/apply`, { method: 'POST', headers, body: JSON.stringify({ confirm: 'APPLY', baseFingerprint: config.fingerprint, content: JSON.stringify(desired) }) });
      const success = await call(); assert.equal(success.status, 200); assert.equal((await success.json()).changed, true);
      assert.equal(loadCount, 1); assert.equal(active.apps.http.servers.demo.routes.length, 1);
      const [snapshot] = await readdir(backup); assert.match(snapshot, /\.json$/);
      assert.equal((await stat(join(backup, snapshot))).mode & 0o777, 0o600);
      const stale = await call(); assert.equal(stale.status, 409); assert.equal(loadCount, 1);
    });
  } finally {
    await new Promise(resolve => stub.close(resolve));
    await rm(backup, { recursive: true, force: true });
  }
});
