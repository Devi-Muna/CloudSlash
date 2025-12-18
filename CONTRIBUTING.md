# Contributing to CloudSlash

## Welcome, Cloud Detective. 🕵️‍♂️

Thank you for your interest in contributing to **CloudSlash**. We believe in "Zero Trust" infrastructure and "Zero Waste" cloud computing. If you share this obsession for precision and efficiency, you are in the right place.

## The "Open Core" Philosophy

CloudSlash is **Open Source (AGPLv3)**. We believe the core engine that detects waste should be transparent, auditable, and accessible to everyone.

However, we also sustainable development. Some advanced features (Automated Reporting, Terraform Generation) are reserved for **Pro Licensed Users**.

- **You MAY** contribute to the core engine, scanners, and heuristics.
- **You MAY** use the code for your own internal audits.
- **You MAY NOT** strip the license checks and resell this as your own SaaS product (that violates the AGPL).

## How to Contribute

### 1. Reporting Bugs

- Usage the GitHub Issues tab.
- Please provide your OS, CloudSlash version (`cloudslash --help`), and a redacted output of the error.

### 2. Suggesting Heuristics

- found a new way to waste money on AWS? Open an Issue titled `[Heuristic Idea]: <Name>`.
- Describe the logic: "If resource X has metric Y < threshold Z for T time..."

### 3. Pull Requests

- Fork the repo.
- Create a branch: `git checkout -b fix/your-fix` or `feat/your-feature`.
- **Run Tests**: Ensure the project compiles with `go build ./cmd/cloudslash`.
- **Format Code**: Run `go fmt ./...`.
- Push and open a PR.

## Development Setup

1.  **Install Go 1.25+**
2.  **Clone the Repo**:
    ```bash
    git clone https://github.com/DrSkyle/CloudSlash.git
    cd CloudSlash
    ```
3.  **Run Locally**:
    ```bash
    go run ./cmd/cloudslash
    ```
4.  **Mock Mode**:
    Use the `--mock` flag to test the UI without AWS credentials (note: flag is hidden in help).
    ```bash
    go run ./cmd/cloudslash --mock
    ```

## Style Guide

- **Zero Errors**: We do not ignore errors. Wrap them, log them, or handle them.
- **Zero Hallucinations**: Do not assume an API call works. Verify identity. Verify regions.
- **Future-Glass UI**: If touching the TUI, strictly adhere to the neon/glassmorphism aesthetic defined in `internal/ui`.

---

_By contributing to this repository, you agree that your contributions will be licensed under the AGPLv3._
