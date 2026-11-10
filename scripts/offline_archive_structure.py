#!/usr/bin/env python3
"""Read-only offline tar inventory gate; *not* backup recovery or extraction.

Never reads a production mount, executes tar, extracts files, contacts the
network, prints archive members or reports secret contents. A digest-matched
archive must still pass a disposable isolated *restore* before cutover.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import stat
import tarfile
import unicodedata

SCHEMA = "goreecloud.gateway.offline-tar-structure.v1"
MAX_INPUT_BYTES = 8 * 1024**3
MAX_LOGICAL_BYTES = 8 * 1024**3
MAX_ENTRY_BYTES = 4 * 1024**3
MAX_ENTRIES = 20000
MAX_PATH_BYTES = 512
PERMITTED_PAX = frozenset({"path", "size", "mtime"})


class ArchiveGateError(ValueError):
    """Fixed, nonsecret classification only."""


def safe_member_name(name: str) -> str:
    if not isinstance(name, str):
        raise ArchiveGateError("invalid_member")
    if name.endswith("//"):
        raise ArchiveGateError("unsafe_member_path")
    name = name[:-1] if name.endswith("/") else name
    if (not name or len(name.encode("utf-8", errors="replace")) > MAX_PATH_BYTES
            or name.startswith("/") or "\\" in name
            or "\x00" in name or unicodedata.normalize("NFC", name) != name
            or any(ord(c) < 32 or ord(c) == 127 for c in name)):
        raise ArchiveGateError("unsafe_member_path")
    pieces = name.split("/")
    if any(piece in ("", ".", "..") for piece in pieces):
        raise ArchiveGateError("unsafe_member_path")
    if ":" in pieces[0]:
        raise ArchiveGateError("unsafe_member_path")
    return name


def inspect_archive(path: Path) -> dict:
    if not hasattr(os, "O_NOFOLLOW") or not hasattr(os, "O_DIRECTORY"):
        raise ArchiveGateError("safe_file_open_unavailable")
    # Anchor the exact file open to an owner-private parent directory. A
    # symlinked/replaced parent must not redirect the inspector into a live
    # ingress tree or a different backup while a path is being checked.
    parent_fd = None
    try:
        parent_fd = os.open(
            path.parent, os.O_RDONLY | os.O_NOFOLLOW | os.O_DIRECTORY
            | getattr(os, "O_CLOEXEC", 0)
        )
        parent = os.fstat(parent_fd)
        if (not stat.S_ISDIR(parent.st_mode) or parent.st_uid != os.geteuid()
                or parent.st_mode & 0o077):
            raise ArchiveGateError("unsafe_parent_directory")
        fd = os.open(
            path.name, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW
            | getattr(os, "O_CLOEXEC", 0), dir_fd=parent_fd
        )
    except ArchiveGateError:
        raise
    except (OSError, TypeError, ValueError, AttributeError):
        raise ArchiveGateError("unavailable_or_unsafe") from None
    finally:
        if parent_fd is not None:
            os.close(parent_fd)
    try:
        with os.fdopen(fd, "rb") as archive:
            first = os.fstat(archive.fileno())
            if (not stat.S_ISREG(first.st_mode) or first.st_uid != os.geteuid()
                    or first.st_nlink != 1 or first.st_mode & 0o077
                    or first.st_size <= 0 or first.st_size > MAX_INPUT_BYTES):
                raise ArchiveGateError("unsafe_archive_properties")
            regular, directories, logical_bytes = 0, 0, 0
            seen, regular_paths, parent_paths = set(), set(), set()
            with tarfile.open(fileobj=archive, mode="r|*") as container:
                for member in container:
                    if regular + directories >= MAX_ENTRIES:
                        raise ArchiveGateError("archive_limits_exceeded")
                    name = safe_member_name(member.name)
                    canonical = name.casefold()
                    if canonical in seen:
                        raise ArchiveGateError("duplicate_member_path")
                    # File entries must never be used as parents. Check both
                    # archive orderings without O(member_count**2) scanning.
                    parts = canonical.split("/")
                    parents = {"/".join(parts[:i]) for i in range(1, len(parts))}
                    if any(parent in regular_paths for parent in parents):
                        raise ArchiveGateError("unsafe_member_hierarchy")
                    if member.isfile() and canonical in parent_paths:
                        raise ArchiveGateError("unsafe_member_hierarchy")
                    seen.add(canonical)
                    if (not member.pax_headers.keys() <= PERMITTED_PAX
                            or member.sparse is not None):
                        raise ArchiveGateError("unsupported_archive_metadata")
                    if member.isfile():
                        if not 0 <= member.size <= MAX_ENTRY_BYTES:
                            raise ArchiveGateError("archive_limits_exceeded")
                        logical_bytes += member.size
                        if logical_bytes > MAX_LOGICAL_BYTES:
                            raise ArchiveGateError("archive_limits_exceeded")
                        regular += 1
                    elif member.isdir():
                        if member.size != 0:
                            raise ArchiveGateError("unsupported_archive_metadata")
                        directories += 1
                    else:
                        # No hard/symbolic links, devices, FIFOs or exotic types.
                        raise ArchiveGateError("unsupported_archive_member")
                    if member.isfile():
                        regular_paths.add(canonical)
                    parent_paths.update(parents)
            last = os.fstat(archive.fileno())
            if (first.st_ino != last.st_ino or first.st_dev != last.st_dev
                    or first.st_size != last.st_size
                    or first.st_mtime_ns != last.st_mtime_ns
                    or first.st_ctime_ns != last.st_ctime_ns):
                raise ArchiveGateError("archive_changed_during_scan")
            if regular == 0:
                raise ArchiveGateError("empty_archive")
    except (tarfile.TarError, OSError, UnicodeError, OverflowError, RecursionError):
        raise ArchiveGateError("unreadable_archive") from None
    return {
        "schema": SCHEMA,
        "inspection": "offline_archive_headers_only",
        "structural_gate_passed": True,
        "regular_member_count": regular,
        "directory_member_count": directories,
        "uncompressed_logical_bytes": logical_bytes,
        "archive_sha256_verified": False,
        "archive_content_semantics_verified": False,
        "isolated_restore_verified": False,
        "acme_renewal_verified": False,
        "rollback_verified": False,
        "production_changed": False,
        "production_cutover_authorized": False,
    }


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(
        description="Inspect an already-created private offline tar archive without extraction."
    )
    parser.add_argument("offline_archive", type=Path)
    args = parser.parse_args(argv)
    try:
        result = inspect_archive(args.offline_archive)
    except ArchiveGateError:
        print(json.dumps({
            "schema": SCHEMA, "structural_gate_passed": False,
            "production_changed": False, "production_cutover_authorized": False,
            "error": "offline_archive_rejected",
        }, sort_keys=True))
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
