export const demoConfig = {
  apps: { http: { servers: {
    public: { listen: [':443'], routes: [
      { match: [{ host: ['cloud.goreecloud.com'] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:8080' }] }], terminal: true },
      { match: [{ host: ['search.goreecloud.com'] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:8888' }] }], terminal: true },
      { match: [{ host: ['id.goreecloud.com'] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:3100' }] }], terminal: true },
      { match: [{ host: ['photos.goreecloud.com'] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:2283' }] }], terminal: true },
      { match: [{ host: ['files.goreecloud.com'] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:8090' }] }], terminal: true },
    ] },
    internal: { listen: [':80'], routes: [
      { match: [{ host: ['status.goreecloud.com'] }], handle: [{ handler: 'reverse_proxy', upstreams: [{ dial: '127.0.0.1:3001' }] }], terminal: true },
    ] },
  } } },
};
export const demoUpstreams = { upstreams: [
  { address: '127.0.0.1:8080', num_requests: 13, fails: 0 },
  { address: '127.0.0.1:8888', num_requests: 3, fails: 0 },
  { address: '127.0.0.1:3100', num_requests: 2, fails: 0 },
  { address: '127.0.0.1:2283', num_requests: 3, fails: 0 },
  { address: '127.0.0.1:8090', num_requests: 7, fails: 0 },
] };
export const demoMetrics = 'caddy_http_requests_total{server="public"} 12584\ncaddy_http_requests_total{server="internal"} 904\n';
