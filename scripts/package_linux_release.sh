#!/usr/bin/env bash
# Source-pinned, local-only Linux release packaging. Does not publish or deploy.
set -euo pipefail
umask 077

if [[ "$#" != 2 ]]; then
  echo "Usage: bash scripts/package_linux_release.sh 0.1.0-alpha.N /path/to/new-output-dir" >&2
  exit 2
fi
version="$1"
out="$2"
if [[ ! "$version" =~ ^0\.1\.0-alpha\.[1-9][0-9]*$ ]]; then
  echo "Only explicit 0.1.0-alpha.N prerelease versions are supported" >&2
  exit 2
fi
if [[ ! -d .git && ! -f .git ]]; then
  echo "Run from the Gateway source repository root" >&2
  exit 2
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Refusing to package a dirty Git worktree" >&2
  exit 2
fi
source_sha="$(git rev-parse --verify HEAD)"
if [[ -e "$out" || -L "$out" ]]; then
  echo "Refusing to overwrite an existing release output path: $out" >&2
  exit 2
fi
mkdir -m 0700 -- "$out"
out="$(cd "$out" && pwd -P)"
temp="$(mktemp -d "${TMPDIR:-/tmp}/goreecloud-gateway-package-XXXXXXXX")"
cleanup() { rm -rf -- "$temp"; }
trap cleanup EXIT

echo "Source commit: $source_sha"
echo "Release version: $version"
echo "Verification: source dependencies and tests"
GOTELEMETRY=off go mod verify
npm ci --prefix web --ignore-scripts
npm run build --prefix web
npm audit --prefix web --audit-level=high
python3 -m unittest discover -s scripts -p 'test_*.py' -q
# The Go tests intentionally create mode-0644/0755 negative fixtures to
# exercise refusal of shared files/directories. Keep the restrictive archive
# umask 0077, but run *only tests* with a normal fixture umask.
(
  umask 022
  GOTELEMETRY=off go test -count=1 ./...
  GOTELEMETRY=off go vet ./...
)
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Build unexpectedly changed tracked source files; refusing to package" >&2
  exit 1
fi

for arch in amd64 arm64; do
  stage="$temp/$arch"
  mkdir -m 0700 -- "$stage"
  echo "Building linux/$arch (static)"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOTELEMETRY=off \
    go build -trimpath \
      -ldflags="-s -w -X github.com/GoreeCloud/gateway/internal/gateway.Version=$version" \
      -o "$stage/gateway" .
  test -s "$stage/gateway"
  file "$stage/gateway" | grep -E "ELF 64-bit.*$([[ "$arch" = amd64 ]] && echo 'x86-64' || echo 'aarch64|ARM aarch64')"
  go version -m "$stage/gateway" | grep -F "GOARCH=$arch"
  go version -m "$stage/gateway" | grep -F 'github.com/caddyserver/caddy/v2'
  go version -m "$stage/gateway" | grep -F 'github.com/caddy-dns/porkbun'
  if [[ "$arch" = amd64 ]]; then
    "$stage/gateway" list-modules | grep -F http.handlers.reverse_proxy
    "$stage/gateway" list-modules | grep -F dns.providers.porkbun
  fi
  cp LICENSE THIRD_PARTY_NOTICES.md README.md "$stage/"
  cp -a docs examples scripts third_party_licenses "$stage/"
  {
    printf 'Release: %s\n' "$version"
    printf 'Protected-source candidate commit: %s\n' "$source_sha"
    printf 'Platform: linux/%s\n' "$arch"
    printf 'Go: %s\n' "$(go version)"
    printf 'Caddy: v2.11.9\nPorkbun DNS provider: v0.3.1\n'
    printf 'Development-only: no production release or deployment acceptance\n'
  } > "$stage/BUILDINFO.txt"
  tar --exclude='__pycache__' --exclude='*.pyc' \
    -C "$stage" -czf "$out/goreecloud-gateway-v$version-linux-$arch.tar.gz" \
    gateway LICENSE THIRD_PARTY_NOTICES.md README.md BUILDINFO.txt \
    docs examples scripts third_party_licenses
done

(
  cd "$out"
  sha256sum "goreecloud-gateway-v$version-linux-"*.tar.gz > SHA256SUMS.txt
  sha256sum -c SHA256SUMS.txt
)
python3 - "$out" "$version" "$source_sha" <<'PY'
import pathlib
import sys
import tarfile

out, version, source_sha = pathlib.Path(sys.argv[1]), sys.argv[2], sys.argv[3]
for arch in ("amd64", "arm64"):
    archive = out / f"goreecloud-gateway-v{version}-linux-{arch}.tar.gz"
    with tarfile.open(archive, "r:gz") as tf:
        members = tf.getmembers()
        assert all(
            not m.name.startswith("/")
            and ".." not in pathlib.PurePosixPath(m.name).parts
            and "__pycache__" not in m.name
            and not m.name.endswith(".pyc")
            and (m.isfile() or m.isdir())
            for m in members
        )
        names = {m.name for m in members}
        for name in (
            "gateway", "LICENSE", "README.md", "THIRD_PARTY_NOTICES.md",
            "BUILDINFO.txt", "third_party_licenses/opentelemetry-go-LICENSE",
        ):
            assert name in names, name
        info = tf.extractfile("BUILDINFO.txt").read().decode()
        assert source_sha in info and f"linux/{arch}" in info
    print(arch, "package checked:", archive.stat().st_size, "bytes")
PY
test -z "$(git status --porcelain)"
echo "Both Linux archives, source metadata, licenses and SHA256SUMS.txt verified."
echo "No GitHub release was published; verify protected CI and signatures separately."
