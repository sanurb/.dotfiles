# ADR-0007: Admission criteria for new directories under `apps/`

A new directory under `apps/` is justified only when *all three* of the following hold: (a) it has an independent release cadence from existing apps, (b) its audience is partially disjoint from existing apps, (c) sharing a `go.mod` with `apps/cli` would generate parasitic dependencies. Aesthetic appeal of a fuller `apps/` tree is not a criterion — this is the filter that keeps `apps/lsp`, `apps/playground`, and similar speculative additions out of the tree until a real trigger appears.
