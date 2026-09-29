import json
import shutil
import unittest
from pathlib import Path

from scripts.prepare_corpus import load_records, split_records


def record(dialogue_id: str, parent: str = "", complete: bool = True) -> dict:
    item = {
        "schemaVersion": "1.0",
        "origin": "synthetic",
        "dialogueId": dialogue_id,
        "status": "finished",
        "scenario": {"id": "scenario"},
        "quality": {"complete": complete, "issues": []},
        "turns": [
            {
                "player": {"text": "Вопрос"},
                "opponent": {"text": "Ответ"},
                "analysis": {"intent": "ask_interest", "replySource": "local"},
            }
        ],
    }
    if parent:
        item["parentDialogueId"] = parent
    return item


class PrepareCorpusTest(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = Path.cwd() / "corpus" / "exports" / "test-prepare-corpus"
        shutil.rmtree(self.directory, ignore_errors=True)
        self.directory.mkdir(parents=True)

    def tearDown(self) -> None:
        shutil.rmtree(self.directory, ignore_errors=True)

    def test_filters_incomplete_and_keeps_forks_in_one_split(self) -> None:
        source = self.directory / "source.jsonl"
        empty = record("empty")
        empty["turns"] = []
        items = [record("root"), record("child", "root"), record("legacy", complete=False), empty]
        source.write_text("".join(json.dumps(item, ensure_ascii=False) + "\n" for item in items), encoding="utf-8")
        loaded, skipped = load_records(source)
        self.assertEqual(2, len(loaded))
        self.assertEqual(1, skipped["incomplete"])
        self.assertEqual(1, skipped["empty_dialogue"])
        splits = split_records(loaded, "seed", 0.7, 0.15)
        locations = {
            item["dialogueId"]: split
            for split, records in splits.items()
            for item in records
        }
        self.assertEqual(locations["root"], locations["child"])

    def test_rejects_player_identifier(self) -> None:
        source = self.directory / "source.jsonl"
        item = record("unsafe")
        item["playerId"] = "secret"
        source.write_text(json.dumps(item) + "\n", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "playerId"):
            load_records(source)

    def test_keeps_siblings_with_missing_parent_together(self) -> None:
        items = [record("child-a", "missing-root"), record("child-b", "missing-root")]
        splits = split_records(items, "seed", 0.7, 0.15)
        locations = {
            item["dialogueId"]: split
            for split, records in splits.items()
            for item in records
        }
        self.assertEqual(locations["child-a"], locations["child-b"])

    def test_populates_all_splits_when_three_roots_exist(self) -> None:
        splits = split_records([record("a"), record("b"), record("c")], "seed", 0.7, 0.15)
        self.assertTrue(all(splits.values()))


if __name__ == "__main__":
    unittest.main()
