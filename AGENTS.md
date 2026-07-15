# Project Instructions

## Product Direction

- This is a commercial-ready Windows and macOS desktop application for long-form fiction creation.
- The approved desktop stack is Rust, Tauri 2, React, TypeScript, Vite, SQLite, and SQLx.
- The accepted browser prototype lives in `prototype/web` and is the UX reference, not the production architecture.
- Preserve heterogeneous multi-model review: different agents must be able to use different providers and models.

## Workflow

- Read `docs/handoff/START-HERE.md`, `docs/product/product-baseline.md`, and architecture decisions before planning changes.
- Complete product and architecture specifications before scaffolding production modules.
- Keep domain, application, persistence, agent runtime, provider adapters, and UI boundaries explicit.
- Use test-driven development for production features and verify Windows/macOS implications.
- Never commit API keys, model credentials, signing certificates, user manuscripts, or local databases.
- Keep the accepted prototype operational while production code is developed separately.

## Runtime Notes

- When Python is required, use the pyenv-managed Python 3.12 runtime, currently 3.12.3.
- Prefer `pyenv exec python` or `pyenv exec python3`; do not use the system Python directly.

