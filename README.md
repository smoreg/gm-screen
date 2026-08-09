# gm-screen

Ширма мастера для D&D 5e (2014) — одностраничное веб-приложение + маленький Go-сервер.
Экран отдаётся статикой, а сервер добавляет одну ручку: **импорт листов персонажей игроков
через нейронку** (вставляешь текст чарника → LLM приводит его к формату ширмы).

- Фронт: `web/index.html` — самодостаточный HTML (ваниль JS, без сборки), 16 приключений,
  статблоки, кубы, трекер инициативы, читалка доков, бестиарий, **вкладка «Партия»**.
- Бэк: `main.go` + `parse.go` — `net/http`, один эндпоинт `POST /api/parse-character`,
  который дёргает Claude через Anthropic Go SDK и возвращает объект `gm-character/v1`.

Контракт формата персонажа описан в **[FORMAT.md](FORMAT.md)** и в
**[web/character.schema.json](web/character.schema.json)** — LLM всегда отдаёт ровно эту форму,
велосипед не переизобретается.

> ⚠️ **Репозиторий публичный.** Никаких ключей в git. Все секреты — в `secrets/`
> (папка в `.gitignore`), на проде — через EnvironmentFile / секрет-менеджер хоста.

## Быстрый старт

```bash
# 1) положи ключ (файл secrets/env НЕ коммитится)
cp secrets/env.example secrets/env
$EDITOR secrets/env          # впиши ANTHROPIC_API_KEY=sk-ant-...

# 2) собери и запусти
go build -o gm-screen .
set -a; . ./secrets/env; set +a
./gm-screen
# → http://localhost:8777  (экран мастера; вкладка «Партия» — импорт игроков)
```

Без ключа сервер всё равно поднимется и раздаст экран — не работает только импорт
персонажей (ручка вернёт `503 ANTHROPIC_API_KEY is not set`). Всё остальное на экране
живёт целиком в браузере и сервер ему не нужен.

## Переменные окружения

| Переменная          | По умолчанию        | Зачем |
|---------------------|---------------------|-------|
| `ANTHROPIC_API_KEY` | —                   | Ключ для импорта персонажей. Только для `/api/parse-character`. |
| `PORT`              | `8777`              | Порт HTTP. |
| `PARSE_MODEL`       | `claude-opus-4-8`   | Модель-парсер. Строкой, чтобы пережить смену версий SDK. |
| `GM_WEB_DIR`        | `./web`             | Откуда раздавать статику. |

## API

### `POST /api/parse-character`
```jsonc
// запрос
{ "text": "любой текст чарника: экспорт D&D Beyond, OCR со скрина, описание словами…" }
// ответ 200 — объект gm-character/v1 (см. FORMAT.md)
{ "schema":"gm-character/v1", "name":"…", "level":3, "ac":15, "hp":{…}, "abilities":{…}, … }
```
Коды ошибок: `400` пустой/битый body, `422` модель не вернула валидный JSON,
`502` вызов модели упал, `503` не задан ключ.

### `GET /healthz` → `{"ok":true}`

## Деплой

Репозиторий публичный, поэтому ключ живёт **вне** git.

**systemd** (см. `deploy/gm-screen.service`) — ключ в отдельном EnvironmentFile
`/etc/gm-screen/env` с правами `600`, root-only. `deploy/gm-screen.service` можно
скопировать в `/etc/systemd/system/`, положить ключ в `/etc/gm-screen/env` и
`systemctl enable --now gm-screen`.

**Docker** (см. `Dockerfile`) — multi-stage, статик-бинарь на `gcr.io/distroless/static`.
Ключ пробрасывается на рантайме, в образ не пекётся:
```bash
docker build -t gm-screen .
docker run -p 8777:8777 --env-file ./secrets/env gm-screen
```

## Раскладка

```
go-screen/
  main.go                 HTTP-сервер, роутинг, middleware, конфиг из env
  parse.go                вызов Claude, извлечение и нормализация JSON
  FORMAT.md               человекочитаемый контракт gm-character/v1
  web/
    index.html            вся ширма мастера (раздаётся как есть)
    character.schema.json JSON Schema формата персонажа
    ФАЗА-1_канон-и-сюжет.md, ФАЗА-2_боевой-чистовик.md   модуль про коров
    docs_txt/*.txt        тексты всех 16 приключений для читалки
  secrets/
    env.example           шаблон (коммитится)
    env                   реальный ключ (в .gitignore, НЕ коммитится)
  deploy/
    gm-screen.service     unit для systemd
  Dockerfile
```
