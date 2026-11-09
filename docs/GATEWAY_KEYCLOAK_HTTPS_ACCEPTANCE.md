# Gateway — Real Keycloak HTTPS authorization acceptance

**Status:** Test harness added; **real Keycloak-to-Go acceptance has NOT yet passed**. This is source-only development work, not a production SSO deployment.

## Why a separate test?

The [Identity CI contract](https://github.com/GoreeCloud/identity/actions/runs/41738382885) has proven that disposable Keycloak issues a signed OAuth access token with the required Gateway resource audience, OAuth client, role, scope, and tenant claims. Gateway's actual Go verifier intentionally requires an **HTTPS issuer** and rejects the loopback HTTP issuer used by the original Identity test. Substituting a weaker issuer or disabling TLS verification would fail to test the production trust boundary.

## Gated Go harness

`cmd/console/keycloak_real_integration_test.go` is compiled only under the `keycloak_integration` build tag and is skipped unless all of these are exact: `CI=true`, `GITHUB_ACTIONS=true`, `GITHUB_REPOSITORY=GoreeCloud/identity`, and `GOREE_TEST_KEYCLOAK_HTTPS_GATEWAY=1`.

It expects one disposable Keycloak issuer at `https://localhost:8443/realms/goreecloud` and strictly permissioned local files:

| Path under `/tmp/goreecloud-keycloak-gateway-ci/` | Purpose |
|---|---|
| `tls.crt` | Short-lived localhost self-signed certificate to trust **only inside this test's TLS client** |
| `access.jwt` | Real disposable Keycloak access token, never logged |
| `id.jwt` | Real disposable Keycloak ID token, used only to prove it is *not* an API bearer token |

The test uses Go's validated TLS stack with a test-scoped certificate root, the existing `newGatewayOIDCAuth` (including issuer/JWKS pinning), and the real `newHandlerWithOIDC` read-only overview API. A valid signed resource token and the historical static token must both pass. ID-token substitution, a modified signature, wrong audience, wrong OAuth client, wrong tenant, missing and malformed tokens must fail.

The Gateway CI job uses `go test -tags=keycloak_integration ./cmd/console -run "^TestRealKeycloakGatewayHTTPSAccessToken$"` to **compile** the test and confirm that it cannot execute without the authorized Identity CI gate. Passing that job does not mean actual Keycloak was started.

## Acceptance gates still pending

1. In a fresh GitHub-hosted GoreeCloud Identity job only, generate and trust a short-lived localhost certificate, launch a disposable **HTTPS** Keycloak realm with a synthetic client, and issue the access/ID tokens using Authorization Code + S256 PKCE. Never use production credentials or a live Keycloak realm.
2. Verify the exact Keycloak issuer and Gateway resource claims, place the real tokens in owner-only files, and run the pinned Gateway Go integration test with its explicit CI gate. Confirm both positive and negative cases pass.
3. Collect exact-commit CI evidence and securely destroy synthetic credentials, tokens, certificates, and temporary database containers.
4. Separately review production HTTPS, independent resource ownership, online revocation, MFA, distributed sessions, Android/browser login, off-host recovery, privacy and GoreeCloud UI acceptance.

Do not configure production proxy trust, expose Gateway's management port, or turn on optional Gateway OIDC merely to satisfy these tests. No production changes are authorized by this document.

**Tracking:** [Gateway issue #14](https://github.com/GoreeCloud/gateway/issues/14), [Identity issue #2](https://github.com/GoreeCloud/identity/issues/2).
