# Changelog

All notable changes to this project will be documented in this file.  
本项目的重要变更会记录在此文件中。

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).  
格式遵循 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，版本遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [v1.2.5] - 2026-02-16

### Added / 新增
- Added GitHub CI workflow: `.github/workflows/ci.yml`.
- 新增 GitHub CI 工作流：`.github/workflows/ci.yml`。
- Added issue templates for bug reports and feature requests.
- 新增 Bug 与 Feature 的 Issue 模板。
- Added pull request template and issue template config (including security contact link).
- 新增 PR 模板和 Issue 配置（包含安全报告入口）。

### Changed / 变更
- Added CI and Release badges in `README.md`.
- 在 `README.md` 增加 CI 与 Release 徽章。
- Improved collaboration baseline for external contributors and maintainers.
- 强化外部贡献者与维护者的协作基础设施。

## [v1.2.4] - 2026-02-15

### Added / 新增
- Added open-source governance files: `LICENSE`, `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`.
- 新增开源治理文件：`LICENSE`、`CONTRIBUTING.md`、`SECURITY.md`、`CODE_OF_CONDUCT.md`。

### Changed / 变更
- Aligned `README.md` and `docs/COOLIFY_INTEGRATION.md` with actual API routes (`/api/*`) and runtime configuration.
- 将 `README.md` 与 `docs/COOLIFY_INTEGRATION.md` 对齐到实际 API 路由（`/api/*`）和运行配置。
- Replaced `API_TOKEN` references with `METRICS_AGENT_TOKEN` in integration docs.
- 将集成文档中的 `API_TOKEN` 统一替换为 `METRICS_AGENT_TOKEN`。
- Clarified port semantics: default app port is `3000`, while Compose deployment examples use `8080`.
- 明确端口语义：应用默认端口为 `3000`，Compose 部署示例使用 `8080`。

## [v1.2.3] - 2025-12-29

### Added / 新增
- Added `/api/containers/:id/all` endpoint to fetch `stats + history + requests` in one request.
- 新增 `/api/containers/:id/all` 聚合端点，单次请求返回 `stats + history + requests`。

### Improved / 优化
- Improved API responsiveness with cached CPU values and improved metrics collection flow.
- 通过 CPU 缓存与采集流程优化，提升接口响应性能。
