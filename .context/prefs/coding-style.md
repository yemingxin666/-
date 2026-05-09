# Coding Style Guide

> 此文件定义团队编码规范，所有 LLM 工具在修改代码时必须遵守。
> 提交到 Git，团队共享。

## General
- Prefer small, reviewable changes; avoid unrelated refactors.
- Keep functions short (<50 lines); avoid deep nesting (≤3 levels).
- Name things explicitly; no single-letter variables except loop counters.
- Handle errors explicitly; never swallow errors silently.

## Language-Specific

### Go (api/)
- Follow `gofmt` / `goimports`; run `go vet` before commit.
- Errors: wrap with `fmt.Errorf("...: %w", err)`; never ignore.
- Use dependency injection via `uber-go/fx` (existing pattern).
- HTTP handlers: validate input at boundary, return JSON error envelope consistent with existing handlers.

### Vue 3 / JavaScript (web/)
- Composition API + `<script setup>`; avoid Options API in new code.
- Pinia stores: keep state minimal; expose only what's needed.
- Reuse components over copy-paste (DRY); extract to `components/` when used in 2+ places.
- ESLint must pass before commit (`npm run lint`).

## Git Commits
- Conventional Commits, imperative mood.
- Atomic commits: one logical change per commit.
- Scope conventions in this repo: `aicommerce`, `chat`, `mj`, `sd`, etc.

## Testing
- Every feat/fix MUST include corresponding tests when test infra exists.
- Coverage must not decrease.
- Fix flow: write failing test FIRST, then fix code.

## Security
- Never log secrets (tokens/keys/cookies/JWT).
- Validate inputs at trust boundaries.
- Never commit `.env` / `config.toml` / credentials.
