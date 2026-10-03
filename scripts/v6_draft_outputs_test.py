#!/usr/bin/env python3
"""Tests for the v6-draft request schema's output selection: weather and
building.envelope are required only when heating or cooling is selected, and
the occupancy fields required by building_type.

Run directly: python scripts/v6_draft_outputs_test.py
"""

from __future__ import annotations

import copy
import json
import unittest
from pathlib import Path

import jsonschema

DRAFT = Path(__file__).resolve().parent.parent / "schemas" / "v6-draft"


def _load(name: str) -> dict:
    return json.loads((DRAFT / name).read_text())


class OutputSelectionTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.validator = jsonschema.Draft202012Validator(_load("request_schema.json"))
        cls.example = _load("example_request.json")

    def _valid(self, mutate) -> bool:
        doc = copy.deepcopy(self.example)
        mutate(doc)
        return not list(self.validator.iter_errors(doc))

    @staticmethod
    def _buem(doc: dict, i: int) -> dict:
        return doc["features"][i]["properties"]["buem"]

    def test_example_is_valid(self) -> None:
        self.assertTrue(self._valid(lambda d: None))

    def test_full_run_needs_weather_and_envelope(self) -> None:
        self.assertFalse(self._valid(lambda d: self._buem(d, 0).pop("weather")))
        self.assertFalse(self._valid(lambda d: self._buem(d, 0)["building"].pop("envelope")))

    def test_selecting_heating_needs_envelope(self) -> None:
        self.assertFalse(self._valid(lambda d: self._buem(d, 2)["outputs"].update(heating="summary")))

    def test_omitted_heating_counts_as_selected(self) -> None:
        self.assertFalse(self._valid(lambda d: self._buem(d, 2)["outputs"].pop("heating")))

    def test_mfh_and_ab_need_residential_units(self) -> None:
        self.assertFalse(self._valid(lambda d: self._buem(d, 2)["building"].pop("residential_units")))
        self.assertTrue(self._valid(lambda d: self._buem(d, 0)["building"].pop("residential_units")))

    def test_classification_fields_are_required(self) -> None:
        for field in ("building_type", "country", "A_ref"):
            with self.subTest(field=field):
                self.assertFalse(self._valid(lambda d: self._buem(d, 2)["building"].pop(field)))

    def test_service_buildings_take_no_equipment_and_one_unit(self) -> None:
        self.assertFalse(self._valid(lambda d: self._buem(d, 1)["building"].update(equipment={"kettle": True})))
        self.assertFalse(self._valid(lambda d: self._buem(d, 1)["building"].update(residential_units=2)))
        self.assertTrue(self._valid(lambda d: self._buem(d, 1)["building"].update(residential_units=1)))

    def test_output_levels_are_closed(self) -> None:
        self.assertFalse(self._valid(lambda d: self._buem(d, 2)["outputs"].update(electricity="hourly")))
        self.assertFalse(self._valid(lambda d: self._buem(d, 2)["outputs"].update(gas="summary")))


if __name__ == "__main__":
    unittest.main()
