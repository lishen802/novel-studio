# Project Instructions

## Product Direction

- This is a commercial-ready Windows and macOS desktop application for long-form fiction creation.
- The approved primary stack is Go, Wails v2, React, TypeScript, Vite, and SQLite.
- Rust is optional and must be introduced only for a measured performance bottleneck, a missing reliable Go library, or a native capability that is materially safer or more maintainable in Rust.
- Prefer an isolated Rust sidecar with a versioned stdin/stdout or local IPC contract. Do not add CGO/FFI without a separate approved architecture decision.
- The accepted browser prototype lives in `prototype/web` and is the UX reference, not the production architecture.
- Preserve heterogeneous multi-model review: different agents must be able to use different providers and models.

## Workflow

- Read `docs/handoff/START-HERE.md`, `docs/product/product-baseline.md`, and architecture decisions before planning changes.
- Complete product and architecture specifications before scaffolding production modules.
- Keep Go domain, application, persistence, agent runtime, provider adapters, platform adapters, and UI boundaries explicit.
- Keep Wails bindings thin; React components must not call persistence or model providers directly.
- Use test-driven development for production features and verify Windows/macOS implications.
- Small, narrowly scoped feature optimizations and maintenance changes may be implemented directly on `main` without a separate implementation plan.
- For large features or changes spanning multiple modules, database schema, or major product capabilities, create an isolated Git worktree automatically without requesting confirmation.
- After a large feature is complete and verified, merge its feature branch into `main` automatically without requesting confirmation.
- Never commit API keys, model credentials, signing certificates, user manuscripts, or local databases.
- Keep the accepted prototype operational while production code is developed separately.
- Wails v3 is not the production baseline while it remains Alpha. Re-evaluate it through a new architecture decision after a stable release.

## Runtime Notes

- When Python is required, use the pyenv-managed Python 3.12 runtime, currently 3.12.3.
- Prefer `pyenv exec python` or `pyenv exec python3`; do not use the system Python directly.
