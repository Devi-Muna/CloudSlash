# Contributing to CloudSlash

## The Philosophy

CloudSlash is **100% Open Source** software licensed under the **GNU Affero General Public License v3.0 (AGPLv3)**.

We do not have a "Pro" version or "Open Core" restrictions. Every heuristic, every feature, and every line of code is available to everyone. We believe in transparency and community ownership.

## Workflow

### Reporting Issues

1.  **Search First**: check existing issues to avoid duplicates.
2.  **Version Info**: Include `cloudslash --version` or commit hash.
3.  **Logs**: Provide sanitized logs or a minimal reproduction case.

### Pull Requests

1.  **Fork** the repository.
2.  **Branch**: `git checkout -b feat/my-feature`
3.  **Verify**: Ensure it builds `go build ./cmd/cloudslash`
4.  **Format**: `go fmt ./...`
5.  **Submit**.

## Development

**Prerequisites**: Go 1.25+

```bash
git clone https://github.com/DrSkyle/CloudSlash.git
cd CloudSlash
go run ./cmd/cloudslash --mock
```

## Standards

- **Error Handling**: Wrap errors with context. No silent failures.
- **AWS Safety**: Always validate region/identity. Read-only by default unless `--delete` is set.
- **No Fluff**: Keep comments and UI text direct and technical. Avoid emojis and "AI-like" operational phrases.

## License

By contributing, you agree that your code will be licensed under the **AGPLv3** to maintain the freedom of the project.
