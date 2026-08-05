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

## Go Dependencies

Third-party Go modules used by this project are listed in `go.mod`. All are used under their
respective open-source licenses (MIT, BSD, Apache 2.0). No modifications have been made to any
dependency source code.
