#!/usr/bin/env python3
"""Validates schemas/v5/{request,response}_schema.json against their example
files, and confirms the current-version string agrees across
docs/versioning.md, CHANGELOG.md, and docs/openapi/openapi.yaml.

CURRENT_VERSION_DIR is the frozen directory for the current contract; bump it
when a new version is promoted out of schemas/v6-draft/.

Run from anywhere: python scripts/validate_schemas.py
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

from jsonschema import Draft202012Validator

REPO_ROOT = Path(__file__).resolve().parent.parent
CURRENT_VERSION_DIR = "v5"


def validate_schema_examples(root: Path) -> list[str]:
    """Validate schemas/<CURRENT_VERSION_DIR>/{request,response}_schema.json
    against their example files under root. Returns failure messages, empty
    if both validate cleanly.
    """
    failures: list[str] = []
    current = root / "schemas" / CURRENT_VERSION_DIR
    for name in ("request", "response"):
        schema = json.loads((current / f"{name}_schema.json").read_text())
        example = json.loads((current / f"example_{name}.json").read_text())
        errors = list(Draft202012Validator(schema).iter_errors(example))
        if errors:
            for error in errors:
                failures.append(f"{name}: {error.message}")
        else:
            print(f"{name}: OK")
    return failures


def _version_from_versioning_doc(root: Path) -> str | None:
    """First version cell in versioning.md's "Released versions" table."""
    text = (root / "docs" / "versioning.md").read_text()
    match = re.search(r"^\|\s*(v\d+\.\d+\.\d+)\s*\|", text, re.MULTILINE)
    return match.group(1) if match else None


def _version_from_changelog(root: Path) -> str | None:
    """First "## vX.Y.Z" heading in CHANGELOG.md."""
    text = (root / "CHANGELOG.md").read_text()
    match = re.search(r"^## (v\d+\.\d+\.\d+)", text, re.MULTILINE)
    return match.group(1) if match else None


def _version_from_openapi(root: Path) -> str | None:
    """info.version in docs/openapi/openapi.yaml, normalized to a "vX.Y.Z" string."""
    text = (root / "docs" / "openapi" / "openapi.yaml").read_text()
    match = re.search(r"^\s*version:\s*(\d+\.\d+\.\d+)\s*$", text, re.MULTILINE)
    return f"v{match.group(1)}" if match else None


def validate_version_sync(root: Path) -> list[str]:
    """Confirm the current-version string agrees across docs/versioning.md,
    CHANGELOG.md, and docs/openapi/openapi.yaml -- the exact drift the release
    procedure otherwise asks a human to prevent by hand.
    """
    sources = {
        "docs/versioning.md": _version_from_versioning_doc(root),
        "CHANGELOG.md": _version_from_changelog(root),
        "docs/openapi/openapi.yaml": _version_from_openapi(root),
    }
    failures: list[str] = []
    for path, version in sources.items():
        if version is None:
            failures.append(f"{path}: could not find a version string")
    found = {version for version in sources.values() if version is not None}
    if len(found) > 1:
        detail = ", ".join(f"{path}={version}" for path, version in sources.items())
        failures.append(f"version string mismatch across files: {detail}")
    return failures


def validate_all(root: Path) -> list[str]:
    return validate_schema_examples(root) + validate_version_sync(root)


def main() -> int:
    failures = validate_all(REPO_ROOT)
    if failures:
        for failure in failures:
            print(failure, file=sys.stderr)
        return 1
    print("version sync: OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
