import unittest
from math import sqrt
from types import SimpleNamespace

from packages.python.lmeh.utils.cost_stats import (
    ScalarStatsAccumulator,
    VectorStatsAccumulator,
    extract_cost,
)

_MISSING = object()


def make_request(prompt, completion, cached=_MISSING):
    usage = SimpleNamespace(prompt_tokens=prompt, completion_tokens=completion)
    if cached is not _MISSING:
        usage.prompt_tokens_details = SimpleNamespace(cached_tokens=cached)
    return SimpleNamespace(resp=SimpleNamespace(usage=usage))


class ExtractCostTest(unittest.TestCase):
    def test_no_usage_returns_empty(self):
        request = SimpleNamespace(resp=SimpleNamespace())
        self.assertEqual(extract_cost([request]), [])

    def test_cached_informed(self):
        request = make_request(10, 20, cached=5)
        self.assertEqual(extract_cost([request]), [10.0, 20.0, 5.0])

    def test_cached_not_informed_is_none(self):
        request = make_request(10, 20)
        self.assertEqual(extract_cost([request]), [10.0, 20.0, None])

    def test_multiple_requests_all_informed(self):
        requests = [make_request(10, 20, cached=1), make_request(30, 40, cached=2)]
        self.assertEqual(extract_cost(requests), [40.0, 60.0, 3.0])

    def test_multiple_requests_some_not_informed(self):
        requests = [make_request(10, 20, cached=1), make_request(30, 40)]
        self.assertEqual(extract_cost(requests), [40.0, 60.0, None])

    def test_multiple_requests_no_usage_returns_empty(self):
        requests = [SimpleNamespace(resp=SimpleNamespace())]
        self.assertEqual(extract_cost(requests), [])


class VectorStatsAccumulatorTest(unittest.TestCase):
    def test_mean_skips_none_per_position(self):
        # Example from the specification.
        acc = VectorStatsAccumulator()
        acc.add_mean([1, 2, None])
        acc.add_mean([4, 5, 6])
        acc.add_mean([7, 8, 9])
        self.assertEqual(acc.mean(), [4.0, 5.0, 7.5])

    def test_all_none_position_is_none(self):
        acc = VectorStatsAccumulator()
        acc.add_mean([None, 2])
        acc.add_mean([None, 4])
        self.assertEqual(acc.mean(), [None, 3.0])

    def test_empty_is_empty(self):
        acc = VectorStatsAccumulator()
        self.assertEqual(acc.mean(), [])
        self.assertEqual(acc.median(), [])
        self.assertEqual(acc.std(), [])
        self.assertEqual(acc.n(), [])

    def test_appended_position_uses_only_samples_reporting_it(self):
        # A future feature appends a 4th position; only some samples report it.
        acc = VectorStatsAccumulator()
        acc.add_mean([1, 2, 3])
        acc.add_mean([1, 2, 3])
        acc.add_mean([1, 2, 3, 10])
        acc.add_mean([1, 2, 3, 20])
        self.assertEqual(acc.mean(), [1.0, 2.0, 3.0, 15.0])

    def test_median_is_mean_of_medians(self):
        acc = VectorStatsAccumulator()
        acc.add_median([2.0, 4.0])
        acc.add_median([4.0, None])
        self.assertEqual(acc.median(), [3.0, 4.0])

    def test_sem_uses_per_position_n(self):
        acc = VectorStatsAccumulator()
        acc.add_sem([2.0, 6.0], [4, 9])  # (2/2)^2=1 ; (6/3)^2=4
        acc.add_sem([3.0, None], [1, 0])  # (3/1)^2=9
        self.assertAlmostEqual(acc.std()[0], sqrt(10.0))
        self.assertAlmostEqual(acc.std()[1], 2.0)

    def test_sem_accepts_scalar_n(self):
        acc = VectorStatsAccumulator()
        acc.add_sem([2.0, 3.0], 4)  # both positions use n=4
        self.assertAlmostEqual(acc.std()[0], 1.0)
        self.assertAlmostEqual(acc.std()[1], 1.5)

    def test_n_is_min_per_position(self):
        acc = VectorStatsAccumulator()
        acc.add_min([3, 5])
        acc.add_min([2, 9])
        acc.add_min([4, 1])
        self.assertEqual(acc.n(), [2, 1])


class ScalarStatsAccumulatorTest(unittest.TestCase):
    def test_empty_defaults(self):
        acc = ScalarStatsAccumulator()
        self.assertEqual(acc.mean(), 0.0)
        self.assertEqual(acc.median(), 0.0)
        self.assertEqual(acc.std(), 0.0)
        self.assertEqual(acc.n(), 0)

    def test_mean_median_sem_and_min(self):
        acc = ScalarStatsAccumulator()
        acc.add_mean(1.0)
        acc.add_mean(3.0)
        acc.add_median(2.0)
        acc.add_median(4.0)
        acc.add_sem(2.0, 4)  # (2/2)^2 = 1
        acc.add_sem(3.0, 9)  # (3/3)^2 = 1
        acc.add_min(10)
        acc.add_min(5)
        self.assertEqual(acc.mean(), 2.0)
        self.assertEqual(acc.median(), 3.0)
        self.assertAlmostEqual(acc.std(), sqrt(2.0))
        self.assertEqual(acc.n(), 5)


if __name__ == "__main__":
    unittest.main()
