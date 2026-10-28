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
        self.assertEqual(extract_cost(requests), [20.0, 30.0, 1.5])

    def test_multiple_requests_some_not_informed(self):
        # cached_tokens averages only over the requests that report it, so
        # partial information is kept instead of dropping the whole position.
        requests = [make_request(10, 20, cached=1), make_request(30, 40)]
        self.assertEqual(extract_cost(requests), [20.0, 30.0, 1.0])

    def test_multiple_requests_partial_cached_mean_over_reported(self):
        requests = [
            make_request(10, 20, cached=2),
            make_request(30, 40),
            make_request(50, 60, cached=4),
        ]
        self.assertEqual(extract_cost(requests), [30.0, 40.0, 3.0])

    def test_multiple_requests_no_usage_returns_empty(self):
        requests = [SimpleNamespace(resp=SimpleNamespace())]
        self.assertEqual(extract_cost(requests), [])


class ScalarStatsAccumulatorTest(unittest.TestCase):
    def test_empty_defaults(self):
        acc = ScalarStatsAccumulator()
        self.assertEqual(acc.mean(), 0.0)
        self.assertEqual(acc.median(), 0.0)
        self.assertEqual(acc.se(), 0.0)
        self.assertEqual(acc.v_eff(), 0.0)
        self.assertEqual(acc.n(), 0)

    def test_single_group(self):
        acc = ScalarStatsAccumulator()
        acc.add(mean=1.0, median=1.0, se=0.5, n=4, df=3)
        self.assertEqual(acc.mean(), 1.0)
        self.assertEqual(acc.median(), 1.0)
        self.assertAlmostEqual(acc.se(), 0.5)  # sqrt(se^2) / K = 0.5 / 1
        self.assertAlmostEqual(acc.v_eff(), 3.0)  # single group -> n - 1
        self.assertEqual(acc.n(), 4)

    def test_mean_and_median_are_averages(self):
        acc = ScalarStatsAccumulator()
        acc.add(mean=1.0, median=2.0, se=1.0, n=4, df=3)
        acc.add(mean=3.0, median=4.0, se=1.0, n=4, df=3)
        self.assertEqual(acc.mean(), 2.0)
        self.assertEqual(acc.median(), 3.0)

    def test_se_is_standard_error_of_the_mean(self):
        acc = ScalarStatsAccumulator()
        acc.add(mean=1.0, median=1.0, se=1.0, n=4, df=3)
        acc.add(mean=3.0, median=3.0, se=1.0, n=4, df=3)
        # V_total = 2, K = 2 -> sqrt(2) / 2
        self.assertAlmostEqual(acc.se(), sqrt(2.0) / 2.0)
        # Welch-Satterthwaite: 2^2 / (1/3 + 1/3) = 6
        self.assertAlmostEqual(acc.v_eff(), 6.0)

    def test_welch_excludes_non_positive_df(self):
        acc = ScalarStatsAccumulator()
        # n=1 group has se=0 and no degrees of freedom: it counts for the mean
        # but not for the Welch-Satterthwaite denominator.
        acc.add(mean=1.0, median=1.0, se=0.0, n=1, df=0)
        acc.add(mean=1.0, median=1.0, se=1.0, n=4, df=3)
        self.assertAlmostEqual(acc.se(), 1.0 / 2.0)
        self.assertAlmostEqual(acc.v_eff(), 3.0)

    def test_n_is_min_coverage(self):
        acc = ScalarStatsAccumulator()
        acc.add(mean=1.0, median=1.0, se=1.0, n=10, df=9)
        acc.add(mean=1.0, median=1.0, se=1.0, n=5, df=4)
        self.assertEqual(acc.n(), 5)


class VectorStatsAccumulatorTest(unittest.TestCase):
    def test_empty(self):
        acc = VectorStatsAccumulator()
        self.assertEqual(acc.mean(), [])
        self.assertEqual(acc.median(), [])
        self.assertEqual(acc.se(), [])
        self.assertEqual(acc.v_eff(), [])

    def test_mean_and_median_skip_none(self):
        acc = VectorStatsAccumulator()
        acc.add([1, 2, None], [1, 2, None], [0, 0, None], [3, 3, 0])
        acc.add([4, 5, 6], [4, 5, 6], [0, 0, 0], [3, 3, 3])
        acc.add([7, 8, 9], [7, 8, 9], [0, 0, 0], [3, 3, 3])
        self.assertEqual(acc.mean(), [4.0, 5.0, 7.5])
        self.assertEqual(acc.median(), [4.0, 5.0, 7.5])

    def test_se_and_v_eff_per_position(self):
        acc = VectorStatsAccumulator()
        acc.add([1, 1], [1, 1], [1, 2], [3, 3])
        acc.add([3, 3], [3, 3], [1, 2], [3, 3])
        # pos0: V_total=2, K=2 -> sqrt(2)/2 ; v_eff=4/(1/3+1/3)=6
        # pos1: V_total=8, K=2 -> sqrt(8)/2 ; v_eff=64/(16/3+16/3)=6
        self.assertAlmostEqual(acc.se()[0], sqrt(2.0) / 2.0)
        self.assertAlmostEqual(acc.se()[1], sqrt(8.0) / 2.0)
        self.assertAlmostEqual(acc.v_eff()[0], 6.0)
        self.assertAlmostEqual(acc.v_eff()[1], 6.0)

    def test_appended_position_uses_only_samples_reporting_it(self):
        acc = VectorStatsAccumulator()
        acc.add([1, 2, 3], [1, 2, 3], [0, 0, 0], [3, 3, 3])
        acc.add([1, 2, 3, 10], [1, 2, 3, 10], [0, 0, 0, 0], [3, 3, 3, 3])
        self.assertEqual(acc.mean(), [1.0, 2.0, 3.0, 10.0])

    def test_all_none_position_is_none(self):
        acc = VectorStatsAccumulator()
        acc.add([None, 2], [None, 2], [None, 1], [0, 3])
        acc.add([None, 4], [None, 4], [None, 1], [0, 3])
        self.assertEqual(acc.mean(), [None, 3.0])
        self.assertEqual(acc.se()[0], None)
        self.assertEqual(acc.v_eff()[0], None)

    def test_v_eff_zero_when_no_degrees_of_freedom(self):
        acc = VectorStatsAccumulator()
        acc.add([1], [1], [0], [0])
        self.assertEqual(acc.mean(), [1.0])
        self.assertEqual(acc.se(), [0.0])
        self.assertEqual(acc.v_eff(), [0.0])


if __name__ == "__main__":
    unittest.main()
