import test from 'node:test';
import assert from 'node:assert/strict';
import { addProxyRoute, extractRoutes, fingerprint, parseMetrics, summarize, validateAdminUrl, validateDraft, validateHostname, validateDial } from '../server/caddy.js';
import { demoConfig, demoUpstreams, demoMetrics } from '../server/demo.js';
import { withProxyDraft } from '../src/model.js';

test('config hashing is order independent', () => assert.equal(fingerprint({ a: 1, b: { z: 2, x: 3 } }), fingerprint({ b: { x: 3, z: 2 }, a: 1 })));
test('config editor blocks invalid root and JSON', () => {
  assert.throws(() => validateDraft('[]'), /object/);
  assert.throws(() => validateDraft('{broken'), /Invalid JSON/);
  assert.deepEqual(validateDraft('{}'), {});
});
test('Caddy admin endpoint must remain on loopback', () => {
  assert.equal(validateAdminUrl('http://127.0.0.1:2019'), 'http://127.0.0.1:2019');
  for (const value of ['https://127.0.0.1:2019', 'http://example.com:2019', 'http://user:pass@localhost:2019', 'http://localhost:2019/config']) assert.throws(() => validateAdminUrl(value), /loopback/);
});
test('extracts domain routes and upstreams from Caddy JSON', () => {
  const routes = extractRoutes(demoConfig);
  assert.equal(routes.length, 6);
  assert.ok(routes.some(r => r.hosts.includes('search.goreecloud.com') && r.upstreams.includes('127.0.0.1:8888')));
});
test('metrics parser ignores comments and nonnumeric lines', () => {
  assert.equal(parseMetrics(demoMetrics).caddy_http_requests_total, 13488);
  assert.deepEqual(parseMetrics('# HELP unrelated\ninvalid line'), {});
});
test('overview computes distinct sites and operational counters', () => {
  const o = summarize(demoConfig, demoUpstreams, demoMetrics);
  assert.equal(o.servers, 2); assert.equal(o.sites, 6); assert.equal(o.routeCount, 6); assert.equal(o.upstreams.length, 5);
});
test('draft route insertion never mutates original config', () => {
  const before = fingerprint(demoConfig);
  const next = addProxyRoute(demoConfig, { hostname: 'new.example.com', upstream: 'localhost:5000', server: 'public' });
  assert.equal(fingerprint(demoConfig), before);
  assert.equal(extractRoutes(next).length, 7);
  assert.equal(next.apps.http.servers.public.routes[0].match[0].host[0], 'new.example.com');
});
test('route composer rejects duplicate and invalid host/upstream', () => {
  assert.throws(() => addProxyRoute(demoConfig, { hostname: 'search.goreecloud.com', upstream: 'localhost:5000', server: 'public' }), /already/);
  assert.throws(() => validateHostname('http://example.com'), /hostname/);
  assert.throws(() => validateDial('localhost:99999'), /host/);
});
test('browser draft composer and server composer produce compatible outputs', () => {
  const args = { host: 'new.example.com', dial: 'localhost:7000', server: 'public' };
  const browser = withProxyDraft(demoConfig, args);
  const server = addProxyRoute(demoConfig, { hostname: args.host, upstream: args.dial, server: args.server });
  assert.equal(fingerprint(browser), fingerprint(server));
});
