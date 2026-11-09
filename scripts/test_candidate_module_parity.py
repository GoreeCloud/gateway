"""Synthetic offline module checks; never execute production Caddy or read secrets."""
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from contextlib import redirect_stdout
from unittest.mock import patch

from candidate_module_parity import (
    CandidateError, EXPECTED_MODULES, inspect_candidate, main,
    parse_modules, pinned_version, synthetic_environment,
)


class CandidateModuleParityTests(unittest.TestCase):
    def setUp(self):
        self.work = tempfile.TemporaryDirectory()
        self.root = Path(self.work.name)
        self.binary = self.root / "candidate"
        self.binary.write_bytes(b"NOT-EXECUTED" * 200)
        self.binary.chmod(0o700)
        (self.root / "configs").mkdir()
        (self.root / "configs/Porkbunfile.example").write_text("fixture")
        (self.root / "configs/RuntimeParity.example").write_text("fixture")
        (self.root / "go.mod").write_text(
            "module synthetic\n"
            "require (\n"
            "github.com/caddyserver/caddy/v2 v2.11.8\n"
            "github.com/caddy-dns/porkbun v0.3.1\n"
            ")\n"
        )

    def tearDown(self):
        self.work.cleanup()

    def runner(self, binary, args, *, env):
        self.assertEqual(binary, self.binary)
        self.assertNotIn("REAL_PORKBUN_CREDENTIAL", json.dumps(env))
        if args == ["version"]:
            return "v2.11.8 h1:synthetic-test-only\n"
        if args == ["list-modules", "--versions"]:
            return "\n".join(
                f"{name} {('v0.3.1' if name == 'dns.providers.porkbun' else 'v2.11.8')}"
                for name in EXPECTED_MODULES
            )
        if args[:2] == ["adapt", "--config"]:
            return (json.dumps({"issuer": {"porkbun": "synthetic"}})
                    if args[2].endswith("Porkbunfile.example")
                    else json.dumps({"handler": "reverse_proxy"}))
        raise AssertionError("unexpected executable invocation")

    def test_all_synthetic_checks_pass_without_claiming_renewal_or_cutover(self):
        report = inspect_candidate(self.binary, self.root, runner=self.runner)
        self.assertTrue(report["candidate_parity_checks_passed"])
        for key in ("live_porkbun_credentials_verified", "acme_renewal_verified",
                    "private_site_parity_verified", "backup_restoration_verified",
                    "rollback_verified", "production_changed", "production_cutover_authorized"):
            self.assertFalse(report[key])

    def test_wrong_engine_version_is_refused(self):
        def wrong(binary, args, *, env):
            return "v2.11.4 h1:old\n" if args == ["version"] else self.runner(binary, args, env=env)
        report = inspect_candidate(self.binary, self.root, runner=wrong)
        self.assertFalse(report["candidate_parity_checks_passed"])
        self.assertFalse(report["candidate_engine_matches_source"])

    def test_missing_porkbun_and_wrong_plugin_version_are_refused(self):
        def missing(binary, args, *, env):
            result = self.runner(binary, args, env=env)
            return result.replace("dns.providers.porkbun v0.3.1", "") if args[0] == "list-modules" else result
        self.assertFalse(inspect_candidate(self.binary, self.root, runner=missing)["candidate_parity_checks_passed"])
        def outdated(binary, args, *, env):
            result = self.runner(binary, args, env=env)
            return result.replace("dns.providers.porkbun v0.3.1", "dns.providers.porkbun v0.2.0") if args[0] == "list-modules" else result
        self.assertFalse(inspect_candidate(self.binary, self.root, runner=outdated)["candidate_parity_checks_passed"])

    def test_synthetic_adaptation_failure_is_refused(self):
        def broken(binary, args, *, env):
            if args[0] == "adapt":
                return "{}"
            return self.runner(binary, args, env=env)
        self.assertFalse(inspect_candidate(self.binary, self.root, runner=broken)["candidate_parity_checks_passed"])

    def test_group_writable_and_symlink_executables_are_refused(self):
        self.binary.chmod(0o770)
        with self.assertRaises(CandidateError):
            inspect_candidate(self.binary, self.root, runner=self.runner)
        link = self.root / "linked-binary"
        link.symlink_to(self.binary)
        with self.assertRaises(CandidateError):
            inspect_candidate(link, self.root, runner=self.runner)

    def test_dependency_lockfile_must_be_unambiguous(self):
        with self.assertRaises(CandidateError):
            pinned_version("github.com/caddyserver/caddy/v2 v2.11.8\n"
                           "github.com/caddyserver/caddy/v2 v2.11.7\n",
                           "github.com/caddyserver/caddy/v2")
        self.assertEqual(parse_modules("dns.providers.porkbun v0.3.1\ninvalid things here"),
                         {"dns.providers.porkbun": "v0.3.1"})

    def test_environment_never_inherits_real_dns_provider_secrets(self):
        with patch.dict(os.environ, {"PORKBUN_API_KEY": "REAL_PORKBUN_CREDENTIAL",
                                     "PORKBUN_API_SECRET_KEY": "REAL_PORKBUN_CREDENTIAL"}):
            env = synthetic_environment()
        self.assertEqual(env["PORKBUN_API_KEY"], "test-only-noncredential")
        self.assertNotIn("REAL_PORKBUN_CREDENTIAL", json.dumps(env))

    def test_missing_executable_cli_failure_is_redacted(self):
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            code = main(["--binary", str(self.root / "nonexistent"),
                         "--source-root", str(self.root)])
        self.assertEqual(code, 2)
        record = json.loads(stdout.getvalue())
        self.assertEqual(record["error"], "offline_preflight_failed")
        self.assertFalse(record["production_cutover_authorized"])
        self.assertNotIn(str(self.root), stdout.getvalue())


if __name__ == "__main__":
    unittest.main()
