# GoreeCloud Gateway

I am building **GoreeCloud Gateway** as GoreeCloud's original, privacy-first web control plane for the [Caddy](https://caddyserver.com/) web server. My guiding requirements are useful operational visibility, deliberate changes, clear distinctions between live and demo data, and protection of Caddy's privileged administrative interface. Gateway follows the Canvas / Surface / Lens direction of [GoreeCloud UI](https://github.com/GoreeCloud/gc-ui), without asserting certification or implementation of all GC UI components.

**Status:** `0.2.1` early development / alpha. The web UI and local management service are implemented. This release has not been deployed to GoreeCloud infrastructure and is not certified production ready.

## Features implemented

- Original adaptive dark/light dashboard, responsive navigation, keyboard route search, user-friendly empty and failure states, and semantic controls.
- Caddy live HTTP server/host/route discovery from `GET /config/`, available upstream counters from `/reverse_proxy/upstreams`, and optional `/metrics` counters. Metrics are explicitly identified as counters, not synthetic request rates.
- Source-aware telemetry: upstream status and metrics are reported independently as available, unavailable, invalid or synthetic. Missing endpoints never imply zero failures or healthy upstreams.
- Route search/filter/detail inspection and a draft-only reverse-proxy route composer for existing Caddy HTTP servers.
- Protected configuration studio with raw JSON draft editing, validation of **JSON syntax and basic structure only**, explicit draft download, redacted structural change previews, base-revision conflict protection, and guarded live reload.
- Safe default: **read only**. Write mode requires `GATEWAY_ALLOW_APPLY=1`; each live reload also requires a successful server-side review of the **exact** draft and live base revision, a typed `APPLY` confirmation, a rechecked live revision, and a private pre-change snapshot.
- Server-side only connection to Caddy's loopback admin API; zero third-party runtime or external CDN dependency; secure HttpOnly SameSite session cookies, CSRF/origin checks and sign-in rate limiting.
- In-memory gateway activity view (not a durable audit log) and an unmistakably marked synthetic preview mode.
- Automated security, parsing, integration and build checks in GitHub Actions.

## Quick start

Requires **Node.js 20+** and a Caddy instance with its admin API **on loopback**.

```sh
npm run build
# A long, random secret. Generate one on a trusted local machine.
export GATEWAY_ACCESS_TOKEN="$(node -e "console.log(require('crypto').randomBytes(40).toString('base64url'))")"
export CADDY_ADMIN_URL=http://127.0.0.1:2019
npm start
# Visit http://127.0.0.1:3088
```

No `npm install` is needed; it uses Node built-ins and original browser JavaScript/CSS. To preview without Caddy, use `GATEWAY_DEMO=1 npm start`. Preview routes and counters are **fabricated examples**, not statements about GoreeCloud's live infrastructure. Applying changes is always disabled in demo mode.

### Protected remote access

Keep `GATEWAY_HOST=127.0.0.1` and the Caddy admin API on loopback. Expose the Gateway only through a correctly configured private-network or authenticated HTTPS reverse proxy. Configure the **exact browser-facing HTTPS origin**:

```sh
export GATEWAY_PUBLIC_ORIGIN=https://gateway.example.com
```

The gateway checks browser `Origin` and `Host` values, does not enable CORS, applies a restrictive Content Security Policy, and sets `Secure` on session cookies when `GATEWAY_PUBLIC_ORIGIN` is set. Configure a real trusted HTTPS termination layer; this project does not provide TLS termination itself. Do not put admin secrets in URLs or the repository, and do not expose Caddy's admin API directly to the internet.

### Carefully opt into live config reloads

```sh
export GATEWAY_ALLOW_APPLY=1
export GATEWAY_BACKUP_DIR=/path/to/private/backups
npm start
```

Writes are forbidden by default. The editor uses JSON, which is Caddy's native configuration format. Gateway verifies the current config fingerprint immediately before POSTing to Caddy's `/load`. It saves a snapshot of the prior config with mode `0600` to a private directory; protect that directory with OS permissions and backups because snapshots can contain secrets. Caddy `/load` itself performs a transactional config swap, but valid JSON is not proof that proposed routing/TLS settings will behave as intended. Recheck routing and TLS on a nonproduction instance first.

### Preview changes without applying

In the Configuration studio, choose **Preview changes** to ask Gateway to compare the edited JSON against the current Caddy API revision. The server returns structural JSON paths, change counts and impact warnings; neither old nor proposed values are included in the response. The preview is bounded (64 change entries and a node traversal budget), and an array that grows or shrinks is summarized by its length instead of exposing every item. This is **not** semantic Caddy validation, a guarantee of traffic safety, or proof of certificate renewal success.

When live reloads are enabled, **Review & apply** must review the exact draft and revision before accepting the separate typed APPLY confirmation. Editing the JSON after preview requires reviewing again. A concurrent change detected during Gateway's pre-apply read causes a conflict. There remains a small race if an independent client changes Caddy after the pre-apply check but before the /load request; do not operate unsynchronized writers against the same instance. Coordinate a single configuration authority or add a tested Caddy ETag-based transaction strategy before production deployment.

**Important source-of-truth warning:** Caddy can be operated with file/CLI or API-managed configuration. Do not mix conflicting controllers. Gateway's live write mode uses `/load` and may diverge from a Caddyfile managed by other tooling. See Caddy's [API documentation](https://caddyserver.com/docs/api) and [getting-started guide](https://caddyserver.com/docs/getting-started).

## Commands

| Command | Purpose |
|---|---|
| `npm run dev` | Run the loopback server, serving built assets when available |
| `npm run build` | Produce standalone `dist/` browser assets |
| `npm run check` | Parse-check shipped JavaScript modules |
| `npm test` | Run Node's built-in test suite |
| `npm run ci` | Check + test + build |

## Architecture and boundaries

```text
Browser (original GC UI-aligned HTML/CSS/JS)
       │ same-origin authenticated requests
       ▼
Gateway Node HTTP service (127.0.0.1:3088)
       │ fixed loopback-only CADDY_ADMIN_URL
       ▼
Caddy Admin API (127.0.0.1:2019)
```

The browser never receives Caddy's admin API address as an access path and never persists token or configuration drafts to local storage. It can display sensitive configuration after login, so trusted devices, protected ingress and short sessions remain important. This alpha uses single-operator authentication (one environment secret); it does not implement external identity federation, multiuser RBAC, distributed sessions or durable audit trails. The admin API URL cannot target arbitrary external hosts.

The proxy composer only adds host-based reverse proxies to an **existing** HTTP server. It does not automatically create listeners, certificates, DNS records, upstream security policies or Caddy TLS automation. Those require separate review.

## Project roadmap

1. **Security + operator QA:** authenticated reverse-proxy end-to-end tests, session lifecycle/lockouts, backup restoration rehearsal, Caddy runtime validation and integration tests against supported Caddy versions.
2. **Routing:** dedicated site wizard with explicit listeners/TLS policy, route precedence visualizer, enhanced semantic diff and backup rollback review.
3. **Observability:** time-series collector and retention policy, latency/error-rate charts backed by measured samples, health-check summaries, certificate state visibility, and meaningful alerts.
4. **Operations:** role-based access, external auth integration, durable append-only audit, encrypted backup strategy, feature flags and deployment/rollback playbooks.
5. **Design quality:** screenshot-led GC UI comparison, keyboard and screen-reader acceptance, mobile/tablet checks, contrast and reduced-motion QA.

## Public repository and licensing

This repository is intentionally **public**, as required for GoreeCloud Gateway. The current package metadata is `UNLICENSED`, so public visibility is not a grant of open-source redistribution rights. I will choose and document a license separately before making open-source licensing claims. Never commit live Caddy JSON, tokens, certificate keys, or backup contents.

## Repository policy

Keep secrets, private Caddy configs, TLS keys, backups, `.env`, `data/`, and generated build artifacts out of Git. Documentation is Markdown. Features are not considered production-stable on the basis of a passing unit test alone.
