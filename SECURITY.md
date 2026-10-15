# Security Policy

## Supported versions

`r2m` is a small, fast-moving tool. Security fixes are released for the
latest minor version only.

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅        |

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report privately through GitHub's
[private vulnerability reporting](https://github.com/originalpages/r2m/security/advisories/new)
(repository → **Security** → **Report a vulnerability**).

Please include:

- A description of the issue and its impact
- Steps to reproduce, or a proof of concept
- The `r2m` version (`r2m --version`) and OS
- Any suggested fix, if you have one

You can expect an acknowledgement within **7 days**. This is a volunteer-run
project, so fix timelines depend on severity, but we will keep you updated and
credit you in the release notes and advisory if you wish.

Please give us a reasonable chance to release a fix before disclosing publicly.

## Scope

`r2m` performs static analysis only: it reads files and import
statements, makes no LLM calls and executes none of the analyzed code. For
GitHub inputs the CLI runs `git clone --depth 1` into a temporary directory;
the web page uses the unauthenticated GitHub REST API from the browser.

In scope:

- Command or argument injection via repo references, file names or paths
  (the `git clone` step, the browser launcher, Action inputs)
- Path traversal or writing outside the intended `--out` / `--update` targets
- Mermaid or HTML injection through file names or labels that executes in the
  web page or in rendered Markdown
- Packaging or CI issues that could compromise releases

Out of scope:

- Vulnerabilities in Mermaid, Go, git or GitHub itself (report upstream)
- Attacks that require an attacker to already control your machine
