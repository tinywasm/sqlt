# PLAN — Kind unification (phase B): SQLite DDL reads `Field.Type.Storage()`

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Phase B of `tinywasm/docs/KIND_UNIFICATION_MASTER_PLAN.md` (Kind unification wave). Requires
> the published phase-A `tinywasm/model`. Runs parallel to orm/form/postgres/mcp.

## Context (zero-context summary)

Phase A changed `tinywasm/model`: `Field.Type` is no longer the `FieldType`
enum but the interface

```go
type Kind interface {
    Storage() FieldType   // the enum survives here — same values, same meaning
    Name() string
    Validate(value string) error
}
```

This repo translates `model.Field` schemas to SQLite DDL and compares
`f.Type` against enum values directly (e.g. `translate.go:69`
`f.IsAutoInc() && f.Type == model.FieldInt`, `translate.go:140`
`f.Type == model.FieldText && f.Maximum > 0`, and `sqliteColumnType`'s
switch).

## Stage 1 — mechanical migration

- Bump `tinywasm/model` to the phase-A version.
- Every `f.Type == model.FieldX` / `switch f.Type` site becomes
  `f.Type.Storage()`. Grep the whole module for `\.Type` (translate.go is
  the known hotspot; check compiler errors for the rest). The enum values
  compared against are unchanged — zero behavior change intended.
- Test fixtures that build `model.Field` literals by hand now use the
  phase-A base kind constructors (`Type: model.Text()`, `model.Int()`, …).

## Stage 2 — tests

- `gotest ./...` green with no weakened assertions: generated DDL for the
  existing fixtures must be byte-identical to before the migration.

## Harness checklist (mandatory)

- No behavior change: this is a call-site migration, not a redesign. If the
  `Kind` contract is insufficient here, **STOP and report** to the master
  plan.
- No unrelated refactors; typed constants stay; `gotest` only.
- Breaking dependency bump: next minor version.

## Acceptance criteria

1. Module compiles against phase-A model; no direct enum comparison on
   `f.Type` remains (all via `.Storage()`).
2. DDL output for existing fixtures is unchanged; `gotest ./...` green.

## Stages

| Stage | File(s) | Action |
|---|---|---|
| 1 | `translate.go` (+ any site the compiler flags), test fixtures | `.Storage()` migration, base-kind literals |
| 2 | `*_test.go` | DDL byte-identical regression |
