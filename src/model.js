// Browser-side draft helpers. Deliberately self-contained to avoid runtime dependencies.
export function hostIsValid(value) {
  return /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(String(value).trim().toLowerCase());
}
export function dialIsValid(value) {
  const match = String(value).trim().match(/^([a-zA-Z0-9.-]+|\[[a-fA-F0-9:]+\]):([0-9]{1,5})$/);
  return Boolean(match && Number(match[2]) >= 1 && Number(match[2]) <= 65535);
}
export function withProxyDraft(source, { host, dial, server }) {
  if (!hostIsValid(host)) throw new Error('Enter a fully qualified hostname');
  if (!dialIsValid(dial)) throw new Error('Enter an upstream host and port');
  const next = structuredClone(source);
  const servers = next?.apps?.http?.servers;
  if (!servers?.[server] || !Array.isArray(servers[server].routes)) throw new Error('Choose a configured HTTP server');
  const name = host.trim().toLowerCase();
  if (Object.values(servers).some(s => JSON.stringify(s?.routes ?? []).includes(`"${name}"`))) throw new Error('Hostname already present in a route');
  servers[server].routes.unshift({ match: [{ host: [name] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: dial.trim() }] }], terminal: true });
  return next;
}
export function pretty(value) { return JSON.stringify(value, null, 2); }
export function formatNumber(value) { return typeof value === 'number' ? Intl.NumberFormat().format(value) : '—'; }
export function formatTime(value) { return value ? new Date(value).toLocaleString() : '—'; }
