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


def verify_archive(path: Path, expected_sha256: str, *, identity_out=None) -> str:
    """Return a fixed classification with no error strings or file paths."""
    if not isinstance(expected_sha256, str) or not SHA256.fullmatch(expected_sha256):
        return "invalid_expected_digest"
    if not hasattr(os, "O_NOFOLLOW") or not hasattr(os, "O_DIRECTORY"):
        return "safe_file_open_unavailable"
    parent_fd = None
    try:
        parent_fd = os.open(
            path.parent, os.O_RDONLY | os.O_NOFOLLOW | os.O_DIRECTORY
            | getattr(os, "O_CLOEXEC", 0)
        )
        parent_info = os.fstat(parent_fd)
        if (not stat.S_ISDIR(parent_info.st_mode)
                or parent_info.st_uid != os.geteuid()
                or parent_info.st_mode & 0o077):
            return "unsafe_parent_directory"
        # A checked parent descriptor pins the directory used to open the
        # archive. Do not separately resolve a potentially swapped path.
        descriptor = os.open(
            path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
            | getattr(os, "O_CLOEXEC", 0), dir_fd=parent_fd
        )
    except (OSError, TypeError, ValueError, AttributeError):
        return "unavailable_or_unsafe"
    finally:
        if parent_fd is not None:
            os.close(parent_fd)
    try:
        # Keep ownership of the raw FD until fdopen has succeeded. Both
        # fstat() and fdopen() can raise; neither must leak the live archive
        # descriptor. Once handed to the file object, its context manager
        # owns closure on every early return and exceptional path.
        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
            return "unsafe_file_properties"
        with os.fdopen(descriptor, "rb") as archive:
            descriptor = None
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
            if not hmac.compare_digest(digest.hexdigest(), expected_sha256):
                return "digest_mismatch"
            # Capture the identity of the *opened and hashed* inode, rather
            # than trusting path text which can alias the same archive.
            # No file names, paths, hashes or metadata enter the report.
            if identity_out is not None:
                identity_out.append((before.st_dev, before.st_ino))
            return "matched"
    except OSError:
        return "read_failed"
    finally:
        if descriptor is not None:
            try:
                os.close(descriptor)
            except OSError:
                pass


def build_report(
    config_status: str, acme_status: str, *, sources_distinct: bool = False
) -> dict:
    # Two passing hashes do not constitute two separate archives if callers
    # accidentally reuse the same underlying inode for both roles.
    distinct = sources_distinct is True
    matched = config_status == acme_status == "matched" and distinct
    return {
        "schema": SCHEMA,
        "inspection": "read_only_offline_files",
        "config_archive_integrity": config_status,
        "acme_archive_integrity": acme_status,
        "both_hashes_match": matched,
        "distinct_archive_sources_verified": distinct,
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
    config_identity, acme_identity = [], []
    config_status = verify_archive(
        args.config_archive, args.config_sha256, identity_out=config_identity
    )
    acme_status = verify_archive(
        args.acme_archive, args.acme_sha256, identity_out=acme_identity
    )
    distinct = bool(
        config_identity and acme_identity
        and config_identity[0] != acme_identity[0]
    )
    report = build_report(
        config_status, acme_status, sources_distinct=distinct
    )
    print(json.dumps(report, sort_keys=True))
    return 0 if report["both_hashes_match"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
