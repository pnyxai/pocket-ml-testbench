import sys
import unittest
from pathlib import Path
from types import SimpleNamespace

REPO_ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(REPO_ROOT))
sys.path.insert(0, str(REPO_ROOT / "apps" / "python" / "sampler"))

from packages.python.lmeh.pocket_lm_eval.models.pocket_network import (
    get_result,
)


def make_response(token_logprobs, top_logprobs):
    return SimpleNamespace(
        logprobs=SimpleNamespace(
            token_logprobs=token_logprobs,
            top_logprobs=top_logprobs,
        )
    )


class GetResultTest(unittest.TestCase):
    def test_null_top_logprobs_keeps_sample(self):
        result, is_greedy = get_result(
            make_response([-0.1, -0.2, -0.3], None), ctxlen=1
        )
        self.assertAlmostEqual(result, -0.5)
        self.assertFalse(is_greedy)

    def test_empty_top_logprobs(self):
        result, is_greedy = get_result(make_response([-0.1, -0.2], []), ctxlen=0)
        self.assertAlmostEqual(result, -0.3)
        self.assertFalse(is_greedy)

    def test_short_top_logprobs(self):
        result, is_greedy = get_result(
            make_response([-0.1, -0.2, -0.3], [{"a": 0.0}]), ctxlen=1
        )
        self.assertAlmostEqual(result, -0.5)
        self.assertFalse(is_greedy)

    def test_none_entry_in_top_logprobs(self):
        result, is_greedy = get_result(
            make_response([-0.1, -0.2], [None, None]), ctxlen=0
        )
        self.assertAlmostEqual(result, -0.3)
        self.assertFalse(is_greedy)

    def test_no_tokens_left_to_check_stays_greedy(self):
        result, is_greedy = get_result(make_response([-0.1, -0.2], []), ctxlen=2)
        self.assertAlmostEqual(result, 0.0)
        self.assertTrue(is_greedy)


if __name__ == "__main__":
    unittest.main()
