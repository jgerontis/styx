---
name: ast-grep
description: Find Go, JavaScript, TypeScript, Python, Rust, and other code structures with ast-grep. Use when a task needs declarations, imports, calls, components, or syntax-aware search rather than literal text.
compatibility: Requires the ast-grep executable available on PATH or through STYX_AST_GREP_PATH.
allowed-tools: search_structure search_text read_file
---

# ast-grep Structural Search

Use `search_text` to locate candidate files and infer their language from file extensions.

Use `search_structure` when the task needs syntax rather than literal text. Its pattern must be valid code in the requested language. `$NAME` matches one syntax node; `$$$NODES` matches zero or more nodes.

Start with the smallest pattern that answers the question. If it finds no matches, remove one constraint and retry once. Then use `read_file` on the relevant lines.