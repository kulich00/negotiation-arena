#!/usr/bin/env python3
"""Train ArenaLM from the versioned dialogue corpus and bootstrap examples."""

from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path

try:
    from .arena_model.model import SUPPORTED_INTENTS, TorchIntentModel
except ImportError:
    from arena_model.model import SUPPORTED_INTENTS, TorchIntentModel


def read_examples(paths: list[Path]) -> list[tuple[str, str]]:
    examples: list[tuple[str, str]] = []
    seen: set[tuple[str, str]] = set()
    for path in paths:
        with path.open(encoding="utf-8") as source:
            for line_number, line in enumerate(source, start=1):
                if not line.strip():
                    continue
                item = json.loads(line)
                if "turns" in item:
                    if not item.get("quality", {}).get("complete", False):
                        continue
                    for turn in item["turns"]:
                        append_example(examples, seen, turn["player"]["text"], turn["analysis"]["intent"], path, line_number)
                else:
                    append_example(examples, seen, item.get("text"), item.get("intent"), path, line_number)
    return examples


def append_example(
    examples: list[tuple[str, str]], seen: set[tuple[str, str]], text: object, intent: object, path: Path, line_number: int
) -> None:
    text, intent = str(text or "").strip(), str(intent or "").strip()
    if not text or intent not in SUPPORTED_INTENTS:
        raise ValueError(f"invalid training example in {path}:{line_number}")
    key = (text.casefold(), intent)
    if key not in seen:
        seen.add(key)
        examples.append((text, intent))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", type=Path, action="append", default=[])
    parser.add_argument("--bootstrap", type=Path, default=Path("ml/data/bootstrap-intents.jsonl"))
    parser.add_argument("--output", type=Path, default=Path("ml/model/arena-intents-v2.pt"))
    parser.add_argument("--version", default="")
    parser.add_argument("--epochs", type=int, default=200)
    parser.add_argument("--seed", type=int, default=42)
    args = parser.parse_args()

    paths = [args.bootstrap, *args.corpus]
    examples = read_examples(paths)
    version = args.version or datetime.now(timezone.utc).strftime("arena-intents-%Y%m%d")
    model = TorchIntentModel(version=version)
    training = model.fit(examples, epochs=args.epochs, seed=args.seed)
    model.save(args.output)
    counts = {intent: sum(1 for _, label in examples if label == intent) for intent in SUPPORTED_INTENTS}
    print(json.dumps({
        "model": str(args.output),
        "version": version,
        "examples": len(examples),
        "parameters": model.parameter_count,
        "training": training,
        "intents": counts,
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
