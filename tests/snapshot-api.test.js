import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createGatewayServer } from '../server/index.js';

test('authenticated snapshot staging does not write to Caddy', async () => {
  const folder = await mkdtemp(join(tmpdir(), 'gateway-api-snapshots-'));
  const name = '2026-10-27T20-00-00.000Z-0123456789.json';
  const backup = { apps: { http: { servers: { old: { listen: [':443'], routes: [] } } } } };
  const current = { apps: { http: { servers: { live: { listen: [':443'], routes: [] } } } } };
  await writeFile(join(folder, name), JSON.stringify(backup), { mode: 0o600 });
  let writes = 0;
  const caddy = http.createServer((req, res) => {
    if (req.method === 'GET' && req.url === '/config/') {
      res.setHeader('content-type', 'application/json');
      return res.end(JSON.stringify(current));
    }
    if (req.method === 'POST') writes++;
    res.statusCode = 404;
    res.end();
  });
  await new Promise(resolve => caddy.listen(0, '127.0.0.1', resolve));
  const gateway = createGatewayServer({
    GATEWAY_ACCESS_TOKEN: 'x'.repeat(36),
    CADDY_ADMIN_URL: `http://127.0.0.1:${caddy.address().port}`,
    GATEWAY_BACKUP_DIR: folder,
  });
  await new Promise(resolve => gateway.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${gateway.address().port}`;
  try {
    assert.equal((await fetch(`${origin}/api/backups`)).status, 401);
    const login = await fetch(`${origin}/api/login`, {
      method: 'POST',
      headers: { Origin: origin, 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: 'x'.repeat(36) }),
    });
    const auth = await login.json();
    const cookie = login.headers.get('set-cookie').split(';')[0];
    const listed = await (await fetch(`${origin}/api/backups`, { headers: { Cookie: cookie } })).json();
    assert.equal(listed.snapshots.length, 1);
    assert.equal(listed.snapshots[0].name, name);
    const endpoint = `${origin}/api/restore-draft`;
    const basicHeaders = { Origin: origin, Cookie: cookie, 'Content-Type': 'application/json' };
    const csrfRejected = await fetch(endpoint, {
      method: 'POST', headers: basicHeaders, body: JSON.stringify({ name }),
    });
    assert.equal(csrfRejected.status, 403);
    const headers = { ...basicHeaders, 'X-CSRF-Token': auth.csrf };
    const invalid = await fetch(endpoint, {
      method: 'POST', headers, body: JSON.stringify({ name: '../outside.json' }),
    });
    assert.equal(invalid.status, 400);
    const staged = await fetch(endpoint, { method: 'POST', headers, body: JSON.stringify({ name }) });
    assert.equal(staged.status, 200);
    const response = await staged.json();
    assert.deepEqual(JSON.parse(response.content), backup);
    assert.match(response.baseFingerprint, /^[a-f0-9]{64}$/);
    assert.equal(writes, 0);
    assert.deepEqual(current.apps.http.servers.live.listen, [':443']);
  } finally {
    await new Promise(resolve => gateway.close(resolve));
    await new Promise(resolve => caddy.close(resolve));
    await rm(folder, { recursive: true, force: true });
  }
});
