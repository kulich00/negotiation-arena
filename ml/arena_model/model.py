"""Dependency-free character n-gram language model for intent classification."""

from __future__ import annotations

import json
import math
import re
import unicodedata
from collections import Counter, defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


MODEL_FORMAT = "arena-ngram-intent-v1"
SUPPORTED_INTENTS = (
    "neutral",
    "ask_interest",
    "ask_situation",
    "identify_problem",
    "explore_implication",
    "clarify_need_payoff",
    "present_evidence",
    "state_batna",
    "propose",
    "accept",
    "pressure",
)


@dataclass(frozen=True)
class Prediction:
    intent: str
    confidence: float
    scores: dict[str, float]


def normalize_text(value: str) -> str:
    value = unicodedata.normalize("NFKC", value).lower().replace("ё", "е")
    value = "".join(char if char.isalnum() else " " for char in value)
    return " ".join(value.split())


def features(value: str, minimum: int = 2, maximum: int = 5) -> Counter[str]:
    normalized = normalize_text(value)
    result: Counter[str] = Counter()
    for word in normalized.split():
        result[f"w:{word}"] += 2
        padded = f"^{word}$"
        for size in range(minimum, maximum + 1):
            for index in range(max(0, len(padded) - size + 1)):
                result[f"c{size}:{padded[index:index + size]}"] += 1
    return result


class NGramIntentModel:
    def __init__(
        self,
        *,
        alpha: float = 0.35,
        minimum_ngram: int = 2,
        maximum_ngram: int = 5,
        version: str = "untrained",
    ) -> None:
        if alpha <= 0 or minimum_ngram < 1 or maximum_ngram < minimum_ngram:
            raise ValueError("invalid model hyperparameters")
        self.alpha = alpha
        self.minimum_ngram = minimum_ngram
        self.maximum_ngram = maximum_ngram
        self.version = version
        self.document_counts: Counter[str] = Counter()
        self.feature_counts: dict[str, Counter[str]] = defaultdict(Counter)
        self.feature_totals: Counter[str] = Counter()
        self.vocabulary: set[str] = set()

    def fit(self, examples: Iterable[tuple[str, str]]) -> None:
        seen = 0
        for text, intent in examples:
            if intent not in SUPPORTED_INTENTS:
                raise ValueError(f"unsupported intent {intent!r}")
            item_features = features(text, self.minimum_ngram, self.maximum_ngram)
            if not item_features:
                continue
            seen += 1
            self.document_counts[intent] += 1
            self.feature_counts[intent].update(item_features)
            self.feature_totals[intent] += sum(item_features.values())
            self.vocabulary.update(item_features)
        if seen == 0:
            raise ValueError("training set is empty")
        missing = set(SUPPORTED_INTENTS) - set(self.document_counts)
        if missing:
            raise ValueError(f"training set misses intents: {', '.join(sorted(missing))}")

    def predict(self, text: str) -> Prediction:
        item_features = features(text, self.minimum_ngram, self.maximum_ngram)
        if not item_features:
            return Prediction("neutral", 1.0, {intent: 0.0 for intent in SUPPORTED_INTENTS})
        total_documents = sum(self.document_counts.values())
        vocabulary_size = max(1, len(self.vocabulary))
        log_scores: dict[str, float] = {}
        for intent in SUPPORTED_INTENTS:
            documents = self.document_counts[intent]
            score = math.log((documents + 1) / (total_documents + len(SUPPORTED_INTENTS)))
            denominator = self.feature_totals[intent] + self.alpha * vocabulary_size
            counts = self.feature_counts[intent]
            for feature, count in item_features.items():
                score += count * math.log((counts[feature] + self.alpha) / denominator)
            log_scores[intent] = score
        maximum = max(log_scores.values())
        probabilities = {intent: math.exp(score - maximum) for intent, score in log_scores.items()}
        probability_sum = sum(probabilities.values())
        probabilities = {intent: value / probability_sum for intent, value in probabilities.items()}
        intent = max(probabilities, key=probabilities.get)
        return Prediction(intent, probabilities[intent], probabilities)

    def to_dict(self) -> dict:
        return {
            "format": MODEL_FORMAT,
            "version": self.version,
            "alpha": self.alpha,
            "minimumNGram": self.minimum_ngram,
            "maximumNGram": self.maximum_ngram,
            "documentCounts": dict(sorted(self.document_counts.items())),
            "featureTotals": dict(sorted(self.feature_totals.items())),
            "featureCounts": {
                intent: dict(sorted(self.feature_counts[intent].items()))
                for intent in SUPPORTED_INTENTS
            },
        }

    @classmethod
    def from_dict(cls, data: dict) -> "NGramIntentModel":
        if data.get("format") != MODEL_FORMAT:
            raise ValueError("unsupported model format")
        model = cls(
            alpha=float(data["alpha"]),
            minimum_ngram=int(data["minimumNGram"]),
            maximum_ngram=int(data["maximumNGram"]),
            version=str(data["version"]),
        )
        model.document_counts.update({key: int(value) for key, value in data["documentCounts"].items()})
        model.feature_totals.update({key: int(value) for key, value in data["featureTotals"].items()})
        for intent, counts in data["featureCounts"].items():
            if intent not in SUPPORTED_INTENTS:
                raise ValueError(f"unsupported intent in model: {intent}")
            model.feature_counts[intent].update({key: int(value) for key, value in counts.items()})
            model.vocabulary.update(counts)
        missing = set(SUPPORTED_INTENTS) - set(model.document_counts)
        if missing:
            raise ValueError("model does not contain every supported intent")
        return model

    def save(self, path: Path) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(self.to_dict(), ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")

    @classmethod
    def load(cls, path: Path) -> "NGramIntentModel":
        return cls.from_dict(json.loads(path.read_text(encoding="utf-8")))


INTEGER_PATTERN = re.compile(r"(?<!\d)(\d{1,6})(?!\d)")


def extract_proposal_value(text: str, maximum: int) -> int:
    match = INTEGER_PATTERN.search(normalize_text(text))
    if not match:
        return 0
    value = int(match.group(1))
    return value if maximum <= 0 or value <= maximum else 0


def extract_alternative(text: str, alternatives: Iterable[str]) -> str:
    normalized = normalize_text(text).replace(" ", "_")
    for alternative in alternatives:
        candidate = str(alternative).strip().lower()
        if candidate and candidate in normalized:
            return candidate
    return ""
