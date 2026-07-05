# Test-plan template

Copy this into the run's report file and fill it in during Phase 1. It is the standard template covering **both manual and automated** flows. Keep the headings; delete the parenthetical guidance.

Principles (from the QA north star):
- Cover every applicable **CLI / config-file / flag** combination.
- Bias hard toward **negative / edge / boundary** cases — a plan that is mostly happy-path is weak.
- Validate **end-to-end workflows / state transitions**, not just single commands.
- Cover the **commonly-impacted areas** (partitions, sequences, status commands, multi-schema) whenever the change could touch them.
- Every scenario has an **expected-outcome oracle**.

---

## Test plan: <feature / PR title>

- **Change under test**: <PR #, branch, commit>
- **Author summary / intent**: <one-liner of what the change does>
- **Docs**: <linked doc changes; do they match the behavior? note any gap>

### Affected surface (from the diff)
| Dimension | Touched? | Details |
| --- | --- | --- |
| Commands | | (export/import schema, export/import data, analyze, assess, cutover, end migration, status, …) |
| CLI flags | | (list new/changed flags) |
| Config-file keys | | (corresponding YAML keys) |
| Migration flows | | (offline / live / fall-forward / fall-back / changes-only) |
| Sources / targets | | (PG source; single-node vs cluster YB; YB version) |
| Commonly-impacted areas | | (partitions / sequences / status commands / multi-schema — mark any plausibly affected) |
| Serialized state | | (MigrationStatusRecord, assessment DB schema, callhome payload — backward-compat risk?) |

### Regression-library scenarios pulled in
(list the `regression-library/*.md` entries that apply to the affected areas — these are mandatory)

### Scenario matrix

Use one row per scenario. `Type`: Positive / Negative / Boundary / State-transition / Usability. `Mode`: CLI / Config. `Auto?`: can an existing migtest cover it (name it) or is it manual.

| # | Type | Scenario | Flow | Mode | Fixture | Expected-outcome oracle | Auto? |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Positive | Happy-path offline migration | offline | CLI | base fixture | source↔target row+content parity; sequences advanced | `pg/<test>` |
| 2 | Boundary | `--adaptive-parallelism-max` below auto jobs | offline | CLI | base | caps+warns, imports fully, **never hangs** | manual |
| 3 | Negative | `--truncate-tables` without `--start-clean` | offline | CLI | base | fast reject naming both flags | manual |
| 4 | Negative | unreachable target host | offline | CLI | base | fast connection error, no stack trace, no hang | manual |
| 5 | State | kill import mid-run, resume | live | CLI | base+delta | resumes from checkpoint; final parity; no dupes | manual |
| … | | | | | | | |

(Expand: every changed flag → at least one positive + one boundary/negative row, in **both** CLI and Config mode. Every relevant commonly-impacted area → at least one row.)

### Execution results (filled in Phase 3)

| # | Result | Oracle satisfied / violated | Observed | Severity | Repro |
| --- | --- | --- | --- | --- | --- |
| | PASS / FAIL / EXPECTED-FAIL / BLOCKED | | | Critical / High / Med / Low | (exact commands) |

### Verdict
- **Summary**: <n passed, m failed, k blocked>
- **Blocking issues**: <list FAILs that should block merge, with severity>
- **New regression-library entries added**: <files created/updated in Phase 4>
- **Environment**: <docker images + versions, binary build method, CLI vs config, single-node vs cluster>
