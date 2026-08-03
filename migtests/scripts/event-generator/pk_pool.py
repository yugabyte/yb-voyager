"""
PkPool: an in-memory pool of primary-key values known to currently exist in
a table.

Used by the event generator so UPDATE/DELETE can target explicit rows via
`WHERE pk IN (...)` -- indexed point lookups, O(rows touched) at any table
size, and exact row counts -- instead of asking the database to find random
rows to touch (e.g. `WHERE random() < p`), which is a full-table scan.

Structure (see ~/yb-ratetest/dynamic-worker-pool-design.md, sections 7-8,
and IMPLEMENTATION_CONTRACTS.md):

  - `base` (optional): a read-only, shared PK snapshot -- a
    `shared_cache.PkBase`, built once by the controller and mmap'd (for
    single-column integer PKs) or loaded (for composite/text PKs) read-only
    by every worker process. This pool never mutates `base`.
  - `delta` (private): this worker's own ids, inserted since it started and
    not yet reflected in `base`. Bounded at `maxsize`. Represented as a
    plain list (`_ids`) plus an id -> index map (`_index`) so add/remove
    are O(1) (via swap-pop) and sampling doesn't need to rebuild anything.
    FIFO eviction order is tracked separately in `_order`, a
    `collections.deque` that may contain stale (already-removed) ids --
    those are skipped, at eviction time, in amortized O(1).
  - `tombstones` (private): `base` ids this pool has "deleted". Since
    `base` is read-only and shared, a delete of a base id can't remove it
    from `base` -- it's recorded here instead and excluded from sampling.

`sample(n)` draws from `(base - tombstones) UNION delta`.

If `base` is None, this behaves as a plain bounded pool (delta only) --
back-compat for legacy/no-cache mode, where a worker self-seeds by querying
the table directly (see `utils.seed_pk_pool`).

Pure stdlib; no third-party dependencies.
"""

import random
from collections import deque


class PkPool(object):
    """Bounded pool of live primary-key values: shared read-only base plus
    a private, mutable delta and tombstone set.

    `base` is never mutated by this class. All mutation (`add_many`,
    `remove_many`) is confined to this pool's own private `delta` and
    `tombstones`, so many `PkPool` instances (one per worker) can safely
    share the same `base` object/mmap concurrently with no locking.
    """

    def __init__(self, base=None, maxsize=20000, rng=None):
        self.base = base
        self.maxsize = maxsize
        self._rng = rng if rng is not None else random

        # Private delta: bounded, deduplicated ids this pool has itself
        # added (e.g. via INSERT) that aren't yet in `base`.
        #
        # `_ids` holds the live delta ids (order is whatever swap-pop
        # removal leaves it in -- not meaningful FIFO order). `_index`
        # maps id -> its position in `_ids`, so membership checks and
        # removal are O(1). `_order` is a FIFO queue of ids used only to
        # decide which id to evict next; it may contain ids already
        # removed from `_index`/`_ids` (stale), which are skipped when
        # popped.
        self._ids = []
        self._index = {}
        self._order = deque()

        # Private tombstones: `base` ids this pool has "deleted". `base`
        # itself is never touched.
        self._tombstones = set()

    # ---- mutation ----

    def add_many(self, ids):
        """Add new ids to the private delta (deduplicated).

        FIFO-evicts the oldest still-live delta ids once the delta exceeds
        `maxsize`. Adds are O(1) each (amortized); eviction skips stale
        `_order` entries in amortized O(1) per eviction.
        """
        for pk in ids:
            if pk in self._index:
                continue
            self._ids.append(pk)
            self._index[pk] = len(self._ids) - 1
            self._order.append(pk)
        self._evict_excess()

    def _evict_excess(self):
        while len(self._ids) > self.maxsize:
            while self._order:
                oldest = self._order.popleft()
                if oldest in self._index:
                    self._remove_at(self._index[oldest])
                    break
            else:
                # `_order` exhausted but `_ids` still over maxsize --
                # shouldn't happen since every live id is pushed onto
                # `_order` exactly once, but guard against infinite loop.
                break

    def remove_many(self, ids):
        """Mark ids as no-longer-live (e.g. after a DELETE).

        If an id is in the private delta, it's dropped from there (O(1)
        swap-pop). Else, if it's a `base` id, it's recorded in the private
        tombstone set (since `base` itself can't be edited). Ids that are
        neither are a no-op. `_order` is left untouched -- the stale entry
        it still holds for this id is simply skipped later at eviction
        time.
        """
        for pk in ids:
            pos = self._index.get(pk)
            if pos is not None:
                self._remove_at(pos)
                continue
            if self.base is not None and pk in self.base:
                self._tombstones.add(pk)

    def _remove_at(self, pos):
        """Remove the id at `_ids[pos]` in O(1) via swap-pop: move the last
        element into `pos` (unless it's already the last), then pop."""
        pk = self._ids[pos]
        last = len(self._ids) - 1
        if pos != last:
            moved = self._ids[last]
            self._ids[pos] = moved
            self._index[moved] = pos
        self._ids.pop()
        del self._index[pk]

    # ---- sampling ----

    def sample(self, n):
        """Return up to `n` distinct live ids, chosen at random, from
        `(base - tombstones) UNION delta`.

        Returns [] if the pool is empty. If `n` is greater than or equal to
        the number of live ids, returns all of them (order randomized).
        """
        if n <= 0:
            return []

        if self.base is None:
            total = len(self._ids)
            if total == 0:
                return []
            if n >= total:
                delta_list = list(self._ids)
                self._rng.shuffle(delta_list)
                return delta_list
            return self._rng.sample(self._ids, n)

        base_live = max(0, len(self.base) - len(self._tombstones))
        total = base_live + len(self._ids)
        if total == 0:
            return []

        if n >= total:
            base_ids = self._sample_base(base_live)
            result = list(self._ids) + base_ids
            self._rng.shuffle(result)
            return result

        # Proportional split between delta and base, so a small fresh
        # delta isn't starved (or doesn't dominate) relative to the shared
        # base.
        delta_take = 0
        if self._ids:
            delta_take = min(
                len(self._ids),
                int(round(n * len(self._ids) / float(total))),
            )
        base_take = n - delta_take
        if base_take > base_live:
            base_take = base_live
            delta_take = min(len(self._ids), n - base_take)

        sampled_delta = self._rng.sample(self._ids, delta_take) if delta_take else []
        sampled_base = self._sample_base(base_take)
        result = sampled_delta + sampled_base
        self._rng.shuffle(result)
        return result

    def _sample_base(self, k):
        """Sample up to `k` ids from `base`, excluding tombstoned ones.

        `base.sample()` doesn't know about this pool's private tombstones,
        so over-sample and post-filter, retrying a bounded number of times
        if too many candidates land on tombstoned ids.
        """
        if self.base is None or k <= 0:
            return []

        base_len = len(self.base)
        if base_len <= 0:
            return []

        got = []
        seen = set()
        attempts = 0
        max_attempts = 6
        while len(got) < k and attempts < max_attempts:
            remaining = k - len(got)
            request = min(base_len, remaining * (attempts + 2) + 8)
            candidates = self.base.sample(request, self._rng)
            attempts += 1
            for pk in candidates:
                if len(got) >= k:
                    break
                if pk in seen or pk in self._tombstones:
                    continue
                seen.add(pk)
                got.append(pk)
            if request >= base_len:
                # Asked for (essentially) everything available; retrying
                # won't turn up more distinct ids than the base actually
                # has.
                break
        return got

    def __len__(self):
        """Approximate count of live ids (base minus tombstones, plus
        delta). Approximate because a pathological caller could add an id
        via `add_many` that happens to collide with a `base` id (not
        expected under the monotonic seed-allocation scheme this pool is
        designed for), which would double count it here."""
        base_live = 0
        if self.base is not None:
            base_live = max(0, len(self.base) - len(self._tombstones))
        return base_live + len(self._ids)
