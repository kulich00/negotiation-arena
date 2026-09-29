import unittest
from pathlib import Path

import torch

from ml.arena_model.model import TorchIntentModel, extract_proposal_value
from ml.server import ModelApplication


ROOT = Path(__file__).resolve().parents[2]


class TorchIntentModelTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.model = TorchIntentModel.load(ROOT / "ml/model/arena-intents-v2.pt")

    def test_uses_pytorch_network(self) -> None:
        self.assertIsInstance(self.model.network, torch.nn.Module)
        self.assertGreater(self.model.parameter_count, 100_000)

    def test_classifies_unseen_phrases(self) -> None:
        tests = {
            "Какие параметры выбора для вас принципиальны?": "ask_interest",
            "Без соглашения обратимся к конкурентам": "state_batna",
            "Подписывайте сейчас, иначе я ухожу": "pressure",
            "Я принимаю этот вариант": "accept",
        }
        for text, expected in tests.items():
            with self.subTest(text=text):
                self.assertEqual(expected, self.model.predict(text).intent)

    def test_model_round_trip(self) -> None:
        path = ROOT / "ml/reports/test-model-roundtrip.pt"
        path.parent.mkdir(parents=True, exist_ok=True)
        try:
            self.model.save(path)
            restored = TorchIntentModel.load(path)
        finally:
            path.unlink(missing_ok=True)
        text = "Предлагаю скидку 7 процентов"
        self.assertEqual(self.model.predict(text).intent, restored.predict(text).intent)

    def test_application_rejects_accept_without_offer(self) -> None:
        result = ModelApplication(self.model, minimum_confidence=0).interpret(
            {"message": "Я согласен", "offerMade": False}
        )
        self.assertEqual("neutral", result["intent"])
        self.assertFalse(result["relevant"])

    def test_extracts_bounded_proposal(self) -> None:
        self.assertEqual(7, extract_proposal_value("Предлагаю повышение на 7 процентов", 30))
        self.assertEqual(0, extract_proposal_value("Предлагаю повышение на 70 процентов", 30))

    def test_application_rejects_invalid_types(self) -> None:
        application = ModelApplication(self.model)
        with self.assertRaisesRegex(ValueError, "message"):
            application.interpret({"message": {"text": "hello"}})
        with self.assertRaisesRegex(ValueError, "proposalMaximum"):
            application.interpret({"message": "Предлагаю вариант", "proposalMaximum": "30"})


if __name__ == "__main__":
    unittest.main()
