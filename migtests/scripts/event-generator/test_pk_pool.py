"""
Unit tests for pk_pool.py.

Uses stdlib unittest (not pytest), with a seeded random.Random() injected
where determinism must be verified.
"""

import random
import unittest

from pk_pool import PkPool


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


if __name__ == "__main__":
    unittest.main()
