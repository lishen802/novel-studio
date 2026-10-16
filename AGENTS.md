# Project Instructions

## Product Direction

- This is a commercial-ready Windows and macOS desktop application for long-form fiction creation.
- The approved primary stack is Go, Wails v2, React, TypeScript, Vite, and SQLite.
- Rust is optional and must be introduced only for a measured performance bottleneck, a missing reliable Go library, or a native capability that is materially safer or more maintainable in Rust.
- Prefer an isolated Rust sidecar with a versioned stdin/stdout or local IPC contract. Do not add CGO/FFI without a separate approved architecture decision.
- The accepted browser prototype lives in `prototype/` and is the authoritative standard for frontend visuals and interaction. Production UI must align with the prototype's design tokens, layout, and component structure; where backend capabilities are not yet implemented, render the prototype's UI with local state and an explicit "not ready" notice rather than deviating from the prototype.
- Preserve heterogeneous multi-model review: different agents must be able to use different providers and models.

## Workflow

- Before planning changes, read only the three long-lived authority documents: `docs/product/requirements.md`, `docs/product/architecture.md`, and the applicable stage in `docs/product/roadmap.md`.
- For a large feature, also read its single feature design in `docs/features/R<stage>/`. Do not load completed or unrelated feature documents into context.
- A large feature must have one detailed feature design before implementation. Create the stage directory only when that stage has an active feature, for example `docs/features/R7/长期记忆.md`; do not pre-create empty stage directories. Multiple sibling subfeatures of the same stage belong in the same `R<stage>` directory. Feature design filenames must use clear Chinese names and must not include dates.
- A feature design is both the design specification and implementation plan. Update the same document when its behavior, API, data model, state machine, test contract, or implementation sequence changes; do not create separate fix, recovery, adjustment, supplement, or V2 plan documents.
- Small implementation bugs that do not change design require code and tests only. Do not create documentation merely to record a bug fix.
- The roadmap defines stages, dependencies, and exit criteria only. It must not link to or enumerate feature design documents.
- Git history is the source for obsolete plans and implementation history. Do not keep superseded documents in the current documentation tree.
- Complete product and architecture specifications before scaffolding production modules.
- Keep Go domain, application, persistence, agent runtime, provider adapters, platform adapters, and UI boundaries explicit.
- Keep Wails bindings thin; React components must not call persistence or model providers directly.
- Use test-driven development for production features and verify Windows/macOS implications.
- Keep Go code idiomatic and cohesive in line with Effective Go and Go Code Review Comments: avoid package-name stuttering in exported identifiers, define small interfaces at the consumer, keep files and types focused, wrap errors with actionable context using `%w`, and before commit require `gofmt`, `go vet`, and affected-package tests to pass; cross-package or concurrent changes must additionally pass full-repository tests and targeted `go test -race`.
- For small, narrowly scoped feature optimizations and maintenance changes, do not create planning or implementation documents. Explain the impact and obtain confirmation in conversation, then implement directly on the current branch.
- Add concise Chinese comments to key functions explaining their purpose; all code comments must be in Chinese.
- For large features or changes spanning multiple modules, database schema, or major product capabilities, create an isolated Git worktree automatically without requesting confirmation.
- After a large feature is complete and verified, merge its feature branch into `main` automatically without requesting confirmation.
- After any requested change is complete and proportionally verified, automatically create a scoped Git commit without waiting for a separate commit request. Include only files belonging to the completed change, preserve unrelated worktree changes, and do not push unless the user explicitly asks.
- Never commit API keys, model credentials, signing certificates, user manuscripts, or local databases.
- Keep the accepted prototype operational; production frontend must follow the prototype's visual and interaction design.
- Wails v3 is not the production baseline while it remains Alpha. Re-evaluate it through a new architecture decision after a stable release.

## Documentation Structure

- `docs/product/requirements.md`: product scope, user value, functional requirements, release boundaries, and product acceptance.
- `docs/product/architecture.md`: product interaction, domain rules, Agent design, data boundaries, technical architecture, reliability, and quality gates.
- `docs/product/roadmap.md`: stage order, scope, dependencies, status, and exit criteria.
- `docs/features/R<stage>/<中文功能名称>.md`: the only design and implementation document for one subfeature. Stage directories are created on demand; for example, all R7 subfeature designs belong under `docs/features/R7/`.
- Do not add `handoff`, `plans`, `architecture`, `roadmap`, `testing`, `operations`, or archive document trees. Put durable global decisions into the three product documents, executable behavior into code and tests, and history into Git.

## Runtime Notes

- When Python is required, use the pyenv-managed Python 3.12 runtime, currently 3.12.3.
- Prefer `pyenv exec python` or `pyenv exec python3`; do not use the system Python directly.

## Cursor Cloud specific instructions

- Prefer `/usr/local/go/bin` and `/usr/local/bin` at the front of `PATH`. They provide Go 1.25.12, Node.js 22.23.3, and Wails v2.13.0. A login shell may also expose another Node earlier on `PATH`.
- `apps/desktop/main.go` embeds `frontend/dist`. Before `go test` on `./apps/desktop`, run `npm --prefix apps/desktop/frontend run build` if that directory is missing.
- On Ubuntu, build the desktop shell from `apps/desktop` with `wails build -m -tags webkit2_41`. This image provides WebKitGTK 4.1. macOS and Windows release builds stay on CI.
- The accepted prototype listens on `http://127.0.0.1:5173/` after startup. Use it for browser interaction. The native window is `apps/desktop/build/bin/NovelStudio` after the Linux build above.
