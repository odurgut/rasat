# Security policy

Report a vulnerability **privately**. Do not open a public issue, pull request, or discussion for a security problem.

Use GitHub's private vulnerability reporting on this repository: **Security → Report a vulnerability**. If that button is missing, contact the maintainer via [github.com/odurgut](https://github.com/odurgut).

This project has one maintainer. There is no dedicated security team and no promised response SLA. You will get a reply when the report has been read.

## What to include

- Rasat version (`GET /version` or the image tag, e.g. `odurgut/rasat:0.1.2`)
- How you run it (image, binary, Compose)
- Steps to reproduce
- Impact: who can trigger it, and what they get

Do not include ClickHouse passwords, Hub tokens, or other secrets.

## Supported versions

Only the latest tagged release on Docker Hub (`odurgut/rasat`) is patched. Older tags are not backported; the fix lands on `main` and ships in the next patch. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Out of scope

This version has **no login**. Anyone who can reach the HTTP port can query and ingest. That is documented, not a vulnerability. Bind Rasat (and ClickHouse) to a network you trust.

Also not a vulnerability in this project:

- An exposed ClickHouse native or HTTP port in your deployment
- Missing auth, tenants, TLS termination, Kubernetes, or alerting — those are not in this version ([current limits](https://rasat.dev/limits))
