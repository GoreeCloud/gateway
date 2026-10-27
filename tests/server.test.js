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
test('preview API rejects invalid JSON safely and requires a CSRF token', async () => {
  await withServer({ GATEWAY_DEMO: '1' }, async origin => {
    const options = {
      method: 'POST',
      headers: { Origin: origin, 'Content-Type': 'application/json', 'X-CSRF-Token': 'demo' },
      body: JSON.stringify({ baseFingerprint: 'irrelevant', content: '{invalid' }),
    };
    const invalid = await fetch(`${origin}/api/review`, options);
    assert.equal(invalid.status, 400);
    const invalidBody = await invalid.text();
    assert.doesNotMatch(invalidBody, /SyntaxError|at position|unexpected token/i);
    const missingCSRF = await fetch(`${origin}/api/review`, {
      ...options, headers: { Origin: origin, 'Content-Type': 'application/json' },
    });
    assert.equal(missingCSRF.status, 403);
  });
});
test('live preview data requires authenticated session', async () => {
  await withServer({ GATEWAY_ACCESS_TOKEN: 'a'.repeat(36) }, async origin => {
    const response = await fetch(`${origin}/api/review`, {
      method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json' },
      body: JSON.stringify({ baseFingerprint: 'x', content: '{}' }),
    });
    assert.equal(response.status, 401);
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
  const { mkdtemp, readdir, stat, rm, readFile } = await import('node:fs/promises');
  const { tmpdir } = await import('node:os');
  const { join } = await import('node:path');
  let active = { apps: { http: { servers: { demo: { listen: [':443'], routes: [] } } } } };
  let loadCount = 0;
  let outsideRevisions = 0;
  let simulateRace = false;
  let omitEtag = false;
  const etag = () => `"/config/ ${(loadCount + outsideRevisions).toString(16).padStart(16, '0')}"`;
  const stub = http.createServer(async (req, res) => {
    if (req.url === '/config/' && req.method === 'GET') { res.setHeader('content-type', 'application/json'); if (!omitEtag) res.setHeader('etag', etag()); return res.end(JSON.stringify(active)); }
    if (req.url === '/reverse_proxy/upstreams') return res.end(JSON.stringify({ upstreams: [] }));
    if (req.url === '/metrics') return res.end('caddy_http_requests_total 2\n');
    if (req.url === '/config/' && req.method === 'POST') {
      if (simulateRace) { simulateRace = false; outsideRevisions++; active.apps.http.servers.demo.listen = [':8443']; }
      if (req.headers['if-match'] !== etag()) { res.statusCode = 412; return res.end('revision conflict'); }
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
      const noReview = await call(); assert.equal(noReview.status, 409);
      assert.equal(loadCount, 0, 'unreviewed drafts must not reach Caddy');
      const reviewRequest = (content, baseFingerprint = config.fingerprint) => fetch(`${origin}/api/review`, {
        method: 'POST', headers, body: JSON.stringify({ baseFingerprint, content }),
      });
      const reviewed = await reviewRequest(JSON.stringify(desired));
      assert.equal(reviewed.status, 200);
      const changePreview = await reviewed.json();
      assert.equal(changePreview.changed, true);
      assert.equal(changePreview.counts.modified, 1);
      const tampered = structuredClone(desired);
      tampered.apps.http.servers.demo.listen = [':8443'];
      const unreviewedMutation = await fetch(`${origin}/api/apply`, {
        method: 'POST', headers,
        body: JSON.stringify({ confirm: 'APPLY', baseFingerprint: config.fingerprint, content: JSON.stringify(tampered) }),
      });
      assert.equal(unreviewedMutation.status, 409);
      assert.equal(loadCount, 0, 'review token is bound to exact draft content');
      const success = await call(); assert.equal(success.status, 200); assert.equal((await success.json()).changed, true);
      assert.equal(loadCount, 1); assert.equal(active.apps.http.servers.demo.routes.length, 1);
      const [snapshot] = await readdir(backup); assert.match(snapshot, /\.json$/);
      assert.equal((await stat(join(backup, snapshot))).mode & 0o777, 0o600);
      const savedOriginal = JSON.parse(await readFile(join(backup, snapshot), 'utf8'));
      assert.equal(savedOriginal.apps.http.servers.demo.routes.length, 0);
      const staleReview = await reviewRequest(JSON.stringify(desired));
      assert.equal(staleReview.status, 409);
      const stale = await call(); assert.equal(stale.status, 409); assert.equal(loadCount, 1);
      const current = await (await fetch(`${origin}/api/config`, { headers: { Cookie: cookie } })).json();
      const secondDraft = structuredClone(active);
      secondDraft.apps.http.servers.demo.listen = [':444'];
      const reviewedSecond = await reviewRequest(JSON.stringify(secondDraft), current.fingerprint);
      assert.equal(reviewedSecond.status, 200);
      simulateRace = true;
      const racedApply = await fetch(`${origin}/api/apply`, {
        method: 'POST', headers,
        body: JSON.stringify({ confirm: 'APPLY', baseFingerprint: current.fingerprint, content: JSON.stringify(secondDraft) }),
      });
      assert.equal(racedApply.status, 409);
      assert.equal(loadCount, 1, 'conditional write did not overwrite the concurrent update');
      assert.deepEqual(active.apps.http.servers.demo.listen, [':8443']);

      const latest = await (await fetch(`${origin}/api/config`, { headers: { Cookie: cookie } })).json();
      const thirdDraft = structuredClone(active);
      thirdDraft.apps.http.servers.demo.listen = [':443'];
      assert.equal((await reviewRequest(JSON.stringify(thirdDraft), latest.fingerprint)).status, 200);
      omitEtag = true;
      const unsupported = await fetch(`${origin}/api/apply`, {
        method: 'POST', headers,
        body: JSON.stringify({ confirm: 'APPLY', baseFingerprint: latest.fingerprint, content: JSON.stringify(thirdDraft) }),
      });
      assert.equal(unsupported.status, 409, 'missing ETag must refuse writes');
      assert.equal(loadCount, 1);
      omitEtag = false;
      const beforeRestore = await (await fetch(`${origin}/api/config`, { headers: { Cookie: cookie } })).json();
      assert.equal((await reviewRequest(JSON.stringify(savedOriginal), beforeRestore.fingerprint)).status, 200);
      const restored = await fetch(`${origin}/api/apply`, {
        method: 'POST', headers,
        body: JSON.stringify({ confirm: 'APPLY', baseFingerprint: beforeRestore.fingerprint, content: JSON.stringify(savedOriginal) }),
      });
      assert.equal(restored.status, 200);
      assert.equal((await restored.json()).verified, true);
      assert.deepEqual(active, savedOriginal);
      assert.equal(loadCount, 2);
    });
  } finally {
    await new Promise(resolve => stub.close(resolve));
    await rm(backup, { recursive: true, force: true });
  }
});

test('optional upstream and metrics failures are visible without misreporting zero health', async () => {
  const http = await import('node:http');
  let invalidUpstreams = false;
  const caddy = http.createServer((req, res) => {
    if (req.url === '/config/') {
      res.setHeader('content-type', 'application/json');
      return res.end(JSON.stringify({ apps: { http: { servers: {} } } }));
    }
    if (req.url === '/reverse_proxy/upstreams' && invalidUpstreams) {
      res.setHeader('content-type', 'application/json');
      return res.end('not-json');
    }
    res.statusCode = 404;
    return res.end('disabled');
  });
  await new Promise(resolve => caddy.listen(0, '127.0.0.1', resolve));
  try {
    await withServer({
      GATEWAY_ACCESS_TOKEN: 'z'.repeat(40),
      CADDY_ADMIN_URL: `http://127.0.0.1:${caddy.address().port}`,
    }, async origin => {
      const login = await fetch(`${origin}/api/login`, {
        method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: 'z'.repeat(40) }),
      });
      assert.equal(login.status, 200);
      const cookie = login.headers.get('set-cookie').split(';')[0];
      const sample = async () => {
        const result = await fetch(`${origin}/api/overview`, { headers: { Cookie: cookie } });
        assert.equal(result.status, 200);
        return result.json();
      };
      const unavailable = await sample();
      assert.equal(unavailable.connected, true);
      assert.equal(unavailable.sources.upstreams, 'unavailable');
      assert.equal(unavailable.sources.metrics, 'unavailable');
      assert.equal(unavailable.upstreams.length, 0);
      assert.deepEqual(unavailable.metrics, {});
      invalidUpstreams = true;
      const invalid = await sample();
      assert.equal(invalid.sources.upstreams, 'invalid');
      assert.equal(invalid.sources.metrics, 'unavailable');
    });
  } finally {
    await new Promise(resolve => caddy.close(resolve));
  }
});
