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

### 6. System 1 gates System 2
Not every decision in the loop needs a full LLM round-trip. A **System 1 gate** — a small, fast, non-generative classifier — can answer narrow, calibrated questions ("does this request need a skill, and which one?", "does this request need tests?", "does this input look like a prompt injection?") in milliseconds, before the slower, generative **System 2** LLM is ever invoked. System 1 never generates tool calls, code, or arguments — it only classifies, scores, or gates with a confidence value, and low-confidence answers always fall back to System 2 or the user. The backend is pluggable and never assumed: a small local model called through Styx's own provider abstraction, a self-hosted sidecar, or a remote API are all just different `system1.Gate` implementations. Styx runs fully without any of them; it's a pure accelerant/filter layered onto the existing loop, never a hard dependency.

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
  system1/           optional fast-gate client (System 1) — typed decisions, confidence gating
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
- **System 1 fails safe.** The `system1.Gate` is `Noop` by default; a real gate is opt-in via config, whether that's a small local model called through the existing `provider` abstraction or an HTTP call to a self-hosted sidecar or remote API. Any gate error, timeout, or low-confidence answer degrades to the existing static rules and full LLM path — System 1 can only narrow, never widen, what already required approval.

### Agent modes

Modes compose the shared harness rather than duplicate it. Each mode owns its system prompt, context assembler, and allowed tool subset; providers, streaming, tool validation, approval, and loop guards remain shared. Planned modes are `chat` for exploration, `plan` for read-only decomposition, `implement` for supervised changes, `validate` for focused checks, and `work` for the full Plan → Test → Implement → Validate cycle.

## Scope (v1)

**In:** streaming providers (Ollama, OpenAI), built-in tools (fs / shell / search / edit), the 4-phase TDD loop, agentskills.io skills, permissions + middleware, event bus, file-based sessions, Cobra CLI/REPL.

**Out (for now):** database persistence, sub-agents / multi-agent orchestration, a GUI workspace, HTTP-fetch tooling, dynamic plugin loading, and active multimodal input (the message shape supports it, but v1 is text-only).

## Status

### Milestones

1. **Usable Ollama chat CLI** *(complete)*: `styx chat` connects to Ollama, verifies the requested model, streams responses, preserves an in-memory conversation, and supports `/reset` and `/exit`.
2. **Safe tool primitives** *(complete)*: grounded workspace inspection; bounded `list_files`, line-ranged `read_file`, literal `search_text`, exact-match `edit_file`, new-file-only `write_file`, and timeout-bounded `run_command` tools. JSON Schema validation rejects malformed calls before execution. Read-only tools auto-run; writes and commands receive one user approval.
3. **Skills** *(current)*: load agentskills.io-compatible `SKILL.md` bundles using progressive disclosure and `allowed-tools` permissions.
4. **Plan/Test/Implement/Validate harness**: add isolated job phases, context assembly, and TDD-forward validation.
5. **Durable sessions and refinement**: file-backed job artifacts, context-window strategies, and high-quality retry behavior.
6. **System 1 skill selection**: a two-stage gate — a cheap rank-and-threshold pass over every skill's name/description, then a reread of the top few candidates' bodies to confirm fit — decides which skill, if any, gets injected, instead of the model choosing from a full skill listing every turn. Backend is pluggable (a small local model via the existing `provider` abstraction by default; a sidecar or remote API optional); with no gate configured, Styx falls back to today's model-driven `load_skill` behavior unchanged.
7. **Extend System 1 usage** *(exploratory)*: mode routing and phase-skip triage in the Plan/Test/Implement/Validate loop, and prompt/tool-output guardrails before content re-enters context. Approval gating (`permission.Policy.Evaluate`) stays lowest priority and off by default — a System 1 model narrowing what's already required approval is a much higher bar to clear than narrowing what enters context.
8. **Markdown-rendered output**: render completed assistant output (code fences, bold, lists) as ANSI in the terminal instead of raw text. Contained to how `runTurnWithLimits` writes output; no change to the event bus or I/O structure.
9. **Real TUI**: the chat loop publishes to `event.Bus` (tokens, tool start/stop, approval requests) instead of writing `io.Writer` directly; a `bubbletea`/`lipgloss` subscriber replaces the scanner/stdout loop with scrollable panes, live tool-call status, and approval prompts as UI widgets. Deliberately sequenced last — it renders job/phase state that milestones 4-7 still need to shape.

Early development. Milestones 1 and 2 are complete; Skills are in progress.

## Build and Install

Build a local executable:

```bash
make build
./bin/styx chat --model gemma4:12b
```

Install `styx` into Go's binary directory (`GOBIN`, or `$(go env GOPATH)/bin`) so it can run from any repository:

```bash
make install
styx chat --model gemma4:12b
```

Ensure that Go's binary directory is on your `PATH`. Run the full local pipeline with:

```bash
make check
```

### Skills

Styx discovers [Agent Skills](https://agentskills.io) from three layers: built-ins embedded in the executable, global `~/.styx/skills/`, and repository-local `.styx/skills/`. A skill is a directory containing `SKILL.md` with required `name` and `description` YAML frontmatter. Later layers override earlier skills with the same name, so repository-local skills override global skills, and global skills override built-ins. Metadata is loaded by `styx skills list`; instructions are loaded only by `styx skills show <name>` or future mode activation.

```bash
styx skills list
styx skills show code-review
```

## License

MIT — see [LICENSE](LICENSE).
