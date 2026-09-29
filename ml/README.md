# ArenaLM

ArenaLM v2 — небольшая локальная нейросеть на PyTorch для классификации
переговорных реплик. Она определяет одно из 11 намерений игрока, возвращает
уверенность и извлекает числовое предложение. Модель работает на CPU без
внешнего API.

Текст преобразуется в стабильный вектор из слов и символьных n-грамм. Линейная
голова PyTorch обучается различать намерения по этим признакам, а temperature
scaling калибрует уверенность перед применением порога fallback. Текущий
артефакт содержит 180 235 параметров. Такой размер подходит имеющемуся
небольшому корпусу и быстро работает в локальном контейнере.

## Данные

- `../corpus/seed-v1.jsonl` — диалоги, созданные игровым движком;
- `data/bootstrap-intents.jsonl` — дополнительные размеченные формулировки;
- `data/eval-intents.jsonl` — отдельный набор для регрессионной оценки;
- `model/arena-intents-v2.pt` — обученный PyTorch артефакт.

Runtime корпус можно добавить несколькими флагами `--corpus`. Неполные записи
автоматически пропускаются.

## Локальная установка

Из корня проекта в PowerShell:

```powershell
py -3.13 -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install --index-url https://download.pytorch.org/whl/cpu -r ml/requirements.txt
```

Зависимости устанавливаются в `.venv`, который исключён из Git.

## Обучение и оценка

```powershell
python -m ml.train `
  --corpus corpus/seed-v1.jsonl `
  --output ml/model/arena-intents-v2.pt `
  --version arena-intents-v2-pytorch

python -m ml.evaluate --minimum-accuracy 0.90
python -m unittest ml.tests.test_model -v
```

Обучение воспроизводимо при одинаковых данных и `--seed`. Eval набор содержит
33 близких к предметной области примера. Текущий артефакт показывает accuracy
0,9394 и macro F1 0,9409. Эта выборка служит регрессионной проверкой; для
надёжной оценки потребуется больше размеченных реальных реплик.

## HTTP API

Сервис запускается контейнером `model` на порту `8090`.

- `GET /health` — готовность и версия модели;
- `POST /v1/interpret` — классификация `InterpretationRequest` backend.

Пример запроса:

```json
{
  "message": "Какие условия для вас наиболее важны?",
  "scenarioTopic": "Условия поставки",
  "playerGoal": "Согласовать выгодные условия",
  "opponentRole": "Поставщик",
  "phase": "exploration",
  "offerMade": false,
  "proposalKind": "none",
  "proposalMaximum": 0,
  "proposalAlternatives": [],
  "conversationHistory": []
}
```

Пример ответа:

```json
{
  "intent": "ask_interest",
  "proposalValue": 0,
  "alternativeId": "",
  "relevant": true,
  "confidence": 0.97,
  "modelVersion": "arena-intents-v2-pytorch"
}
```

Backend сначала применяет строгие локальные правила к очевидным действиям,
затем обращается к ArenaLM. При низкой уверенности или недоступности сервиса
сохраняется существующий fallback. ArenaLM не изменяет очки, состояние
переговоров или итог сессии.
