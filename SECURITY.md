# Security Policy

## Supported Versions

`v4.0.0` is the first git-tagged release of the Go connector itself (releases before it covered
only the JSON schema contract, which now versions separately — see
[`docs/versioning.md`](docs/versioning.md)). Only the latest tagged release and the latest commit
on `main` are supported — please update before reporting an issue.

## Reporting a Vulnerability

Please report security vulnerabilities privately, not through a public GitHub issue.

Use [GitHub's private vulnerability reporting](https://github.com/enerplanet/buem-gateway/security/advisories/new)
(Security tab → **Report a vulnerability**). This opens a private advisory thread with the
maintainers — the report stays hidden from the public repository until a fix is out.

You should hear back within a week. If the report is valid, we'll work with you on a fix and
coordinate disclosure timing before anything is made public.

## Scope

A few things about buem-gateway's design that are **intentional, documented limitations**, not
vulnerabilities to report:

- **The `X-Api-Key` header is a prototype-stage credential**, checked by the Caddy reverse proxy
  in front of the `buem-gateway` container (see [`environment/caddy/Caddyfile`](environment/caddy/Caddyfile)).
  It is not a substitute for real authentication and must not be relied on in a production
  deployment — see the orchestration-layer decision note referenced from the docs for the intended
  long-term replacement.
- **Neither backing container authenticates on its own.** `buem-gateway` (the Go connector) and
  `buem-model` (the BuEM Flask model it calls) publish no port and are only reachable through the
  reverse proxy on the shared `building-simulation` Docker network — see
  [`docs/getting-started.md`](docs/getting-started.md). Deploying either container with a
  published port, bypassing the proxy, is a misconfiguration, not a vulnerability in buem-gateway
  itself.
- **The BuEM model (`buem-model`) trusts its caller.** It performs a real physics
  simulation on the input it's given and is not designed to be exposed directly — it is only ever
  called by `buem-gateway` on the internal Docker network.

If you find a genuine issue within that design — for example, a way to bypass the reverse proxy's
checks, an injection vulnerability, a way to access another caller's data, or a way to reach
`buem-model` directly from outside the Docker network — please report it as above.
