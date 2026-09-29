#!/usr/bin/env python3
"""Evaluate ArenaLM on a separately maintained intent set."""

from __future__ import annotations

import argparse
import json
from collections import Counter
from pathlib import Path

try:
    from .arena_model.model import SUPPORTED_INTENTS, TorchIntentModel
    from .train import read_examples
except ImportError:
    from arena_model.model import SUPPORTED_INTENTS, TorchIntentModel
    from train import read_examples


def evaluate(model: TorchIntentModel, examples: list[tuple[str, str]]) -> dict:
    confusion: Counter[tuple[str, str]] = Counter()
    confidence_sum = 0.0
    for text, expected in examples:
        prediction = model.predict(text)
        confusion[(expected, prediction.intent)] += 1
        confidence_sum += prediction.confidence
    correct = sum(confusion[(intent, intent)] for intent in SUPPORTED_INTENTS)
    f1_values = []
    per_intent = {}
    for intent in SUPPORTED_INTENTS:
        true_positive = confusion[(intent, intent)]
        false_positive = sum(confusion[(other, intent)] for other in SUPPORTED_INTENTS if other != intent)
        false_negative = sum(confusion[(intent, other)] for other in SUPPORTED_INTENTS if other != intent)
        precision = true_positive / (true_positive + false_positive) if true_positive + false_positive else 0.0
        recall = true_positive / (true_positive + false_negative) if true_positive + false_negative else 0.0
        f1 = 2 * precision * recall / (precision + recall) if precision + recall else 0.0
        f1_values.append(f1)
        per_intent[intent] = {"precision": round(precision, 4), "recall": round(recall, 4), "f1": round(f1, 4)}
    return {
        "examples": len(examples),
        "accuracy": round(correct / len(examples), 4) if examples else 0.0,
        "macroF1": round(sum(f1_values) / len(f1_values), 4),
        "meanConfidence": round(confidence_sum / len(examples), 4) if examples else 0.0,
        "perIntent": per_intent,
        "errors": [
            {"expected": expected, "predicted": predicted, "count": count}
            for (expected, predicted), count in sorted(confusion.items())
            if expected != predicted
        ],
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model", type=Path, default=Path("ml/model/arena-intents-v2.pt"))
    parser.add_argument("--data", type=Path, default=Path("ml/data/eval-intents.jsonl"))
    parser.add_argument("--minimum-accuracy", type=float, default=0.0)
    args = parser.parse_args()
    report = evaluate(TorchIntentModel.load(args.model), read_examples([args.data]))
    print(json.dumps(report, ensure_ascii=False, indent=2))
    if report["accuracy"] < args.minimum_accuracy:
        raise SystemExit(f"accuracy {report['accuracy']:.4f} is below {args.minimum_accuracy:.4f}")


if __name__ == "__main__":
    main()
