"""
Transaction mode for the event generator (opt-in, config-gated, default OFF).

See event-generator.yaml's commented-out 'generator.transaction_mode' block
for the config shape, and utils.validate_transaction_mode for validation.

When disabled (the block absent, or 'enabled: false'), generator.py's main
loop is completely untouched -- this module is only consulted via
is_transaction_mode_enabled(), and nothing else here is ever called.

When enabled, each main-loop iteration builds one "transaction plan"
(build_transaction_plan, pure -- no DB access) and executes it
(run_transaction, against a live cursor/connection):

    BEGIN
    [SAVEPOINT sp_k ... RELEASE SAVEPOINT sp_k]*   (release = success)
    <H statements against a random 'hot' table>
    <O statements against a non-hot table, existing table_weights>
    COMMIT

interleaved in one shuffled order, with savepoint pairs wrapping random
contiguous sub-runs of that order. INSERT is always single-row. UPDATE and
DELETE are single-row by default too, but can each affect up to k rows in
one statement when the optional 'rows_per_statement' config key is set
(k sampled per statement from its [min, max]) -- see execute_single_statement.
All statements reuse the same builders (build_insert_values /
build_update_values / build_pk_in_condition / build_sampling_condition) and
PK-pool-first targeting the legacy single-op path in generator.py uses.

Rate accounting: each COMMITTED ROW counts as one event -- a k-row
UPDATE/DELETE statement contributes k events, not 1. (When
'rows_per_statement' is absent/default, every statement is single-row, so
this coincides with "one event per statement", matching the pre-existing
behavior byte-for-byte.) This matches generator.py's
GOVERNOR.pace(events_emitted) call for the legacy path, which is also
rows (max(cursor.rowcount, 0)), not statements.

Error handling: run_transaction does not catch anything itself. If any
statement raises, the (already open) DB transaction is simply abandoned
mid-flight and the exception propagates to generator.py's existing
per-iteration `except psycopg2.Error` handler -- the same one the legacy
single-op path already relies on for conn.rollback() / the "current
transaction is aborted" message / reconnect-on-connection-loss. This
avoids duplicating that logic here, and events_emitted for the iteration
stays at its pre-set 0 (run_transaction never returns on failure).
"""

import random as _random_module
from typing import Any, Callable, Dict, List, Optional, Tuple

from utils import (
    build_insert_values,
    build_pk_in_condition,
    build_sampling_condition,
    build_update_values,
    get_insert_column_list,
    quote_ident,
)

_OPS = ("INSERT", "UPDATE", "DELETE")


def is_transaction_mode_enabled(tm_cfg: Optional[Dict[str, Any]]) -> bool:
    """True only when the 'transaction_mode' block is present AND
    'enabled: true'. Absent block, or enabled false/missing, => False --
    the gate generator.py uses to keep running the legacy single-op path
    byte-for-byte unchanged."""
    if not tm_cfg:
        return False
    return bool(tm_cfg.get("enabled", False))


def _weighted_ops(op_weights: Dict[str, Any]) -> Tuple[List[str], List[float]]:
    """Same shape as generator.py's OPERATIONS/OPERATION_WEIGHTS build: drop
    any op with weight <= 0."""
    names: List[str] = []
    weights: List[float] = []
    for op in _OPS:
        w = float(op_weights.get(op, 0) or 0)
        if w > 0:
            names.append(op)
            weights.append(w)
    return names, weights


def resolve_txn_counts(tm_cfg: Dict[str, Any], rng: Any) -> Tuple[int, int, int]:
    """Choose (T, H, O) for one transaction.

    Samples T from statements_per_txn, H from hot_statements_per_txn, and O
    from other_statements_per_txn independently, then reconciles H+O == T
    by adjusting O first, then H -- each clamped to stay within its own
    configured [min, max]. The returned T is always exactly H+O (by
    construction), which equals the originally sampled T whenever the
    configured ranges are "compatible" (statements_per_txn.min <=
    hot_statements_per_txn.min + other_statements_per_txn.min and
    statements_per_txn.max >= hot_statements_per_txn.max +
    other_statements_per_txn.max) -- assumed here; see
    utils.validate_transaction_mode, which does not currently enforce this
    cross-field relationship, so a pathological/incompatible config can
    still produce a returned T outside statements_per_txn's own range in
    the (documented) edge case where H/O can't fully reach it.

    `rng` is any object exposing `.randint(a, b)` -- generator.py passes
    the already-seeded module-level `random`; tests pass a
    `random.Random(seed)` instance for determinism.
    """
    t_rng = tm_cfg["statements_per_txn"]
    h_rng = tm_cfg["hot_statements_per_txn"]
    o_rng = tm_cfg["other_statements_per_txn"]

    T = rng.randint(t_rng["min"], t_rng["max"])
    H = rng.randint(h_rng["min"], h_rng["max"])
    O = rng.randint(o_rng["min"], o_rng["max"])

    delta = T - (H + O)
    if delta > 0:
        # H+O too small: grow O first, then H, each capped at its own max.
        bump = min(delta, o_rng["max"] - O)
        O += bump
        delta -= bump
        bump = min(delta, h_rng["max"] - H)
        H += bump
        delta -= bump
    elif delta < 0:
        # H+O too big: shrink O first, then H, each floored at its own min.
        need = -delta
        cut = min(need, O - o_rng["min"])
        O -= cut
        need -= cut
        cut = min(need, H - h_rng["min"])
        H -= cut
        need -= cut

    return H + O, H, O


def choose_savepoint_ranges(
    total_statements: int,
    sp_cfg: Dict[str, int],
    rng: Any,
) -> List[Tuple[int, int]]:
    """Choose non-overlapping contiguous 0-based inclusive index ranges
    ("sub-runs") over `total_statements` statements, to wrap in
    SAVEPOINT/RELEASE SAVEPOINT pairs.

    Number of ranges P is sampled from sp_cfg's [min, max], clamped to
    [0, total_statements] -- a pair needs at least one statement, and pairs
    never overlap, so there can never be more pairs than statements.

    Algorithm: draw P distinct start indices (rng.sample -- no two ranges
    can share a start), sort them, then for each start pick a random run
    length bounded by the gap to the next start (or the end of the
    statement list for the last one). This guarantees P disjoint,
    contiguous, length >= 1 ranges in a single pass, with no combinatorial
    partitioning needed.
    """
    if total_statements <= 0:
        return []

    lo = max(0, sp_cfg.get("min", 0))
    hi = max(lo, sp_cfg.get("max", 0))
    hi = min(hi, total_statements)
    lo = min(lo, hi)
    p = rng.randint(lo, hi) if hi >= lo else 0
    if p <= 0:
        return []

    starts = sorted(rng.sample(range(total_statements), p))
    ranges: List[Tuple[int, int]] = []
    for idx, start in enumerate(starts):
        next_start = starts[idx + 1] if idx + 1 < len(starts) else total_statements
        max_len = next_start - start
        length = rng.randint(1, max_len)
        ranges.append((start, start + length - 1))
    return ranges


def build_transaction_plan(
    tm_cfg: Dict[str, Any],
    hot_tables: List[str],
    other_table_weights: Dict[str, float],
    rng: Any,
) -> Dict[str, Any]:
    """Build one transaction's statement plan. Pure -- no DB access.

    Returns {"statements": [{"table": str, "operation": "INSERT"|"UPDATE"|"DELETE",
    "hot": bool}, ...], "savepoint_ranges": [(start, end), ...]}
    (0-based inclusive indices into "statements").

    H hot statements each target `rng.choice(hot_tables)`, operation chosen
    by hot_op_weights. O other statements each target a table chosen from
    `other_table_weights` (generator.py passes RESOLVED_TABLE_WEIGHTS with
    hot_tables excluded), operation by other_op_weights. The combined H+O
    statements are then shuffled together so a transaction reads like
    interleaved mixed traffic, and savepoint ranges are chosen over that
    final shuffled order.

    If `other_table_weights` is empty (e.g. every known table is 'hot'),
    O is forced to 0 rather than raising -- there is nothing non-hot left
    to target.
    """
    _, H, O = resolve_txn_counts(tm_cfg, rng)

    other_tables = list(other_table_weights.keys())
    if not other_tables:
        O = 0

    hot_op_names, hot_op_w = _weighted_ops(tm_cfg["hot_op_weights"])
    other_op_names, other_op_w = _weighted_ops(tm_cfg["other_op_weights"])

    statements: List[Dict[str, Any]] = []
    for _ in range(H):
        table = rng.choice(hot_tables)
        op = rng.choices(hot_op_names, weights=hot_op_w)[0]
        statements.append({"table": table, "operation": op, "hot": True})
    for _ in range(O):
        table = rng.choices(other_tables, weights=list(other_table_weights.values()))[0]
        op = rng.choices(other_op_names, weights=other_op_w)[0]
        statements.append({"table": table, "operation": op, "hot": False})

    rng.shuffle(statements)

    savepoint_ranges = choose_savepoint_ranges(
        len(statements), tm_cfg["savepoint_pairs_per_txn"], rng
    )

    return {"statements": statements, "savepoint_ranges": savepoint_ranges}


def execute_single_statement(
    cursor: Any,
    table_schemas: Dict[str, Any],
    pools: Optional[Dict[str, Any]],
    db_flavor: str,
    row_estimates: Optional[Dict[str, int]],
    column_overrides: Optional[Dict[str, Any]],
    min_col_size_bytes: int,
    pk_value_fn_for_table: Optional[Callable[[str], Optional[Callable[[], Any]]]],
    unique_value_fns_for_table: Optional[Callable[[str], Optional[Dict[str, Callable[[], Any]]]]],
    table_name: str,
    operation: str,
    rows_per_statement: Tuple[int, int] = (1, 1),
) -> Tuple[int, Tuple[List[Any], List[Any]]]:
    """Execute one INSERT/UPDATE/DELETE for `table_name` on `cursor`,
    mirroring generator.py's legacy per-operation branch (same builders,
    same PK-pool-first targeting) but with no per-statement retry (see
    module docstring: a mid-transaction error aborts the whole transaction
    via the caller's existing handler, rather than being retried in place
    like the legacy path's execute_with_retry).

    INSERT is always single-row (rows_per_statement is ignored for it).
    UPDATE and DELETE each affect k rows, k = rng.randint(*rows_per_statement)
    drawn fresh per call from the module-level `_random_module` (matching how
    the UPDATE column-count pick already draws from `_random_module` directly
    rather than the plan-building rng) -- k collapses to 1 under the default
    (1, 1), the original single-row behavior.

    Returns (rowcount, (pool_add_ids, pool_remove_ids)) -- rowcount is
    cursor.rowcount floored at 0; pool_add_ids/pool_remove_ids are ids the
    caller should feed to the table's PkPool.add_many/remove_many, but only
    after the enclosing transaction actually commits (see run_transaction).

    Skips (returns (0, ([], []))) exactly like the legacy path does for a
    PK-less table on UPDATE/DELETE, or a table with no updateable columns
    on UPDATE.
    """
    schema = table_schemas[table_name]
    pool = pools.get(table_name) if pools else None

    if operation == "INSERT":
        pk_value_fn = pk_value_fn_for_table(table_name) if pk_value_fn_for_table else None
        unique_value_fns = (
            unique_value_fns_for_table(table_name) if unique_value_fns_for_table else None
        )
        # Any column still unsynthesizable is omitted from the column list
        # entirely (see get_insert_column_list) so its DEFAULT applies,
        # instead of an explicit NULL that would violate a NOT NULL
        # constraint -- same fix as generator.py's legacy INSERT branch.
        columns = ", ".join(quote_ident(c) for c in get_insert_column_list(
            table_schemas, table_name, column_overrides, unique_value_fns, pk_value_fn,
        ))
        values_list, pk_values = build_insert_values(
            table_schemas,
            table_name,
            1,
            min_col_size_bytes,
            column_overrides,
            pk_value_fn=pk_value_fn,
            unique_value_fns=unique_value_fns,
        )
        cursor.execute(f"INSERT INTO {quote_ident(table_name)} ({columns}) VALUES {values_list}")
        rowcount = max(cursor.rowcount or 0, 0)
        add_ids = [pk for pk in pk_values if pk is not None]
        return rowcount, (add_ids, [])

    if operation == "UPDATE":
        primary_key = schema["primary_key"]
        if not primary_key:
            return 0, ([], [])

        pk_set = set(primary_key) if isinstance(primary_key, list) else {primary_key}
        columns = schema["columns"]
        no_update = pk_set | set(schema.get("unique_columns", []))
        updateable_columns = [c for c in columns if c not in no_update]
        if not updateable_columns:
            return 0, ([], [])

        # Matches generator.py's legacy UPDATE branch, which also draws
        # from the shared (seeded) `random` module directly for this pick
        # rather than the plan-building rng.
        num_columns_to_update = _random_module.randint(1, len(updateable_columns))
        columns_to_update = _random_module.sample(updateable_columns, num_columns_to_update)
        set_clause, params = build_update_values(
            table_schemas, table_name, columns_to_update, min_col_size_bytes, column_overrides
        )

        rmin, rmax = rows_per_statement
        k = _random_module.randint(rmin, rmax)
        pool_ids = pool.sample(k) if pool is not None and len(pool) > 0 else []
        if pool_ids:
            where_clause, sampling_params = build_pk_in_condition(primary_key, pool_ids)
        else:
            where_clause, sampling_params = build_sampling_condition(
                db_flavor=db_flavor,
                table_name=table_name,
                primary_key=primary_key,
                target_row_count=k,
                estimated_row_count=(row_estimates or {}).get(table_name),
            )
        query = f"UPDATE {quote_ident(table_name)} SET {set_clause} WHERE {where_clause}"
        cursor.execute(query, params + sampling_params)
        rowcount = max(cursor.rowcount or 0, 0)
        return rowcount, ([], [])

    if operation == "DELETE":
        primary_key = schema["primary_key"]
        if not primary_key:
            return 0, ([], [])

        rmin, rmax = rows_per_statement
        k = _random_module.randint(rmin, rmax)
        pool_ids = pool.sample(k) if pool is not None and len(pool) > 0 else []
        if pool_ids:
            where_clause, sampling_params = build_pk_in_condition(primary_key, pool_ids)
        else:
            where_clause, sampling_params = build_sampling_condition(
                db_flavor=db_flavor,
                table_name=table_name,
                primary_key=primary_key,
                target_row_count=k,
                estimated_row_count=(row_estimates or {}).get(table_name),
            )
        query = f"DELETE FROM {quote_ident(table_name)} WHERE {where_clause}"
        cursor.execute(query, sampling_params)
        rowcount = max(cursor.rowcount or 0, 0)
        # remove_ids must be ALL k sampled ids (pool_ids, already a list) --
        # every one of them was just deleted.
        remove_ids = pool_ids
        return rowcount, ([], remove_ids)

    raise ValueError(f"Unknown operation: {operation!r}")


def run_transaction(
    conn: Any,
    cursor: Any,
    plan: Dict[str, Any],
    table_schemas: Dict[str, Any],
    pools: Optional[Dict[str, Any]],
    db_flavor: str,
    row_estimates: Optional[Dict[str, int]],
    column_overrides: Optional[Dict[str, Any]],
    min_col_size_bytes: int,
    pk_value_fn_for_table: Optional[Callable[[str], Optional[Callable[[], Any]]]] = None,
    unique_value_fns_for_table: Optional[
        Callable[[str], Optional[Dict[str, Callable[[], Any]]]]
    ] = None,
    rows_per_statement: Tuple[int, int] = (1, 1),
) -> int:
    """Execute one full transaction from `plan` (see build_transaction_plan)
    against a live cursor/connection: explicit BEGIN, each planned
    statement in order (opening/closing any configured savepoint ranges
    around it), then conn.commit().

    `rows_per_statement` is threaded straight through to every statement's
    execute_single_statement call (see there -- it only affects UPDATE/
    DELETE; INSERT stays single-row regardless).

    Returns the SUM of each statement's rowcount (reached only once
    conn.commit() has returned without raising) -- i.e. the number of rows
    actually changed across the whole transaction, not the number of
    statements. Each committed ROW counts as one event for the rate
    governor, matching generator.py's `GOVERNOR.pace(events_emitted)` call
    (which is likewise rows, not statements, on the legacy single-op path).
    Under the default rows_per_statement of (1, 1) every statement is
    single-row, so this sum equals len(plan["statements"]) exactly --
    unchanged from the pre-existing behavior.

    Raises on any error (BEGIN/SAVEPOINT/a statement/RELEASE
    SAVEPOINT/commit) -- see module docstring: no rollback happens here,
    by design, so the caller's existing exception handler (generator.py's
    per-iteration `except psycopg2.Error`) is the single place that deals
    with rollback/reconnect, exactly as it already does for the legacy
    single-op path.
    """
    statements = plan["statements"]
    savepoint_ranges = plan["savepoint_ranges"]

    # index -> [("open"|"close", savepoint_name), ...], built once so the
    # main loop below doesn't have to search savepoint_ranges per statement.
    boundary_at: Dict[int, List[Tuple[str, str]]] = {}
    for k, (start, end) in enumerate(savepoint_ranges, start=1):
        name = f"sp_{k}"
        boundary_at.setdefault(start, []).append(("open", name))
        boundary_at.setdefault(end, []).append(("close", name))

    cursor.execute("BEGIN")

    # PK pool bookkeeping is deferred until AFTER conn.commit() succeeds
    # below (mirrors the legacy path's pool.add_many/remove_many placement
    # right after each conn.commit()) -- a rolled-back id must never be
    # marked live/dead in the pool.
    pool_updates: List[Tuple[str, List[Any], List[Any]]] = []
    total_rows = 0

    for idx, stmt in enumerate(statements):
        for kind, name in boundary_at.get(idx, ()):
            if kind == "open":
                cursor.execute(f"SAVEPOINT {name}")

        rowcount, (add_ids, remove_ids) = execute_single_statement(
            cursor,
            table_schemas,
            pools,
            db_flavor,
            row_estimates,
            column_overrides,
            min_col_size_bytes,
            pk_value_fn_for_table,
            unique_value_fns_for_table,
            stmt["table"],
            stmt["operation"],
            rows_per_statement,
        )
        total_rows += rowcount
        if add_ids or remove_ids:
            pool_updates.append((stmt["table"], add_ids, remove_ids))

        for kind, name in boundary_at.get(idx, ()):
            if kind == "close":
                cursor.execute(f"RELEASE SAVEPOINT {name}")

    conn.commit()

    if pools:
        for table_name, add_ids, remove_ids in pool_updates:
            pool = pools.get(table_name)
            if pool is None:
                continue
            if add_ids:
                pool.add_many(add_ids)
            if remove_ids:
                pool.remove_many(remove_ids)

    return total_rows
