"""Small PyTorch intent model used by Negotiation Arena."""

from __future__ import annotations

import hashlib
import math
import re
import unicodedata
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

import torch
from torch import nn


MODEL_FORMAT = "arena-pytorch-intent-v2"
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


def _feature_index(value: str, dimension: int) -> int:
    digest = hashlib.blake2b(value.encode("utf-8"), digest_size=8).digest()
    return int.from_bytes(digest, "little") % dimension


def vectorize(value: str, dimension: int) -> torch.Tensor:
    vector = torch.zeros(dimension, dtype=torch.float32)
    for feature, count in features(value).items():
        vector[_feature_index(feature, dimension)] += math.log1p(count)
    norm = torch.linalg.vector_norm(vector)
    if norm.item() > 0:
        vector /= norm
    return vector


class IntentNetwork(nn.Module):
    def __init__(self, feature_dimension: int, hidden_size: int, intent_count: int) -> None:
        super().__init__()
        self.layers = nn.Sequential(
            nn.Linear(feature_dimension, hidden_size),
            nn.ReLU(),
            nn.Dropout(0.1),
            nn.Linear(hidden_size, intent_count),
        )

    def forward(self, inputs: torch.Tensor) -> torch.Tensor:
        return self.layers(inputs)


class TorchIntentModel:
    def __init__(
        self,
        *,
        feature_dimension: int = 4096,
        hidden_size: int = 96,
        temperature: float = 2.5,
        version: str = "untrained",
    ) -> None:
        if feature_dimension < 128 or hidden_size < 8 or temperature <= 0:
            raise ValueError("invalid model hyperparameters")
        self.feature_dimension = feature_dimension
        self.hidden_size = hidden_size
        self.temperature = temperature
        self.version = version
        self.network = IntentNetwork(feature_dimension, hidden_size, len(SUPPORTED_INTENTS))
        self.network.eval()

    @property
    def parameter_count(self) -> int:
        return sum(parameter.numel() for parameter in self.network.parameters())

    def fit(
        self,
        examples: Iterable[tuple[str, str]],
        *,
        epochs: int = 200,
        learning_rate: float = 0.02,
        seed: int = 42,
    ) -> dict[str, float | int]:
        items = list(examples)
        if not items:
            raise ValueError("training set is empty")
        counts = Counter(intent for _, intent in items)
        unknown = set(counts) - set(SUPPORTED_INTENTS)
        if unknown:
            raise ValueError(f"unsupported intents: {', '.join(sorted(unknown))}")
        missing = set(SUPPORTED_INTENTS) - set(counts)
        if missing:
            raise ValueError(f"training set misses intents: {', '.join(sorted(missing))}")
        if epochs < 1 or learning_rate <= 0:
            raise ValueError("invalid training hyperparameters")

        torch.manual_seed(seed)
        self.network.apply(_reset_parameters)
        inputs = torch.stack([vectorize(text, self.feature_dimension) for text, _ in items])
        labels = torch.tensor([SUPPORTED_INTENTS.index(intent) for _, intent in items], dtype=torch.long)
        weights = torch.tensor(
            [math.sqrt(len(items) / (len(SUPPORTED_INTENTS) * counts[intent])) for intent in SUPPORTED_INTENTS],
            dtype=torch.float32,
        )
        self.network.train()
        optimizer = torch.optim.AdamW(self.network.parameters(), lr=learning_rate, weight_decay=1e-4)
        loss_function = nn.CrossEntropyLoss(weight=weights)
        final_loss = 0.0
        for _ in range(epochs):
            optimizer.zero_grad(set_to_none=True)
            logits = self.network(inputs)
            loss = loss_function(logits, labels)
            loss.backward()
            optimizer.step()
            final_loss = loss.item()
        self.network.eval()
        with torch.no_grad():
            accuracy = (self.network(inputs).argmax(dim=1) == labels).float().mean().item()
        return {"epochs": epochs, "loss": final_loss, "trainingAccuracy": accuracy}

    def predict(self, text: str) -> Prediction:
        normalized = normalize_text(text)
        if not normalized:
            return Prediction("neutral", 1.0, {intent: 0.0 for intent in SUPPORTED_INTENTS})
        inputs = vectorize(normalized, self.feature_dimension).unsqueeze(0)
        with torch.inference_mode():
            probabilities = torch.softmax(self.network(inputs) / self.temperature, dim=1).squeeze(0)
        index = int(probabilities.argmax().item())
        scores = {intent: float(probabilities[position].item()) for position, intent in enumerate(SUPPORTED_INTENTS)}
        return Prediction(SUPPORTED_INTENTS[index], scores[SUPPORTED_INTENTS[index]], scores)

    def save(self, path: Path) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        torch.save(
            {
                "format": MODEL_FORMAT,
                "version": self.version,
                "featureDimension": self.feature_dimension,
                "hiddenSize": self.hidden_size,
                "temperature": self.temperature,
                "intents": list(SUPPORTED_INTENTS),
                "stateDict": self.network.state_dict(),
            },
            path,
        )

    @classmethod
    def load(cls, path: Path) -> "TorchIntentModel":
        checkpoint = torch.load(path, map_location="cpu", weights_only=True)
        if checkpoint.get("format") != MODEL_FORMAT:
            raise ValueError("unsupported model format")
        if tuple(checkpoint.get("intents", ())) != SUPPORTED_INTENTS:
            raise ValueError("model intents do not match the service")
        model = cls(
            feature_dimension=int(checkpoint["featureDimension"]),
            hidden_size=int(checkpoint["hiddenSize"]),
            temperature=float(checkpoint["temperature"]),
            version=str(checkpoint["version"]),
        )
        model.network.load_state_dict(checkpoint["stateDict"])
        model.network.eval()
        return model


def _reset_parameters(module: nn.Module) -> None:
    reset = getattr(module, "reset_parameters", None)
    if callable(reset):
        reset()


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
