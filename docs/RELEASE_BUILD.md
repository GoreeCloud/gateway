# Local, source-pinned Linux release packaging

Run the reviewed [packaging script](../scripts/package_linux_release.sh) from a **clean Git checkout** after checking the exact protected source revision and the two required `validate` and `arm64-runtime` CI checks.

The script is a **local packaging helper**. It does not create Git tags, publish GitHub releases, sign artifacts, deploy applications, restart services, or contact DNS/ACME. It does install pinned npm dependencies with lifecycle scripts disabled and runs the Go/TypeScript/test/audit gates; run it only on a trusted development machine with network access and known-good Go, Node 24, npm, Python 3, curl, and GNU/Linux packaging utilities.

```sh
git rev-parse HEAD
git status --short
bash scripts/package_linux_release.sh 0.1.0-alpha.7 /tmp/gateway-alpha7-candidate
cd /tmp/gateway-alpha7-candidate
sha256sum -c SHA256SUMS.txt
```

The output path must **not exist** beforehand and should be outside the Git checkout. The script stages each static Linux AMD64/ARM64 executable, embeds the explicit version string, copies attribution, docs and examples, checks module metadata, creates both archives, and generates `SHA256SUMS.txt` with archive integrity and path checks. It refuses to package a dirty source tree or silently overwrite an output folder. The archive staging uses umask 0077; only the isolated Go test phase restores a 0022 fixture umask so permission-rejection tests remain meaningful.

Before publishing any candidate, a maintainer must independently check the exact source SHA and vulnerability findings, confirm required protected CI, inspect license notices, verify actual binaries and archive contents, and ensure the release target and notes exactly match the packaged source. GitHub release publication is a **separate conscious step**, not performed by this script. A matching checksum is *not* a signature or proof of reproducibility or runtime security.

No build script, CI pass or release replaces independent real-device tests, protected production backup restoration, TLS and DNS-01 issuance/renewal, actual route/security parity, verified rollback, or specific change authorization. The published alpha.6 remains a preproduction development build; future version numbers are examples, not promises.
