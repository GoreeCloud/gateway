import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewChanges } from '../server/review.js';

test('structural review classifies additions/removals/changes without echoing config values', () => {
  const live = { apps: { http: { servers: { public: { routes: [{ handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:8080' }] }] }] } } } }, secret: 'old-sensitive-value', removed: 'confidential-old' };
  const draft = structuredClone(live);
  draft.secret = 'new-sensitive-value';
  delete draft.removed;
  draft.added = 'private-new-data';
  draft.apps.http.servers.public.routes[0].handle[0].upstreams[0].dial = 'internal-sensitive-upstream:9000';
  const review = reviewChanges(live, draft);
  assert.equal(review.changed, true);
  assert.deepEqual(review.counts, { added: 1, removed: 1, modified: 2 });
  assert.ok(review.changes.some(change => change.path.includes('[sensitive key]')));
  assert.ok(review.warnings.some(warning => warning.includes('routing')));
  const rendered = JSON.stringify(review);
  for (const secret of ['old-sensitive-value', 'new-sensitive-value', 'confidential-old', 'private-new-data', 'internal-sensitive-upstream:9000']) {
    assert.equal(rendered.includes(secret), false, secret);
  }
});

test('review ignores object key order and masks sensitive key names', () => {
  const live = { b: 1, secret_API_KEY: 'secret-value-a', nested: { y: 2, x: 1 } };
  const same = { nested: { x: 1, y: 2 }, secret_API_KEY: 'secret-value-a', b: 1 };
  assert.equal(reviewChanges(live, same).changed, false);
  assert.equal(reviewChanges(live, same).total, 0);
  const modified = reviewChanges(live, { ...same, secret_API_KEY: 'secret-value-b' });
  assert.match(modified.changes[0].path, /sensitive key/);
  assert.doesNotMatch(JSON.stringify(modified), /secret_API_KEY|secret-value/);
});

test('arrays of changed length get concise content-free summaries', () => {
  const review = reviewChanges({ routes: [{ name: 'very-private-host' }] }, { routes: [{ name: 'very-private-host' }, { name: 'another-secret-host' }] });
  assert.equal(review.total, 1);
  assert.equal(review.changes[0].note, 'Array length 1 → 2');
  assert.doesNotMatch(JSON.stringify(review), /very-private|another-secret/);
});

test('review caps large changes while remaining bounded', () => {
  const live = {};
  const draft = {};
  for (let i = 0; i < 130; i++) draft['entry' + i] = 'secret-' + i;
  const result = reviewChanges(live, draft);
  assert.equal(result.changed, true);
  assert.equal(result.total, 130);
  assert.equal(result.changes.length, 64);
  assert.equal(result.truncated, true);
});
