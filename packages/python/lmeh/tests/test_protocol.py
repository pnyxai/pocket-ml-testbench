import unittest

from openai.types.chat.chat_completion_message import Annotation

from packages.python.protocol.protocol import ChatMessage, CompletionLogProbs

_ANNOTATION = {
    "type": "url_citation",
    "url_citation": {
        "url": "https://example.com",
        "title": "Example",
        "start_index": 0,
        "end_index": 10,
    },
}


class CompletionLogProbsTest(unittest.TestCase):
    def test_top_logprobs_accepts_null(self):
        logprobs = CompletionLogProbs(
            text_offset=[],
            token_logprobs=[-0.1],
            tokens=["a"],
            top_logprobs=None,
        )
        self.assertIsNone(logprobs.top_logprobs)

    def test_top_logprobs_defaults_to_none(self):
        self.assertIsNone(CompletionLogProbs().top_logprobs)

    def test_top_logprobs_accepts_list(self):
        logprobs = CompletionLogProbs(top_logprobs=[{"a": -0.1}, None])
        self.assertEqual(logprobs.top_logprobs, [{"a": -0.1}, None])


class ChatMessageAnnotationsTest(unittest.TestCase):
    def test_annotations_accepts_none(self):
        message = ChatMessage(role="assistant", annotations=None)
        self.assertIsNone(message.annotations)

    def test_annotations_defaults_to_none(self):
        message = ChatMessage(role="assistant")
        self.assertIsNone(message.annotations)

    def test_annotations_accepts_empty_list(self):
        message = ChatMessage(role="assistant", annotations=[])
        self.assertEqual(message.annotations, [])

    def test_annotations_accepts_single_dict(self):
        message = ChatMessage(role="assistant", annotations=_ANNOTATION)
        self.assertEqual(len(message.annotations), 1)
        self.assertEqual(message.annotations[0].type, "url_citation")

    def test_annotations_accepts_single_model(self):
        annotation = Annotation.model_validate(_ANNOTATION)
        message = ChatMessage(role="assistant", annotations=annotation)
        self.assertEqual(message.annotations, [annotation])

    def test_annotations_accepts_list(self):
        message = ChatMessage(role="assistant", annotations=[_ANNOTATION])
        self.assertEqual(len(message.annotations), 1)
        self.assertEqual(message.annotations[0].url_citation.url, "https://example.com")

    def test_annotations_accepts_tuple(self):
        message = ChatMessage(role="assistant", annotations=(_ANNOTATION,))
        self.assertEqual(len(message.annotations), 1)

    def test_single_annotation_serializes_as_list(self):
        message = ChatMessage(role="assistant", annotations=_ANNOTATION)
        dumped = message.model_dump()
        self.assertIsInstance(dumped["annotations"], list)
        self.assertEqual(dumped["annotations"][0]["type"], "url_citation")


if __name__ == "__main__":
    unittest.main()
