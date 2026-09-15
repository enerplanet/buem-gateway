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

- **The service authenticates nothing, by design.** `buem-gateway` checks no credential, and
  neither does anything shipped in front of it. Access is controlled by the network the service is
  published on: it is intended to run on a network whose access you control, with callers already
  authenticated by the EnerPlanET platform before they reach it. An earlier prototype
  checked a static `X-Api-Key` header at the reverse proxy; that has been removed rather than
  relied on, because it was never a substitute for real authentication.
- **`buem-gateway` publishes a host port in the HTTP environment, deliberately.**
  [`environment/http`](environment/http) runs the connector with no proxy in front of it and maps
  its port, bound to loopback by default so it is reachable only from that machine. Widening that
  bind, or publishing the service on a network whose access is not controlled, exposes an unauthenticated
  API. That is a deployment decision and its consequences are the deployer's, not a vulnerability
  in buem-gateway. [`environment/https`](environment/https) instead publishes only Caddy, which
  terminates TLS and performs no authentication either.
- **`buem-model` publishes no port in any environment.** The BuEM Flask model is reachable only
  from `buem-gateway`, by service name on the compose network. It trusts its caller completely: it
  performs a real physics simulation on whatever input it is handed and is not built to face a
  caller directly. Giving it a published port is a misconfiguration, not a vulnerability in
  buem-gateway.

If you find a genuine issue within that design, please report it as above. For example: a way to
reach `buem-model` from outside the compose network, an injection vulnerability, a way to read
another caller's results off the shared volume, or a way to make `buem-gateway` act on a URL or a
credential supplied in a request rather than in its own configuration.
