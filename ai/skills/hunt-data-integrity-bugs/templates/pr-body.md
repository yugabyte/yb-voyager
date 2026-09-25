### Describe the changes in this pull request

Adds a failing test that reproduces **silent <data loss | data corruption>** in <flow>: <one-line symptom>.

The test is expected to **fail until the bug is fixed** — CI on this PR is red by design. Found by the automated data-integrity hunt (plan `<plan file name>`, case `<case id>`, mechanism `<Mx>`).

**Setup**
```sql
<minimal schema>
```
<data and flags, one line each>

**Workload**
```sql
<minimal source statements>
```

**Expected:** <target matches source / voyager refuses the configuration>
**Actual:** <exact divergence: rows missing / wrong values, with counts>. Import <kept running | exited 0> and logged only <WARN lines, or nothing>.

**Why** (from code reading and the run evidence): <2–4 sentences: where it goes wrong, with file:function>.

**Reproduced** <k>/3 runs at `<commit>`.

**How to run**
```bash
cd yb-voyager
go test -tags <tag> -count=1 -v -run '<TestName>' ./src/testlivemigration/
```

**Suggested fix:** <guardrail / code change>.

### Describe if there are any user-facing changes

No — test only.

### How was this pull request tested?

The test itself is the reproduction; it fails on `<commit>`.

### Does your PR have changes in callhome/yugabyted payloads? If so, is the payload version incremented?

No.

### Does your PR have changes to on-disk structures that can cause upgrade issues?

No.
