# GoreeCloud Gateway — GoreeCloud UI semantic source and browser acceptance

**Evidence date:** November 20, 2026. **Product status:** Development, partial browser adoption only; **not** an approved Stable / Anchor release, founder visual approval, a TLS production deployment, or actual protected Caddy route acceptance.

## Source, dependency and trust boundary

- Canonical visual source: [GoreeCloud/gc-ui](https://github.com/GoreeCloud/gc-ui) protected-main alpha revision `be8b819748d1ef0f6b2d3e786f4b22864cb3d07d`. The exact first-party `src/tokens.css` and MIT `LICENSE` are copied as immutable source into `web/src/vendor/gc-ui/` with SHA-256 hashes in `provenance.json`. Two Node tests enforce hash integrity, CSS-before-app styling, theme scopes and preserved Gateway-specific route composition.
- `data-gc-theme="dark"` / `data-gc-theme="light"` select the **actual** alpha Iris palette; the existing Gateway dark-default and explicit theme toggle remain intentional. Product-specific names `--canvas`, `--brand`, `--onbrand`, `--surface`, `--radius` etc. resolve to reviewed first-party `--gc-*` roles. Gateway does not import any mandatory unverified hues, increase runtime package dependencies, make external network token requests, or add cloud telemetry.
- Gateway's own console and native Caddy engine still own HTTP routes, configuration, authentication and TLS. **GoreeCloud UI is the design pillar, not GoreeShield privacy, Wardveil cybersecurity, Mesh networking or Identity access control.** No source change affects the Go management API, Caddy, DNS-01 credentials, persistent site files or service deployment. CSS changes must not be described as a runtime MIPS audit.

## Automated and interactive verification actually executed

- Locked `npm ci --ignore-scripts`, `npm run test:source`: **2/2 Node source/hash tests PASS**; `npm run build`: TypeScript and esbuild **PASS**, output `web/dist/` updated (versioned, embedded by the native service).
- Existing `web/e2e/console-accessibility.spec.mjs` uses an in-process HTTP server bound to a random **127.0.0.1** port and serves built local `dist` and fake Gateway API status/routes. The example hostname `fixture.example.test` and secret `fixture-test-operator-token-32-chars` are **synthetic**. A fake route save never reaches a running Gateway/Caddy server, DNS or a live site.
- With existing local Playwright 1.65.1 and installed browser engines, the original keyboard, invalid-token, modal trap/return, mobile nav, and multi-route WCAG A/AA axe-core checks and the new exact light/dark Iris semantic role + 390px/320px no-overflow checks passed on **Chromium, Firefox, Linux WebKit: 18/18 real-engine tests** (6 per engine). E2E checks verified no user token in localStorage, appropriate mobile menu hiding and focus restoration; axe automation is not human assistive-technology acceptance.
- An **opt-in screenshot test** (`GC_QA_SCREENSHOTS_DIR` must be an existing absolute directory) captures synthetic desktop dark overview, desktop light routes, mobile dark open-nav, and compact 320px light routes. The mobile menu snapshot is taken only when its slide-in transform has settled and its bounding box lies wholly within the viewport. The first pre-wait mobile capture was visibly clipped *mid-animation*; this was a test capture timing issue, not verified final-layout clipping. The corrected capture was visually inspected alongside desktop and compact screens, with intentional Gateway-specific layouts retained.
- A physical, authorized USB-connected **OnePlus N200 (DE2117), Android 16/API 36**, loaded only the **blank static development Gateway login page** from a temporary local Python HTTP server (`127.0.0.1:18758`) forwarded through a temporary ADB USB reverse tunnel. Both **Android Chrome 157.0.8103.80** and **Firefox 160.0.1** exposed `Welcome to your gateway.`, `Management token` and `Unlock console` in Android's UI accessibility hierarchy. This confirms physical web rendering and label exposure, **not a human TalkBack session**, signed Android product, authenticated Gateway console, successful login, or real native app.
- A real-phone Chrome screenshot was cropped to exclude status bar, browser address bar and gesture indicators; the token field was empty, no actual site/certificate/user record was shown, and no production traffic was sent. Test files/tunnel/server are temporary and must be cleared after verification.

### Reviewed screenshot evidence

- [Chromium synthetic desktop dark overview](evidence/gateway-chromium-desktop-dark.png)
- [Chromium synthetic desktop light routes](evidence/gateway-chromium-desktop-light-routes.png)
- [Chromium synthetic 390px dark mobile navigation (after animation)](evidence/gateway-chromium-mobile-dark-navigation.png)
- [Chromium synthetic 320px light route table](evidence/gateway-chromium-compact-light-routes.png)
- [Actual Android 16 Chrome blank login, sanitized](evidence/gateway-android16-chrome-physical.png)

## Reproducible reviewer workflow

From `web/`, run `npm ci --ignore-scripts && npm run test:source && npm run build`, then install an **inspected** Playwright browser toolchain consistent with the repository lock and run `npm run test:e2e -- --project=chromium`; repeat with `firefox` and `webkit` when those engines are installed. This exercises only synthetic local HTTP fixtures. To capture screenshots, first create an isolated review directory and set `GC_QA_SCREENSHOTS_DIR=/absolute/existing/review-dir`, then run the same E2E command. Without the variable, the optional screenshot case is skipped.

The source is suitable for protected review after both required Gateway CI jobs, **`validate` and `arm64-runtime`**, complete successfully on the exact proposed head. Tests/builds do not grant founder review approval or authorize production installation or migration.

## Explicitly open gates

Actual Caddyfile parity, state/rollback restoration, production staging TLS, route update with auth error handling and real certificate flows; long translations/RTL, true Firefox and Safari 200–400% zoom, official Apple Safari on macOS/iOS and a maintained browser support matrix; human TalkBack/Orca/NVDA/VoiceOver spoken checks; founder premium visual verdict; cross-consumer design adoption; native app/device functional acceptance; and any Stable / Anchor release certification are **NOT VERIFIED**. The isolated fixture's green checks cannot close those obligations. Rollback is an ordinary protected source revert, not a live configuration restore.
