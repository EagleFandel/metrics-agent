# Contributing Guide / 贡献指南

Thanks for your interest in contributing to `metrics-agent`.  
感谢你关注并参与 `metrics-agent` 项目。

Repository: `https://github.com/EagleFandel/metrics-agent`

## 1. Development Environment / 开发环境

- Go `1.24+`
- Docker (required for runtime behavior and local integration tests)
- A shell environment that can access Docker socket when needed

## 2. Local Setup / 本地启动

```bash
git clone https://github.com/EagleFandel/metrics-agent.git
cd metrics-agent
go mod download
METRICS_AGENT_TOKEN=test123 go run main.go
```

Quick checks / 快速检查：

```bash
curl http://localhost:3000/api/health
curl -H "Authorization: Bearer test123" http://localhost:3000/api/containers
```

## 3. Testing / 测试

Run all tests:

```bash
go test ./...
```

Before opening a PR, make sure your branch passes the test command above.  
提交 PR 前，请确保上述测试命令通过。

## 4. Commit Convention / 提交规范

This project uses Conventional Commits where possible:

- `feat: ...`
- `fix: ...`
- `docs: ...`
- `refactor: ...`
- `test: ...`
- `chore: ...`

Examples:

- `feat: add request stats endpoint`
- `docs: align Coolify integration examples`

## 5. Pull Request Checklist / PR 检查清单

- [ ] The change is focused and clearly described.
- [ ] API behavior and docs stay consistent.
- [ ] `go test ./...` passes.
- [ ] Security-sensitive changes are explained in the PR description.
- [ ] Breaking changes are explicitly called out (if any).

## 6. Issue Reporting / 问题反馈

- Bug reports and feature requests: `https://github.com/EagleFandel/metrics-agent/issues`
- Security issues: see `SECURITY.md` for private reporting workflow.

## 7. Code of Conduct / 行为准则

By participating, you agree to follow `CODE_OF_CONDUCT.md`.  
参与本项目即表示你同意遵守 `CODE_OF_CONDUCT.md`。
