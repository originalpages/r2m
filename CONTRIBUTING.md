# Contributing to r2m

Thanks for helping improve `r2m`! Bug reports, ideas and pull requests
are all welcome.

By participating you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
For security issues, **do not** open a public issue; see [SECURITY.md](SECURITY.md).

## Reporting bugs

Search [existing issues](https://github.com/originalpages/r2m/issues)
first, then open a new one with:

- The command you ran and what you expected
- What happened (full error output)
- `r2m --version`, Node.js version and OS
- The languages in the analyzed repo, and ideally the smallest example that
  reproduces the wrong diagram

## Suggesting features

Open an issue describing the problem you want to solve before writing a large
change, so we can agree on the approach. `r2m` aims to stay small:
static analysis only, no LLM, one runtime dependency.

## Development setup

Requires Go 1.22+ (and Node.js 20+ only if you touch `web/`).

```bash
git clone https://github.com/originalpages/r2m.git
cd r2m
make test        # go vet + go test -race
make build       # ./r2m
```

Web page (optional):

```bash
cd web && npm ci && npm run dev
```

## Making a change

1. Fork the repo and create a branch from `main` (e.g. `fix/go-import-groups`).
   Don't commit directly to `main`.
2. Keep the change focused; one logical change per pull request.
3. Add or update tests for new behaviour and bug fixes
   (`internal/core/*_test.go`, `internal/cli/cli_test.go`). New language parsers
   need tests in `internal/core/parse_test.go`.
4. Run `make test` (runs in CI on Linux, macOS and Windows).
5. Update `README.md` if you change commands or options, and add a line to
   `CHANGELOG.md` for user-visible changes.

### Code guidelines

- `internal/core` is the engine and stays free of terminal/browser concerns;
  `internal/cli` is the command line and file output.
- Standard library only: avoid adding dependencies.
- Parsers must never panic or hang on malformed input.
- If you change engine behaviour, mirror it in `web/src/core` (TypeScript port
  used by the web page) and its tests in `web/test`.
- Prefer clear, short error messages over stack traces for expected failures.

### Commit messages

Short imperative subject line (about 50 characters), with a body explaining
*why* when it isn't obvious.

## Pull requests

- Open the PR against `main` and describe what changed and why; link any
  related issue (`Fixes #123`).
- CI must pass.
- Maintainers may ask for changes; please keep the discussion on the PR.

## License

By contributing you agree that your contributions are licensed under the
[MIT License](LICENSE).
