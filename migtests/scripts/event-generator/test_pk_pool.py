"""
Unit tests for pk_pool.py.

Uses stdlib unittest (not pytest), with a seeded random.Random() injected
where determinism must be verified.
"""

import random
import unittest
from collections import deque

from pk_pool import PkPool
from shared_cache import PkBase


class FakeBase(object):
    """Minimal read-only base test double matching the shared_cache.PkBase
    contract (__len__, __contains__, sample(n, rng), is_composite), so
    PkPool's base-interaction logic can be tested in isolation from the
    real (mmap-backed) implementation."""

    def __init__(self, values, composite=False):
        self._values = list(values)
        self.is_composite = composite

    def __len__(self):
        return len(self._values)

    def __contains__(self, pk):
        return pk in self._values

    def sample(self, n, rng):
        if n <= 0 or not self._values:
            return []
        if n >= len(self._values):
            vals = list(self._values)
            rng.shuffle(vals)
            return vals
        return rng.sample(self._values, n)


class TestAddManyAndLen(unittest.TestCase):
    def test_add_many_increases_len(self):
        pool = PkPool()
        pool.add_many([1, 2, 3])
        self.assertEqual(len(pool), 3)

    def test_add_many_dedups(self):
        pool = PkPool()
        pool.add_many([1, 2, 2, 3])
        pool.add_many([3, 4])
        self.assertEqual(len(pool), 4)

    def test_empty_pool_len_is_zero(self):
        pool = PkPool()
        self.assertEqual(len(pool), 0)


class TestSample(unittest.TestCase):
    def test_sample_returns_only_live_ids(self):
        pool = PkPool()
        pool.add_many(range(100))
        sampled = pool.sample(10)
        self.assertEqual(len(sampled), 10)
        self.assertEqual(len(set(sampled)), 10)  # distinct
        for pk in sampled:
            self.assertIn(pk, range(100))

    def test_sample_at_most_n(self):
        pool = PkPool()
        pool.add_many([1, 2, 3])
        sampled = pool.sample(2)
        self.assertLessEqual(len(sampled), 2)

    def test_sample_more_than_pool_size_returns_all_live(self):
        pool = PkPool()
        pool.add_many([1, 2, 3])
        sampled = pool.sample(1000)
        self.assertEqual(sorted(sampled), [1, 2, 3])

    def test_sample_empty_pool_returns_empty_list(self):
        pool = PkPool()
        self.assertEqual(pool.sample(10), [])

    def test_sample_zero_or_negative_n_returns_empty_list(self):
        pool = PkPool()
        pool.add_many([1, 2, 3])
        self.assertEqual(pool.sample(0), [])
        self.assertEqual(pool.sample(-5), [])


class TestRemoveMany(unittest.TestCase):
    def test_remove_many_makes_ids_unsampleable(self):
        pool = PkPool()
        pool.add_many([1, 2, 3, 4, 5])
        pool.remove_many([2, 4])
        self.assertEqual(len(pool), 3)
        for _ in range(50):
            sampled = pool.sample(3)
            self.assertNotIn(2, sampled)
            self.assertNotIn(4, sampled)

    def test_remove_many_on_missing_ids_is_noop(self):
        pool = PkPool()
        pool.add_many([1, 2])
        pool.remove_many([999])  # never existed
        self.assertEqual(len(pool), 2)

    def test_remove_then_readd_is_sampleable_and_distinct(self):
        pool = PkPool()
        pool.add_many([1, 2, 3])
        pool.remove_many([2])
        pool.add_many([2])
        sampled = pool.sample(10)
        self.assertEqual(sorted(sampled), [1, 2, 3])
        self.assertEqual(len(sampled), len(set(sampled)))


class TestFifoEviction(unittest.TestCase):
    def test_evicts_oldest_beyond_maxsize(self):
        pool = PkPool(maxsize=5)
        pool.add_many(range(5))  # 0..4
        self.assertEqual(len(pool), 5)
        pool.add_many([5])  # should evict oldest (0)
        self.assertEqual(len(pool), 5)
        live = set(pool.sample(10))
        self.assertNotIn(0, live)
        self.assertIn(5, live)

    def test_eviction_keeps_pool_at_maxsize_after_bulk_add(self):
        pool = PkPool(maxsize=10)
        pool.add_many(range(100))
        self.assertEqual(len(pool), 10)
        live = set(pool.sample(100))
        # The 10 most recently added ids (90..99) should be the ones retained.
        self.assertEqual(live, set(range(90, 100)))


class TestDeterminism(unittest.TestCase):
    def _run_ops(self, pool):
        pool.add_many(range(50))
        pool.remove_many([3, 7, 11, 19])
        pool.add_many(range(50, 60))
        _ = pool.sample(5)  # consume some rng state, like real usage would
        pool.remove_many([25, 42])
        return pool.sample(8)

    def test_identical_seeded_rng_and_ops_yield_identical_samples(self):
        pool_a = PkPool(rng=random.Random(42))
        pool_b = PkPool(rng=random.Random(42))

        result_a = self._run_ops(pool_a)
        result_b = self._run_ops(pool_b)

        self.assertEqual(result_a, result_b)

    def test_full_pool_shuffle_is_deterministic_under_seed(self):
        pool_a = PkPool(rng=random.Random(7))
        pool_b = PkPool(rng=random.Random(7))

        pool_a.add_many([1, 2, 3, 4, 5])
        pool_b.add_many([1, 2, 3, 4, 5])

        self.assertEqual(pool_a.sample(1000), pool_b.sample(1000))


class TestDequeEviction(unittest.TestCase):
    def test_eviction_order_is_backed_by_a_deque(self):
        pool = PkPool()
        self.assertIsInstance(pool._order, deque)

    def test_o1_fifo_eviction_still_correct_at_scale(self):
        pool = PkPool(maxsize=100)
        pool.add_many(range(10000))
        self.assertEqual(len(pool), 100)
        live = set(pool.sample(1000))
        self.assertEqual(live, set(range(9900, 10000)))


class TestIndexedDeltaInternals(unittest.TestCase):
    def _assert_ids_index_consistent(self, pool):
        self.assertEqual(len(pool._ids), len(pool._index))
        self.assertEqual(len(pool._ids), len(set(pool._ids)))  # no dupes
        for pos, pk in enumerate(pool._ids):
            self.assertEqual(pool._index[pk], pos)
        for pk, pos in pool._index.items():
            self.assertEqual(pool._ids[pos], pk)

    def test_ids_index_consistent_after_interleaved_add_remove(self):
        pool = PkPool(maxsize=8)
        pool.add_many(range(5))  # 0..4
        self._assert_ids_index_consistent(pool)
        pool.remove_many([1, 3])  # swap-pop removals
        self._assert_ids_index_consistent(pool)
        pool.add_many([10, 11, 12])  # fills back up, still <= maxsize
        self._assert_ids_index_consistent(pool)
        pool.remove_many([0, 10])
        self._assert_ids_index_consistent(pool)
        pool.add_many([20, 21, 22, 23, 24])  # forces eviction
        self._assert_ids_index_consistent(pool)
        self.assertLessEqual(len(pool._ids), pool.maxsize)

    def test_eviction_skips_stale_order_entries_and_evicts_oldest_live(self):
        pool = PkPool(maxsize=3)
        pool.add_many([1, 2, 3])  # order: 1, 2, 3
        pool.remove_many([1])  # 1 removed from _ids/_index; _order still has it
        pool.add_many([4])  # 4 added; still <= maxsize (2,3,4), no eviction yet
        self._assert_ids_index_consistent(pool)
        self.assertEqual(set(pool._ids), {2, 3, 4})

        pool.add_many([5])  # over maxsize -> evict oldest *live* id (skip stale 1)
        self._assert_ids_index_consistent(pool)
        self.assertEqual(len(pool._ids), 3)
        # 2 was the oldest still-live id (1 was already removed, so it's
        # skipped as a stale _order entry rather than evicted again).
        self.assertNotIn(2, pool._ids)
        self.assertEqual(set(pool._ids), {3, 4, 5})


class TestBaseDeltaTombstoneSampling(unittest.TestCase):
    def test_sample_draws_from_base_and_delta_union(self):
        base = FakeBase(range(100, 110))  # 10 base ids
        pool = PkPool(base=base, rng=random.Random(1))
        pool.add_many([1, 2, 3])  # 3 delta ids
        self.assertEqual(len(pool), 13)
        sampled = set(pool.sample(13))
        self.assertEqual(sampled, set(range(100, 110)) | {1, 2, 3})

    def test_remove_many_on_base_id_tombstones_it_not_deleted_from_base(self):
        base = FakeBase([100, 101, 102])
        pool = PkPool(base=base, rng=random.Random(2))
        pool.remove_many([101])
        self.assertEqual(len(pool), 2)
        self.assertIn(101, base)  # base itself is untouched
        for _ in range(50):
            self.assertNotIn(101, pool.sample(2))

    def test_remove_many_on_delta_id_drops_it_from_delta(self):
        base = FakeBase([100, 101])
        pool = PkPool(base=base, rng=random.Random(3))
        pool.add_many([1, 2])
        pool.remove_many([1])
        self.assertEqual(len(pool), 3)  # 2 base + 1 delta (2)
        for _ in range(50):
            self.assertNotIn(1, pool.sample(3))

    def test_remove_many_prefers_delta_over_base_when_id_in_both(self):
        # Not expected under the monotonic seed-allocation scheme (base and
        # delta ranges are disjoint), but exercise the precedence rule
        # anyway: delta wins, so no tombstone is recorded for it.
        base = FakeBase([1, 2, 3])
        pool = PkPool(base=base, rng=random.Random(4))
        pool.add_many([1])
        pool.remove_many([1])
        self.assertNotIn(1, pool._tombstones)
        self.assertNotIn(1, pool._index)

    def test_remove_many_on_id_in_neither_is_noop(self):
        base = FakeBase([1, 2, 3])
        pool = PkPool(base=base, rng=random.Random(5))
        pool.remove_many([999])
        self.assertEqual(len(pool), 3)

    def test_len_reflects_base_minus_tombstones_plus_delta(self):
        base = FakeBase(range(1, 21))  # 20 ids
        pool = PkPool(base=base, rng=random.Random(6))
        self.assertEqual(len(pool), 20)
        pool.remove_many([1, 2, 3])
        self.assertEqual(len(pool), 17)
        pool.add_many([1000, 1001])
        self.assertEqual(len(pool), 19)

    def test_sample_all_when_n_exceeds_total(self):
        base = FakeBase([1, 2, 3])
        pool = PkPool(base=base, rng=random.Random(7))
        pool.add_many([4, 5])
        pool.remove_many([1])  # tombstoned -> excluded
        sampled = pool.sample(1000)
        self.assertEqual(sorted(sampled), [2, 3, 4, 5])

    def test_sample_excludes_tombstoned_base_ids_even_under_oversample_retry(self):
        # A base that's almost entirely tombstoned forces _sample_base's
        # retry loop to actually do work.
        base = FakeBase(range(1, 21))
        pool = PkPool(base=base, rng=random.Random(8))
        pool.remove_many(range(1, 19))  # tombstone all but 19, 20
        for _ in range(20):
            sampled = pool.sample(2)
            self.assertEqual(set(sampled), {19, 20})

    def test_base_none_behaves_as_plain_pool_backcompat(self):
        pool = PkPool(rng=random.Random(9))
        pool.add_many([1, 2, 3])
        self.assertEqual(len(pool), 3)
        pool.remove_many([2])  # no base -> plain drop, no tombstone recorded
        self.assertEqual(len(pool), 2)
        self.assertEqual(pool._tombstones, set())
        self.assertEqual(sorted(pool.sample(10)), [1, 3])

    def test_composite_ids_from_base_are_tuples(self):
        base = FakeBase([(1, "a"), (2, "b")], composite=True)
        pool = PkPool(base=base, rng=random.Random(10))
        sampled = pool.sample(10)
        self.assertEqual(sorted(sampled), [(1, "a"), (2, "b")])


class TestBaseDeltaWithRealSharedCachePkBase(unittest.TestCase):
    """Integration-flavored (but still pure/no-DB) check that a real
    shared_cache.PkBase plugs into PkPool exactly like the FakeBase test
    double above."""

    def test_real_pkbase_int_scalars(self):
        base = PkBase.from_values(list(range(50)), composite=False)
        pool = PkPool(base=base, rng=random.Random(11))
        pool.add_many([1000, 1001])
        pool.remove_many([0, 1])
        self.assertEqual(len(pool), 50)  # 48 base + 2 delta
        sampled = set(pool.sample(1000))
        self.assertNotIn(0, sampled)
        self.assertNotIn(1, sampled)
        self.assertIn(1000, sampled)

    def test_real_pkbase_empty(self):
        base = PkBase.empty()
        pool = PkPool(base=base, rng=random.Random(12))
        self.assertEqual(len(pool), 0)
        self.assertEqual(pool.sample(10), [])
        pool.add_many([1])
        self.assertEqual(pool.sample(10), [1])


if __name__ == "__main__":
    unittest.main()
