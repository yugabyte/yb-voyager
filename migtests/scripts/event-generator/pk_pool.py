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
    not yet reflected in `base`. Bounded at `maxsize`, FIFO-evicted via a
    `collections.deque` for O(1) `popleft` eviction (no `list.pop(0)`,
    which is O(n) and degrades long-running workers).
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

        # Private delta: FIFO-ordered, bounded, deduplicated ids this pool
        # has itself added (e.g. via INSERT) that aren't yet in `base`.
        self._delta_deque = deque()
        self._delta_set = set()

        # Private tombstones: `base` ids this pool has "deleted". `base`
        # itself is never touched.
        self._tombstones = set()

    # ---- mutation ----

    def add_many(self, ids):
        """Add new ids to the private delta (deduplicated).

        FIFO-evicts the oldest delta ids once the delta exceeds `maxsize`,
        via `deque.popleft()` (O(1) per eviction).
        """
        for pk in ids:
            if pk in self._delta_set:
                continue
            self._delta_set.add(pk)
            self._delta_deque.append(pk)
        self._evict_excess()

    def _evict_excess(self):
        while len(self._delta_set) > self.maxsize and self._delta_deque:
            oldest = self._delta_deque.popleft()
            self._delta_set.discard(oldest)

    def remove_many(self, ids):
        """Mark ids as no-longer-live (e.g. after a DELETE).

        If an id is in the private delta, it's dropped from there. Else, if
        it's a `base` id, it's recorded in the private tombstone set (since
        `base` itself can't be edited). Ids that are neither are a no-op.
        """
        for pk in ids:
            if pk in self._delta_set:
                self._delta_set.discard(pk)
                continue
            if self.base is not None and pk in self.base:
                self._tombstones.add(pk)
        self._compact_delta_if_needed()

    def _live_delta_ordered(self):
        """Return live delta ids in FIFO (insertion) order, deduplicated.

        Building this from `_delta_deque` (rather than iterating the
        `_delta_set` directly) keeps sampling deterministic under a seeded
        rng and independent of set/hash iteration order.
        """
        seen = set()
        ordered = []
        for pk in self._delta_deque:
            if pk in self._delta_set and pk not in seen:
                seen.add(pk)
                ordered.append(pk)
        return ordered

    def _compact_delta_if_needed(self):
        """Rebuild `_delta_deque` to drop stale (removed) entries once they
        pile up, so `remove_many` on a long-running worker doesn't leak
        memory in the deque indefinitely (removal itself is O(1); this
        compaction is only occasional and amortized)."""
        live_count = len(self._delta_set)
        if len(self._delta_deque) > 2 * live_count + 16:
            self._delta_deque = deque(self._live_delta_ordered())

    # ---- sampling ----

    def sample(self, n):
        """Return up to `n` distinct live ids, chosen at random, from
        `(base - tombstones) UNION delta`.

        Returns [] if the pool is empty. If `n` is greater than or equal to
        the number of live ids, returns all of them (order randomized).
        """
        if n <= 0:
            return []

        delta_list = self._live_delta_ordered()

        if self.base is None:
            total = len(delta_list)
            if total == 0:
                return []
            if n >= total:
                self._rng.shuffle(delta_list)
                return delta_list
            return self._rng.sample(delta_list, n)

        base_live = max(0, len(self.base) - len(self._tombstones))
        total = base_live + len(delta_list)
        if total == 0:
            return []

        if n >= total:
            base_ids = self._sample_base(base_live)
            result = delta_list + base_ids
            self._rng.shuffle(result)
            return result

        # Proportional split between delta and base, so a small fresh
        # delta isn't starved (or doesn't dominate) relative to the shared
        # base.
        delta_take = 0
        if delta_list:
            delta_take = min(
                len(delta_list),
                int(round(n * len(delta_list) / float(total))),
            )
        base_take = n - delta_take
        if base_take > base_live:
            base_take = base_live
            delta_take = min(len(delta_list), n - base_take)

        sampled_delta = self._rng.sample(delta_list, delta_take) if delta_take else []
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
        return base_live + len(self._delta_set)
