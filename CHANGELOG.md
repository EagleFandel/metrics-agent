# Changelog

All notable changes to this project will be documented in this file.  
本项目的重要变更会记录在此文件中。

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).  
格式遵循 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，版本遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

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
