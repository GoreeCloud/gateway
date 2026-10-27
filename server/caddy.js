import { createHash } from 'node:crypto';

export function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])]));
  }
  return value;
}

export function fingerprint(value) {
  return createHash('sha256').update(JSON.stringify(canonical(value))).digest('hex');
}

export function validateAdminUrl(raw) {
  const url = new URL(raw);
  if (url.protocol !== 'http:' || !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || url.username || url.password || url.search || url.hash || (url.pathname !== '/' && url.pathname !== '')) {
    throw new Error('CADDY_ADMIN_URL must be a plain HTTP loopback origin (for example http://127.0.0.1:2019)');
  }
  return url.origin;
}

export function extractRoutes(config) {
  const servers = config?.apps?.http?.servers ?? {};
  const result = [];
  for (const [serverName, server] of Object.entries(servers)) {
    const visit = (routes, inheritedHosts = [], depth = 0) => {
      if (!Array.isArray(routes) || depth > 12) return;
      routes.forEach((route, index) => {
        const hosts = [...new Set([...(route.match ?? []).flatMap(match => Array.isArray(match.host) ? match.host : []), ...inheritedHosts])];
        const upstreams = [];
        const handlers = [];
        const inspectHandlers = (items, level = 0) => {
          if (!Array.isArray(items) || level > 12) return;
          items.forEach(h => {
            if (h?.handler) handlers.push(h.handler);
            if (h?.handler === 'reverse_proxy' && Array.isArray(h.upstreams)) {
              upstreams.push(...h.upstreams.map(u => u.dial).filter(Boolean));
            }
            if (h?.handler === 'subroute') visit(h.routes, hosts, depth + 1);
            if (h?.handler === 'route') inspectHandlers(h.routes, level + 1);
          });
        };
        inspectHandlers(route.handle);
        if (upstreams.length || hosts.length || handlers.length) {
          result.push({
            id: `${serverName}-${depth}-${index}-${result.length}`,
            server: serverName,
            hosts: [...new Set(hosts)],
            upstreams: [...new Set(upstreams)],
            handlers: [...new Set(handlers)],
            terminal: Boolean(route.terminal),
            listeners: server.listen ?? [],
          });
        }
      });
    };
    visit(server.routes ?? []);
  }
  return result;
}

export function parseMetrics(text) {
  const accepted = ['caddy_http_requests_total', 'caddy_http_response_size_bytes_count', 'caddy_reverse_proxy_upstreams_healthy'];
  const values = {};
  for (const line of String(text).split('\n')) {
    if (!line || line.startsWith('#')) continue;
    const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{[^}]*\})?\s+(-?\d+(?:\.\d+)?(?:e[+-]?\d+)?)(?:\s|$)/i);
    if (match && accepted.includes(match[1])) {
      values[match[1]] = (values[match[1]] ?? 0) + Number(match[2]);
    }
  }
  return values;
}

export function summarize(config, upstreamResponse, metricsText) {
  const routes = extractRoutes(config);
  const hosts = [...new Set(routes.flatMap(route => route.hosts))].sort();
  const upstreams = Array.isArray(upstreamResponse?.upstreams) ? upstreamResponse.upstreams : [];
  return {
    servers: Object.keys(config?.apps?.http?.servers ?? {}).length,
    sites: hosts.length,
    routeCount: routes.length,
    routes,
    hosts,
    upstreams: upstreams.map((upstream, index) => ({
      id: index,
      address: String(upstream.address ?? upstream.dial ?? 'Unknown'),
      numRequests: Number(upstream.num_requests ?? 0),
      fails: Number(upstream.fails ?? 0),
    })),
    metrics: parseMetrics(metricsText),
    fingerprint: fingerprint(config),
    updatedAt: new Date().toISOString(),
  };
}

export function validateDraft(input) {
  if (typeof input !== 'string' || input.length > 1024 * 1024) throw new Error('Configuration must be text under 1 MiB');
  let value;
  try { value = JSON.parse(input); } catch (err) { throw new Error(`Invalid JSON: ${err.message}`); }
  if (!value || Array.isArray(value) || typeof value !== 'object') throw new Error('Caddy configuration must be a JSON object');
  if (value.apps !== undefined && (value.apps === null || typeof value.apps !== 'object' || Array.isArray(value.apps))) throw new Error('The apps field must be an object');
  return value;
}

export function validateHostname(value) {
  const host = String(value).trim().toLowerCase();
  if (host.length > 253 || !/^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(host)) {
    throw new Error('Use a fully qualified hostname, such as app.example.com');
  }
  return host;
}

export function validateDial(value) {
  const dial = String(value).trim();
  const match = dial.match(/^([a-zA-Z0-9.-]+|\[[a-fA-F0-9:]+\]):([0-9]{1,5})$/);
  if (!match || Number(match[2]) < 1 || Number(match[2]) > 65535 || dial.length > 260) {
    throw new Error('Use a hostname or IP and port (for example 127.0.0.1:8080)');
  }
  return dial;
}

// Modifies a draft only. Never contacts or reloads Caddy.
export function addProxyRoute(config, { hostname, upstream, server }) {
  const host = validateHostname(hostname);
  const dial = validateDial(upstream);
  const next = structuredClone(config);
  const servers = next?.apps?.http?.servers;
  if (!servers || !servers[server] || !Array.isArray(servers[server].routes)) {
    throw new Error('Choose an existing Caddy HTTP server with a routes array');
  }
  if (extractRoutes(next).some(route => route.hosts.includes(host))) {
    throw new Error('This hostname is already configured in the draft');
  }
  servers[server].routes.unshift({
    match: [{ host: [host] }],
    handle: [{ handler: 'reverse_proxy', upstreams: [{ dial }] }],
    terminal: true,
  });
  return next;
}
