#!/usr/bin/env python3
"""Offline action-reference policy for all three GoreeCloud Gateway workflows.

This checks exact reviewed source pins and ephemeral checkout auth. It does
not execute, audit or attest third-party action code or external dependencies.
"""
from collections import Counter
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1] / ".github" / "workflows"
PINS = {
    "actions/checkout": "11d5960a326750d5838078e36cf38b85af677262",
    "actions/setup-go": "924ae3a1cded613372ab5595356fb5720e22ba16",
    "actions/setup-java": "cf277c60eb25467037889841efdb72551f06f6c3",
    "gradle/actions/setup-gradle": "ed408507eac070d1f99cc633dbcf757c94c7933a",
    "android-actions/setup-android": "be39fa834029ff78f1a44aa3bb0819b8fc2bd8fd",
    "reactivecircus/android-emulator-runner": "a421e43855164a8197daf9d8d40fe71c6996bb0d",
    "actions/upload-artifact": "ea165f8d65b6e75b540449e92b4886f43607fa02",
}
EXPECTED = {
    "ci.yml": {"actions/checkout": 1, "actions/setup-go": 1,
               "actions/upload-artifact": 1},
    "security.yml": {"actions/checkout": 1, "actions/setup-go": 1,
                     "actions/upload-artifact": 1},
    "android.yml": {"actions/checkout": 2, "actions/setup-java": 2,
                    "gradle/actions/setup-gradle": 2,
                    "android-actions/setup-android": 2,
                    "actions/upload-artifact": 2,
                    "reactivecircus/android-emulator-runner": 1},
}
USES = re.compile(r"(?m)^\s*(?:-\s*)?uses:\s*([^\s#]+)")
CHECKOUT = re.compile(
    r"(?m)^      - uses: actions/checkout@[0-9a-f]{40}(?:[^\n]*)\n"
    r"        with:\n"
    r"          persist-credentials: false\s*$"
)


def verify_workflow(name: str, source: str) -> None:
    expected = Counter(EXPECTED[name])
    observed = Counter()
    actions = USES.findall(source)
    for reference in actions:
        action, separator, pin = reference.rpartition("@")
        if not separator or action not in PINS or pin != PINS[action]:
            raise ValueError(f"{name}: mutable or unreviewed workflow action")
        observed[action] += 1
    if observed != expected:
        raise ValueError(f"{name}: missing, duplicate or extra workflow action")
    if len(CHECKOUT.findall(source)) != expected["actions/checkout"]:
        raise ValueError(f"{name}: persistent or absent checkout credentials guard")


def main() -> None:
    for name in EXPECTED:
        verify_workflow(name, (ROOT / name).read_text(encoding="utf-8"))
    for name in ("ci.yml", "security.yml", "android.yml"):
        original = (ROOT / name).read_text(encoding="utf-8")
        for mutation in (
            original.replace(PINS["actions/checkout"], "v4", 1),
            original.replace("persist-credentials: false",
                             "persist-credentials: true", 1),
            original + "\n      - uses: some-unknown/action@v1\n",
            original.replace("      - uses: actions/checkout@", "      # uses: actions/checkout@", 1),
        ):
            try:
                verify_workflow(name, mutation)
            except ValueError:
                pass
            else:
                raise AssertionError(f"{name}: unreviewed CI action mutation passed")
    print("PASS: exact pins and nonpersistent checkout auth across 3 Gateway workflows")


if __name__ == "__main__":
    main()
