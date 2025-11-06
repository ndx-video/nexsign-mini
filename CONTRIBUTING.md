# nexSign mini contribution guide

The project is open to community changes while remaining sustainable for commercial deployments. This guide explains how licensing works, why we require a Contributor License Agreement (CLA), and how to get a pull request accepted.

## Licensing model

- **Community edition (GPLv3)** – default license for all source code in this repository. You are free to run, study, and modify it, but any derivative work that you distribute must also be GPLv3 compatible.
- **Commercial license** – a paid option offered by NDX Pty Ltd for organisations that cannot comply with GPLv3 obligations (for example, closed firmware images). Revenue from commercial licenses funds maintenance and continued development.

## Why we need a CLA

The CLA allows NDX Pty Ltd to relicense contributions under both GPLv3 and the commercial agreement without fragmenting the codebase. When you sign it you:

- keep the copyright for your work
- grant the project a broad, non-exclusive right to use, modify, and redistribute your contribution
- confirm that you have the authority to contribute the code

You will be guided through the CLA signing experience automatically on your first pull request.

## Contribution checklist

1. Discuss significant ideas in an issue first so we can align on scope and timing.
2. Fork the repository and create a feature branch.
3. Write clear, self-contained commits accompanied by tests and documentation updates when the behaviour changes.
4. Run `go test ./...` and `gofmt -w` on any touched Go files.
5. Submit a pull request and respond to review feedback promptly.

For changes that alter core behaviour, expect maintainers to ask for automated tests and updates to the markdown documentation.
