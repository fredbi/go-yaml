# analysis

Reproducible measurements behind [`../ANALYSIS-go-openapi.md`](../ANALYSIS-go-openapi.md).

A **separate module**: it needs `go.yaml.in/yaml/v3` as a comparison baseline, and the root
module has zero external dependencies, which is worth preserving. `go test ./...` from the
repository root does not descend into it.

```sh
# the findings, as readable output
go test -v ./...

# throughput
go test -run XXX -bench . -benchtime 2s ./...

# where the time and the allocations go
go test -run XXX -bench Parse -cpuprofile cpu.out -memprofile mem.out ./...
go tool pprof -top -cum  -nodecount=20 cpu.out
go tool pprof -top -sample_index=alloc_space   -nodecount=15 mem.out
go tool pprof -top -sample_index=alloc_objects -nodecount=15 mem.out
```

`TestFlatMapScaling` is the important one: it prints the growth factor per doubling of
sibling-key count. **~2 is linear and correct; ~4 means the quadratic `parseMap` defect is
still present.** It is a report, not an assertion, so it never fails the build — see
`TestParseScalesLinearly` for the guard to enable once the fix lands.
