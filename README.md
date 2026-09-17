# Styx

A self-hosted, terminal-based AI agent harness — like OpenCode, focused on doing real work in your codebase without context rot.

## What Styx Is

Styx is a CLI agent that pairs a language model with your local tools (filesystem, shell, search, edit) and a disciplined execution loop. It runs entirely on your machine, works with local or hosted models, and is built to be extended.

- **Terminal-first.** No GUI, no web UI — just a fast, scriptable CLI/REPL. A GUI workspace, if it ever exists, is a separate application.
- **Local & provider-agnostic.** Ollama and OpenAI out of the box; adding a provider is one file.
- **Extensible by design.** New tools, providers, and skills plug in without touching the core.

## Design Philosophy

### 1. Context is the scarcest resource — protect it from rot
The single biggest driver of agent quality is what's in the model's context *right now*. Styx treats context as precious: every phase of work gets a **freshly assembled, minimal context** containing only what that step needs — never an ever-growing transcript of prior reasoning. Stale reasoning is distilled into compact artifacts and dropped.

### 2. Plan → (Write Tests) → Implement → Validate
Every request runs through an explicit, repeating loop rather than one big free-form generation:

| Phase | Sees | Produces |
|-------|------|----------|
| **1. Plan** | the request + scoped files | a concrete plan; decides if tests are warranted |
| **2. Write Tests** *(skipped if not needed)* | just the test description | test code (the spec) |
| **3. Implement** | the plan summary + the **test code** + target files | the change |
| **4. Validate** | the tests + the change + run output | pass / fail |

The payoff: **Implement anchors to test code — a precise, compact spec — not to verbose planning reasoning.** On failure, a fresh cycle begins; Plan reviews what went wrong and decides whether the tests still stand.

### 3. Test-driven, not test-obsessed
Styx is TDD-forward: when a change deserves a test, the test is written first and becomes the definition of "done." When a change is trivial (a one-line color tweak), Phase 2 is simply skipped. No ceremony for the sake of it.

### 4. Problem decomposition
A prompt is a **Job**, broken into as many sub-tasks as needed. Each sub-task runs its own Plan→…→Validate cycle. Large, open-ended requests ("redesign the landing page") start with a collaborative planning conversation before any code is written.

### 5. Tools are primitives; Skills are knowledge
Following the [Agent Skills](https://agentskills.io) open standard:
- **Tools** are the agent's built-in hands — filesystem, shell, search, edit.
- **Skills** are portable `SKILL.md` bundles that teach the agent *how* to use those hands for a domain, loaded on demand (progressive disclosure) and gated by `allowed-tools`.

This keeps skills portable across any skills-compatible agent — grab a `SKILL.md` from anywhere and Styx can use it.

## System Architecture

Everything is wired through a **Runtime** container (dependency injection, no global state):

```
cmd/styx/            CLI entry point (Cobra)
internal/
  runtime/           DI container: config, logger, bus, registries, store
  config/            layered config — flags > env > file > defaults
  log/               structured logging (slog)
  event/             event bus — agent publishes, the TUI subscribes
  provider/          streaming LLM transports (Ollama, OpenAI) + Model abstraction
  message/           messages as content blocks (multimodal-ready)
  tool/              Tool interface + registry + built-ins (fs, shell, search, edit)
  permission/        approval layer — implements skills' allowed-tools
  middleware/        composable pipeline around tool/provider calls
  skill/             agentskills.io loader (SKILL.md, progressive disclosure)
  agent/             the Plan→Test→Implement→Validate loop + context assembly
  state/             Jobs, Cycles, and pluggable session storage
skills/              built-in skills (agentskills.io format)
~/.styx/             user config, installed skills, session checkpoints
```

### Key architectural choices

- **Streaming-first.** Providers return a stream of deltas; a `Collect()` helper gives the synchronous case for free. Retrofitting streaming later would mean a rewrite, so it's baked in from day one.
- **Parallel tool calls.** The model can request several tools at once; they execute concurrently, each passing through the permission gate.
- **JSON Schema tool definitions.** Native to function-calling APIs and precise enough to drive `allowed-tools` gating — no bespoke parameter format.
- **Event bus.** The agent core publishes events (tokens, tool start/stop, approval requests); the CLI is a pure subscriber. The core has no UI assumptions.
- **Middleware pipeline.** Permissions, logging, and metrics are composable wrappers around tool and provider calls rather than tangled into the loop.
- **Approval as a first-class concern.** Read-only operations auto-run outside system paths; writes, edits, and shell commands prompt. `allowed-tools` from a skill pre-approves specific tools.
- **Replace-versioning for artifacts.** Only the latest plan and implementation are retained; a short note records "tried X, it failed because Y" so nothing important is lost while stale code never lingers in context.

## Scope (v1)

**In:** streaming providers (Ollama, OpenAI), built-in tools (fs / shell / search / edit), the 4-phase TDD loop, agentskills.io skills, permissions + middleware, event bus, file-based sessions, Cobra CLI/REPL.

**Out (for now):** database persistence, sub-agents / multi-agent orchestration, a GUI workspace, HTTP-fetch tooling, dynamic plugin loading, and active multimodal input (the message shape supports it, but v1 is text-only).

## Status

Early development. Foundations (runtime, config, event bus, streaming providers) are in place; the tool layer, skills, and agent loop are being built out.

## License

MIT — see [LICENSE](LICENSE).
