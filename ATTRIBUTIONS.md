# Attributions

This project incorporates data and methodology from the following third-party sources.

---

## BuEM

**Component:** ISO 52016-1 thermal building model, called by this connector
**Author:** Somadutta Sahoo, Utrecht University
**License:** MIT
**Link:** [UU-BUEM/buem](https://github.com/UU-BUEM/buem)

buem-gateway builds a fork ([enerplanet/buem](https://github.com/enerplanet/buem)) that only fixes
container packaging and the API surface — the model itself is unchanged. Built fresh from source at
image-build time (`environment/buem.dockerfile`), not vendored into this repository.

---

## TABULA Building Typology

**Component:** Building typology data, used by the TABULA-fallback path (via
[ignis](https://github.com/THD-Spatial-AI/ignis)) when a request omits `envelope`
**Source:** TABULA (Typology Approach for Building Stock Energy Assessment)
**Author:** Institut Wohnen und Umwelt (IWU), Darmstadt, Germany
**Project:** Intelligent Energy Europe Programme, IEE/09/739/SI2.558245
**License:** Creative Commons Attribution 4.0 International (CC BY 4.0)
**Link:** [building-typology.eu](https://webtool.building-typology.eu/) /
[episcope.eu](https://episcope.eu/iee-project/tabula/)

---

## Go Dependencies

Third-party Go modules used by this project are listed in `go.mod`. All are used under their
respective open-source licenses (MIT, BSD, Apache 2.0). No modifications have been made to any
dependency source code.
