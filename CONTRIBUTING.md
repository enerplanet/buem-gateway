# Contributing

Thank you for taking the time to contribute to **buem-gateway**.

This project welcomes contributions such as bug reports, feature requests, documentation
improvements, and code changes.

Please read this guide before opening an issue or submitting a pull request.

## Code of Conduct

By participating in this project, you agree to follow the rules and expectations described in the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Ways to Contribute

- Reporting bugs or unexpected connector behaviour
- Requesting features or improvements
- Improving documentation
- Fixing issues
- Reviewing pull requests

## Before You Start

- Read the `README.md` and [`docs/getting-started.md`](docs/getting-started.md) to understand the
  project and how to run it locally
- Check existing issues and pull requests to avoid duplicates
- Make sure your idea is relevant to the project scope — buem-gateway is the connector between
  EnerPlanET's topology-JSON format and the upstream BuEM thermal model; changes to BuEM itself
  belong in [`enerplanet/buem`](https://github.com/enerplanet/buem)

## Reporting Bugs and Requesting Changes

Use the [GitHub issue tracker](https://github.com/enerplanet/buem-gateway/issues) for bug reports
and feature requests.

When reporting an issue, please include:

- What you expected to happen
- What actually happened
- Steps to reproduce the issue
- Relevant logs or error messages
- Go version and OS

## Development Workflow

### 1. Fork and clone

```bash
git clone https://github.com/enerplanet/buem-gateway.git
cd buem-gateway
```

### 2. Create a branch

```bash
git checkout -b type/short-description
```

Examples: `fix/tabula-fallback-nil-check`, `feat/add-cooling-endpoint`, `docs/readme-update`

### 3. Make your changes

Keep changes focused and small where possible.

### 4. Test your changes

```bash
go build ./...
go vet ./...
go test ./...
```

For changes touching the Docker environment, bring up the stack and confirm it starts healthy —
see [`docs/getting-started.md`](docs/getting-started.md):

```bash
cd environment
docker compose up -d --build
```

### 5. Commit and push

```bash
git add .
git commit -m "Short summary of the change"
git push -u origin <your-branch-name>
```

### 6. Open a pull request

Create a pull request against `main`. Include:

- What changed and why
- Related issue(s) (e.g. `Closes #123`)

## Pull Request Checklist

- [ ] Change is relevant and scoped appropriately
- [ ] `go build ./...` and `go vet ./...` pass
- [ ] `go test ./...` passes, with tests for any new or changed behaviour
- [ ] No sensitive information (credentials, API keys) included
- [ ] Documentation updated if usage or behaviour changed

## Commit Message Guidance

Keep messages clear and specific.

Good examples:
- `fix: TABULA fallback panics when resolver is nil`
- `feat: add cooling support to /api/v1/buem/building`
- `docs: update README quick start`

## Licensing of Contributions

By contributing to this project, you confirm that your contribution is your own work and you
agree it will be licensed under the [MIT License](LICENSE).
