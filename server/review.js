import { fingerprint } from './caddy.js';

// A bounded structural comparison. Paths and node types are surfaced,
// but neither old nor proposed configuration values are ever returned.
const LIMIT = 64;
const NODE_BUDGET = 12000;
const DEPTH_LIMIT = 32;
const sensitive = /(?:password|passwd|secret|token|credential|private|authorization|api[_-]?key|cookie)/i;

function kind(value) {
  return Array.isArray(value) ? 'array' : value === null ? 'null' : typeof value;
}

function safePath(parts) {
  if (!parts.length) return '/';
  // Protect unusual user-defined property names that could contain secrets.
  const encoded = parts.map(part => {
    const text = String(part);
    const label = sensitive.test(text) ? '[sensitive key]' : text.slice(0, 90);
    return label.replaceAll('~', '~0').replaceAll('/', '~1');
  });
  const path = '/' + encoded.join('/');
  return path.length > 220 ? path.slice(0, 217) + '…' : path;
}

export function reviewChanges(live, draft) {
  const changes = [];
  const counts = { added: 0, removed: 0, modified: 0 };
  let visited = 0;
  let truncated = false;

  function record(operation, path, before, after, note = '') {
    counts[operation]++;
    if (changes.length < LIMIT) {
      changes.push({ operation, path: safePath(path), beforeType: kind(before), afterType: kind(after), note });
    } else truncated = true;
  }

  function walk(before, after, path = [], depth = 0) {
    if (++visited > NODE_BUDGET || depth > DEPTH_LIMIT) {
      truncated = true;
      return;
    }
    if (Object.is(before, after)) return;
    const beforeKind = kind(before);
    const afterKind = kind(after);
    if (beforeKind !== afterKind || before === null || after === null || !['object', 'array'].includes(beforeKind)) {
      record('modified', path, before, after);
      return;
    }
    if (Array.isArray(before)) {
      if (before.length !== after.length) {
        record('modified', path, before, after, `Array length ${before.length} → ${after.length}`);
        return;
      }
      for (let i = 0; i < before.length; i++) walk(before[i], after[i], [...path, i], depth + 1);
      return;
    }
    const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
    for (const key of [...keys].sort()) {
      if (++visited > NODE_BUDGET) { truncated = true; return; }
      if (!Object.hasOwn(before, key)) record('added', [...path, key], undefined, after[key]);
      else if (!Object.hasOwn(after, key)) record('removed', [...path, key], before[key], undefined);
      else walk(before[key], after[key], [...path, key], depth + 1);
    }
  }

  walk(live, draft);
  const total = counts.added + counts.removed + counts.modified;
  const impactedPaths = changes.map(c => c.path);
  const warnings = [];
  if (impactedPaths.some(p => p.startsWith('/apps/http'))) warnings.push('HTTP server, listener or routing changes can affect live requests and TLS behavior.');
  if (impactedPaths.some(p => p.startsWith('/apps/tls') || p.startsWith('/pki'))) warnings.push('TLS or PKI changes may affect certificates and trust.');
  if (impactedPaths.some(p => p.startsWith('/admin'))) warnings.push('Admin API changes may affect control-plane connectivity.');
  if (truncated) warnings.push('This preview was capped. Review the complete JSON draft before any reload.');
  return {
    changed: fingerprint(live) !== fingerprint(draft),
    liveFingerprint: fingerprint(live),
    draftFingerprint: fingerprint(draft),
    counts,
    total,
    changes,
    truncated,
    warnings,
    scope: 'Structural JSON paths and types only; values are not included. Caddy runtime behavior is not validated.',
  };
}
