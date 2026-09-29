#!/usr/bin/env python3
"""HTTP inference service for ArenaLM."""

from __future__ import annotations

import json
import os
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

try:
    from .arena_model.model import NGramIntentModel, extract_alternative, extract_proposal_value
except ImportError:
    from arena_model.model import NGramIntentModel, extract_alternative, extract_proposal_value


MAX_REQUEST_BYTES = 64 * 1024


class ModelApplication:
    def __init__(self, model: NGramIntentModel, minimum_confidence: float = 0.55) -> None:
        if not 0 <= minimum_confidence <= 1:
            raise ValueError("minimum confidence must be between zero and one")
        self.model = model
        self.minimum_confidence = minimum_confidence

    def interpret(self, request: dict[str, Any]) -> dict[str, Any]:
        raw_message = request.get("message", "")
        if not isinstance(raw_message, str):
            raise ValueError("message must be a string")
        message = raw_message.strip()
        if not message or len(message) > 4000:
            raise ValueError("message must contain 1-4000 characters")
        offer_made = request.get("offerMade", False)
        proposal_maximum = request.get("proposalMaximum", 0)
        proposal_alternatives = request.get("proposalAlternatives", [])
        if not isinstance(offer_made, bool):
            raise ValueError("offerMade must be a boolean")
        if isinstance(proposal_maximum, bool) or not isinstance(proposal_maximum, int) or proposal_maximum < 0:
            raise ValueError("proposalMaximum must be a non-negative integer")
        if not isinstance(proposal_alternatives, list) or not all(isinstance(item, str) for item in proposal_alternatives):
            raise ValueError("proposalAlternatives must be a string array")
        prediction = self.model.predict(message)
        intent = prediction.intent if prediction.confidence >= self.minimum_confidence else "neutral"
        relevant = intent != "neutral"
        proposal_value = 0
        alternative_id = ""
        if intent == "propose":
            proposal_value = extract_proposal_value(message, proposal_maximum)
            alternative_id = extract_alternative(message, proposal_alternatives)
        if intent == "accept" and not offer_made:
            intent, relevant = "neutral", False
        return {
            "intent": intent,
            "proposalValue": proposal_value,
            "alternativeId": alternative_id,
            "relevant": relevant,
            "confidence": round(prediction.confidence, 6),
            "modelVersion": self.model.version,
        }


def make_handler(application: ModelApplication) -> type[BaseHTTPRequestHandler]:
    class Handler(BaseHTTPRequestHandler):
        server_version = "ArenaLM/1"

        def do_GET(self) -> None:
            if self.path == "/health":
                self.write_json(HTTPStatus.OK, {"status": "ready", "modelVersion": application.model.version})
                return
            self.write_json(HTTPStatus.NOT_FOUND, {"error": "not found"})

        def do_POST(self) -> None:
            if self.path != "/v1/interpret":
                self.write_json(HTTPStatus.NOT_FOUND, {"error": "not found"})
                return
            try:
                content_length = int(self.headers.get("Content-Length", "0"))
                if content_length <= 0 or content_length > MAX_REQUEST_BYTES:
                    raise ValueError("invalid request size")
                request = json.loads(self.rfile.read(content_length))
                if not isinstance(request, dict):
                    raise ValueError("request must be an object")
                self.write_json(HTTPStatus.OK, application.interpret(request))
            except (ValueError, TypeError, json.JSONDecodeError) as error:
                self.write_json(HTTPStatus.BAD_REQUEST, {"error": str(error)})

        def write_json(self, status: HTTPStatus, payload: dict[str, Any]) -> None:
            body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format: str, *args: object) -> None:
            return

    return Handler


def main() -> None:
    model_path = Path(os.getenv("ARENA_MODEL_PATH", "/app/model/arena-intents-v1.json"))
    address = os.getenv("ARENA_MODEL_ADDR", "0.0.0.0")
    port = int(os.getenv("ARENA_MODEL_PORT", "8090"))
    minimum_confidence = float(os.getenv("ARENA_MODEL_MIN_CONFIDENCE", "0.55"))
    application = ModelApplication(NGramIntentModel.load(model_path), minimum_confidence)
    server = ThreadingHTTPServer((address, port), make_handler(application))
    print(json.dumps({"event": "started", "address": address, "port": port, "modelVersion": application.model.version}), flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
