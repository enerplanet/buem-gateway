#!/usr/bin/env python3
"""Tests for validate_schemas.py -- confirms it actually fails (non-empty
failure list) on a broken example or a version mismatch, not just that it
runs without raising.

Run directly: python scripts/validate_schemas_test.py
"""

from __future__ import annotations

import json
import shutil
import tempfile
import unittest
from pathlib import Path

from validate_schemas import validate_all, validate_schema_examples, validate_version_sync

REPO_ROOT = Path(__file__).resolve().parent.parent


def _copy_real_repo_fixtures(dest: Path) -> None:
    """Copy the real schemas/ current-version files and version-source
    files into dest, so each test starts from a known-good state and
    corrupts only what it needs to. Version snapshot folders are not
    needed by the script and are skipped to keep the copy cheap.
    """
    shutil.copytree(
        REPO_ROOT / "schemas",
        dest / "schemas",
        ignore=shutil.ignore_patterns("v1", "v2", "v3", "v4"),
    )
    (dest / "docs" / "openapi").mkdir(parents=True)
    shutil.copy(REPO_ROOT / "docs" / "versioning.md", dest / "docs" / "versioning.md")
    shutil.copy(REPO_ROOT / "docs" / "openapi" / "openapi.yaml", dest / "docs" / "openapi" / "openapi.yaml")
    shutil.copy(REPO_ROOT / "CHANGELOG.md", dest / "CHANGELOG.md")


class TestValidateSchemas(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp())
        _copy_real_repo_fixtures(self.tmp)

    def tearDown(self) -> None:
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_real_repo_state_passes(self) -> None:
        self.assertEqual(validate_all(self.tmp), [])

    def test_invalid_example_fails(self) -> None:
        """A request example missing a required field must be reported as
        a failure, not silently accepted."""
        example_path = self.tmp / "schemas" / "example_request.json"
        example = json.loads(example_path.read_text())
        del example["type"]  # request_schema.json requires "type"
        example_path.write_text(json.dumps(example))

        failures = validate_schema_examples(self.tmp)
        self.assertTrue(
            failures, "expected a failure for a request example missing a required field"
        )

    def test_version_mismatch_fails(self) -> None:
        """openapi.yaml disagreeing with versioning.md/CHANGELOG.md must
        be reported as a failure -- the exact drift this check exists to
        catch."""
        openapi_path = self.tmp / "docs" / "openapi" / "openapi.yaml"
        text = openapi_path.read_text()
        self.assertIn("version: 5.0.0", text, "fixture assumption changed, update this test")
        openapi_path.write_text(text.replace("version: 5.0.0", "version: 999.0.0"))

        failures = validate_version_sync(self.tmp)
        self.assertTrue(
            failures, "expected a failure when openapi.yaml's version disagrees"
        )


if __name__ == "__main__":
    unittest.main()
