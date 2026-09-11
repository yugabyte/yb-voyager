"""
PkPool: an in-memory pool of primary-key values known to currently exist in
a table.

Used by the event generator so UPDATE/DELETE can target explicit rows via
`WHERE pk IN (...)` -- indexed point lookups, O(rows touched) at any table
size, and exact row counts -- instead of asking the database to find random
rows to touch (e.g. `WHERE random() < p`), which is a full-table scan.

Pure stdlib; no third-party dependencies.
"""

import random


class PkPool(object):
    """Bounded, FIFO-evicting pool of live primary-key values.

    Maintains two structures:
      - `_order`: an ordered list of ids in the order they were added. This
        is the source of sampling/eviction order. It may contain stale
        entries (ids that were later removed via `remove_many`) since
        removal is lazy -- it only updates `_live`.
      - `_live`: a set of currently-live ids. This is the authoritative
        liveness record; `__len__` and `sample` are defined in terms of it.

    Stale entries in `_order` are compacted away opportunistically (when
    `_order` grows past roughly twice the live count) rather than on every
    removal, to keep `remove_many` cheap.
    """

    def __init__(self, maxsize=200000, rng=None):
        self.maxsize = maxsize
        self._rng = rng if rng is not None else random
        self._order = []
        self._live = set()

    def add_many(self, ids):
        """Add new live ids to the pool (deduplicated).

        FIFO-evicts the oldest ids once the number of live ids exceeds
        `maxsize`.
        """
        for pk in ids:
            if pk in self._live:
                continue
            self._live.add(pk)
            self._order.append(pk)
        self._evict_excess()

    def _evict_excess(self):
        while len(self._live) > self.maxsize and self._order:
            oldest = self._order.pop(0)
            self._live.discard(oldest)

    def remove_many(self, ids):
        """Mark ids as no-longer-live (e.g. after a DELETE)."""
        for pk in ids:
            self._live.discard(pk)
        self._compact_if_needed()

    def _live_ordered(self):
        """Return live ids in FIFO (insertion) order, deduplicated.

        Building this from `_order` (rather than iterating the `_live` set
        directly) keeps sampling deterministic under a seeded rng and
        independent of set/hash iteration order.
        """
        seen = set()
        ordered = []
        for pk in self._order:
            if pk in self._live and pk not in seen:
                seen.add(pk)
                ordered.append(pk)
        return ordered

    def _compact_if_needed(self):
        live_count = len(self._live)
        if len(self._order) > 2 * live_count + 16:
            self._order = self._live_ordered()

    def sample(self, n):
        """Return up to `n` distinct live ids, chosen at random.

        Returns [] if the pool is empty. If `n` is greater than or equal to
        the number of live ids, returns all of them (order randomized).
        """
        if n <= 0 or not self._live:
            return []
        self._compact_if_needed()
        pool = self._live_ordered()
        if n >= len(pool):
            self._rng.shuffle(pool)
            return pool
        return self._rng.sample(pool, n)

    def __len__(self):
        return len(self._live)
