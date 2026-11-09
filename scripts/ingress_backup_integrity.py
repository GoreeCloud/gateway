#!/usr/bin/env python3
"""Read-only integrity check for two previously created *offline* Caddy backups.

Only hashes existing, owner-readable regular files. Never opens a container,
makes a backup, restores certificates, touches ingress, or reports filenames,
secret content, mount paths, environment values, or exception messages.
A hash match is NOT evidence that the backup can be restored.
"""
from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import stat

SCHEMA = "goreecloud.gateway.offline-backup-integrity.v1"
SHA256 = re.compile(r"[0-9a-f]{64}\Z")
MAX_BYTES = 8 * 1024**3
CHUNK_BYTES = 1024 * 1024


def verify_archive(path: Path, expected_sha256: str) -> str:
    """Return a fixed classification with no error strings or file paths."""
    if not isinstance(expected_sha256, str) or not SHA256.fullmatch(expected_sha256):
        return "invalid_expected_digest"
    if not hasattr(os, "O_NOFOLLOW"):
        return "safe_file_open_unavailable"
    try:
        descriptor = os.open(
            path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
            | getattr(os, "O_CLOEXEC", 0)
        )
    except OSError:
        return "unavailable_or_unsafe"
    try:
        # A directory can make fdopen fail before entering the context manager.
        # Reject it explicitly and close the descriptor on this early path.
        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
            os.close(descriptor)
            return "unsafe_file_properties"
        with os.fdopen(descriptor, "rb") as archive:
            before = os.fstat(archive.fileno())
            if (not stat.S_ISREG(before.st_mode) or before.st_uid != os.geteuid()
                    or before.st_nlink != 1 or before.st_mode & 0o077
                    or before.st_size <= 0
                    or before.st_size > MAX_BYTES):
                return "unsafe_file_properties"
            digest = hashlib.sha256()
            total = 0
            for chunk in iter(lambda: archive.read(CHUNK_BYTES), b""):
                total += len(chunk)
                if total > MAX_BYTES:
                    return "oversized"
                digest.update(chunk)
            after = os.fstat(archive.fileno())
            if (total != before.st_size or before.st_size != after.st_size
                    or before.st_mtime_ns != after.st_mtime_ns
                    or before.st_ctime_ns != after.st_ctime_ns
                    or before.st_ino != after.st_ino
                    or before.st_dev != after.st_dev):
                return "file_changed_during_check"
            return "matched" if hmac.compare_digest(
                digest.hexdigest(), expected_sha256
            ) else "digest_mismatch"
    except OSError:
        return "read_failed"


def build_report(config_status: str, acme_status: str) -> dict:
    matched = config_status == acme_status == "matched"
    return {
        "schema": SCHEMA,
        "inspection": "read_only_offline_files",
        "config_archive_integrity": config_status,
        "acme_archive_integrity": acme_status,
        "both_hashes_match": matched,
        "archive_restoration": "not_verified",
        "porkbun_dns01_renewal": "not_verified",
        "route_and_private_network_parity": "not_verified",
        "rollback_test": "not_verified",
        "production_changed": False,
        "cutover_readiness": "not_established",
    }


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=(
        "Verify existing protected offline ingress backup file digests "
        "without disclosing paths or content; never authorizes a cutover."
    ))
    parser.add_argument("--config-archive", required=True, type=Path)
    parser.add_argument("--config-sha256", required=True)
    parser.add_argument("--acme-archive", required=True, type=Path)
    parser.add_argument("--acme-sha256", required=True)
    args = parser.parse_args(argv)
    config_status = verify_archive(args.config_archive, args.config_sha256)
    acme_status = verify_archive(args.acme_archive, args.acme_sha256)
    report = build_report(config_status, acme_status)
    print(json.dumps(report, sort_keys=True))
    return 0 if report["both_hashes_match"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
