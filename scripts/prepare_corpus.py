#!/usr/bin/env python3
"""Prepare deterministic train/validation/test JSONL splits for the arena corpus."""

from __future__ import annotations

import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path
from typing import Any, Iterable


def load_records(path: Path, include_incomplete: bool = False) -> tuple[list[dict[str, Any]], Counter[str]]:
    records: list[dict[str, Any]] = []
    skipped: Counter[str] = Counter()
    seen_ids: set[str] = set()
    with path.open("r", encoding="utf-8") as source:
        for line_number, raw_line in enumerate(source, start=1):
            line = raw_line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
            except json.JSONDecodeError as error:
                raise ValueError(f"invalid JSON on line {line_number}: {error}") from error
            validate_record(record, line_number)
            dialogue_id = record["dialogueId"]
            if dialogue_id in seen_ids:
                skipped["duplicate_dialogue_id"] += 1
                continue
            seen_ids.add(dialogue_id)
            if not record["turns"]:
                skipped["empty_dialogue"] += 1
                continue
            if not include_incomplete and not record["quality"]["complete"]:
                skipped["incomplete"] += 1
                continue
            records.append(record)
    return records, skipped


def validate_record(record: dict[str, Any], line_number: int) -> None:
    if record.get("schemaVersion") != "1.0":
        raise ValueError(f"unsupported schemaVersion on line {line_number}")
    if record.get("origin") not in {"runtime", "synthetic"}:
        raise ValueError(f"invalid origin on line {line_number}")
    if not isinstance(record.get("dialogueId"), str) or not record["dialogueId"].strip():
        raise ValueError(f"missing dialogueId on line {line_number}")
    if contains_key(record, "playerId"):
        raise ValueError(f"playerId is forbidden in corpus line {line_number}")
    quality = record.get("quality")
    if not isinstance(quality, dict) or not isinstance(quality.get("complete"), bool):
        raise ValueError(f"missing quality metadata on line {line_number}")
    turns = record.get("turns")
    if not isinstance(turns, list):
        raise ValueError(f"turns must be an array on line {line_number}")
    for turn in turns:
        if not isinstance(turn, dict):
            raise ValueError(f"invalid turn on line {line_number}")
        for role in ("player", "opponent"):
            message = turn.get(role)
            if not isinstance(message, dict) or not str(message.get("text", "")).strip():
                raise ValueError(f"empty {role} message on line {line_number}")


def contains_key(value: Any, forbidden: str) -> bool:
    if isinstance(value, dict):
        return forbidden in value or any(contains_key(item, forbidden) for item in value.values())
    if isinstance(value, list):
        return any(contains_key(item, forbidden) for item in value)
    return False


def dialogue_roots(records: Iterable[dict[str, Any]]) -> dict[str, str]:
    by_id = {record["dialogueId"]: record for record in records}
    roots: dict[str, str] = {}
    for dialogue_id in by_id:
        current = dialogue_id
        visited: set[str] = set()
        while current in by_id:
            if current in visited:
                raise ValueError(f"cycle in dialogue parents at {dialogue_id}")
            visited.add(current)
            parent = by_id[current].get("parentDialogueId")
            if not parent:
                break
            current = parent
        roots[dialogue_id] = current
    return roots


def split_records(
    records: list[dict[str, Any]], seed: str, train_ratio: float, validation_ratio: float
) -> dict[str, list[dict[str, Any]]]:
    if train_ratio <= 0 or validation_ratio < 0 or train_ratio + validation_ratio >= 1:
        raise ValueError("ratios must leave a positive test split")
    roots = dialogue_roots(records)
    result: dict[str, list[dict[str, Any]]] = {"train": [], "validation": [], "test": []}
    assignments: dict[str, str] = {}
    buckets: dict[str, float] = {}
    split_ranges = {
        "train": (0.0, train_ratio),
        "validation": (train_ratio, train_ratio + validation_ratio),
        "test": (train_ratio + validation_ratio, 1.0),
    }
    for root in set(roots.values()):
        digest = hashlib.sha256(f"{seed}:{root}".encode("utf-8")).digest()
        bucket = int.from_bytes(digest[:8], "big") / 2**64
        buckets[root] = bucket
        assignments[root] = (
            "train" if bucket < train_ratio else "validation" if bucket < train_ratio + validation_ratio else "test"
        )

    required = ["train", "test"]
    if validation_ratio > 0:
        required.insert(1, "validation")
    if len(assignments) >= len(required):
        for missing in required:
            if missing in assignments.values():
                continue
            counts = Counter(assignments.values())
            donors = [name for name in required if counts[name] > 1]
            if not donors:
                break
            midpoint = sum(split_ranges[missing]) / 2
            candidates = [root for root, split in assignments.items() if split in donors]
            selected = min(candidates, key=lambda root: (abs(buckets[root] - midpoint), root))
            assignments[selected] = missing

    for record in records:
        result[assignments[roots[record["dialogueId"]]]].append(record)
    return result


def write_jsonl(path: Path, records: Iterable[dict[str, Any]]) -> None:
    with path.open("w", encoding="utf-8", newline="\n") as target:
        for record in records:
            target.write(json.dumps(record, ensure_ascii=False, separators=(",", ":")) + "\n")


def build_manifest(splits: dict[str, list[dict[str, Any]]], skipped: Counter[str], source: Path) -> dict[str, Any]:
    manifest: dict[str, Any] = {
        "schemaVersion": "1.0",
        "source": str(source),
        "skipped": dict(sorted(skipped.items())),
        "splits": {},
    }
    for name, records in splits.items():
        origins: Counter[str] = Counter()
        intents: Counter[str] = Counter()
        reply_sources: Counter[str] = Counter()
        scenarios: Counter[str] = Counter()
        outcomes: Counter[str] = Counter()
        turn_count = 0
        for record in records:
            origins[record["origin"]] += 1
            scenarios[record["scenario"]["id"]] += 1
            if record.get("result"):
                outcomes[record["result"].get("outcomeCode", "unknown")] += 1
            for turn in record["turns"]:
                turn_count += 1
                analysis = turn.get("analysis", {})
                intents[analysis.get("intent", "unknown")] += 1
                reply_sources[analysis.get("replySource", "unknown")] += 1
        manifest["splits"][name] = {
            "dialogues": len(records),
            "turns": turn_count,
            "origins": dict(sorted(origins.items())),
            "scenarios": dict(sorted(scenarios.items())),
            "intents": dict(sorted(intents.items())),
            "replySources": dict(sorted(reply_sources.items())),
            "outcomes": dict(sorted(outcomes.items())),
        }
    return manifest


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path, help="source JSONL exported by Negotiation Arena")
    parser.add_argument("output", type=Path, help="output directory")
    parser.add_argument("--include-incomplete", action="store_true", help="keep legacy records with quality issues")
    parser.add_argument("--seed", default="negotiation-arena-v1")
    parser.add_argument("--train-ratio", type=float, default=0.70)
    parser.add_argument("--validation-ratio", type=float, default=0.15)
    args = parser.parse_args()

    records, skipped = load_records(args.input, args.include_incomplete)
    splits = split_records(records, args.seed, args.train_ratio, args.validation_ratio)
    args.output.mkdir(parents=True, exist_ok=True)
    for name, items in splits.items():
        write_jsonl(args.output / f"{name}.jsonl", items)
    manifest = build_manifest(splits, skipped, args.input)
    with (args.output / "manifest.json").open("w", encoding="utf-8", newline="\n") as target:
        json.dump(manifest, target, ensure_ascii=False, indent=2)
        target.write("\n")


if __name__ == "__main__":
    main()
