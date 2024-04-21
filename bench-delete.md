| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `go run ./cmd/deleter/main.go ./test/.git` | 840.3 ± 648.5 | 504.2 | 2137.6 | 1.00 |
| `go run ./cmd/deleter/main.go --parallel ./test/.git` | 902.1 ± 885.1 | 476.1 | 3168.0 | 1.07 ± 1.34 |
