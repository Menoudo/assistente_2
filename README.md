# waiting-mcp

[![Tests](https://github.com/Menoudo/assistente_2/actions/workflows/test.yml/badge.svg)](https://github.com/Menoudo/assistente_2/actions/workflows/test.yml)

MCP-сервер для управления списком GTD **Waiting For** («Ожидаю»). Все данные хранятся в markdown-файлах с фиксированной структурой: один файл на ожидание, отдельные файлы людей.

## Возможности

- CRUD для ожиданий и людей
- Разделение **срока результата** и **даты следующей проверки**
- Ежедневный обзор через `waiting_review_due`
- История проверок в markdown
- MCP resources: `waiting://{id}`, `waiting://due/today`, `people://{id}`
- Транспорты: **stdio** (Cursor) и **HTTP** (Streamable HTTP)

## Структура данных

```
data/
├── waiting/
│   ├── {id}.md
│   └── done/
│       └── {id}.md
└── people/
    └── {id}.md
```

Активные и отменённые ожидания хранятся в `waiting/`. При завершении (`waiting_complete`) карточка автоматически переносится в `waiting/done/`.

## Сборка

```bash
go build -o waiting-mcp ./cmd/waiting-mcp
```

## Развёртывание

Скрипт собирает бинарник и создаёт структуру каталогов в указанном месте:

```bash
chmod +x ./scripts/deploy.sh
./scripts/deploy.sh ~/gtd/waiting-mcp
```

Опции:

```bash
# с примерами markdown и глобальным MCP-конфигом Cursor
./scripts/deploy.sh --examples --cursor-global ~/gtd/waiting-mcp

# если <target-dir> — корень workspace, можно положить .cursor/mcp.json
./scripts/deploy.sh --cursor-project ~/projects/my-gtd
```

Структура после deploy:

```
<target-dir>/
├── bin/waiting-mcp
├── README.md
├── CHANGELOG.md
└── data/
    ├── waiting/
    └── people/
```

Текущая версия: **0.2.2** (см. `CHANGELOG.md`).

## Запуск

### stdio (для Cursor)

```bash
WAITING_DATA_DIR=./data ./waiting-mcp --stdio
```

### HTTP

```bash
export WAITING_DATA_DIR=./data
export WAITING_TIMEZONE=Europe/Moscow
export WAITING_MCP_TOKEN=your-secret-token
./waiting-mcp --http :8080
```

Endpoint: `http://localhost:8080/mcp`

## Подключение в Cursor

Добавьте в `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "waiting": {
      "command": "/absolute/path/to/waiting-mcp",
      "args": ["--stdio"],
      "env": {
        "WAITING_DATA_DIR": "/absolute/path/to/your/data"
      }
    }
  }
}
```

## MCP tools

| Tool | Описание |
|------|----------|
| `waiting_create` | Создать ожидание |
| `waiting_get` | Получить ожидание |
| `waiting_list` | Список с фильтрами (`status`, `responsible`, `due_before`, `context`, `include_done`) |
| `waiting_update` | Обновить поля |
| `waiting_record_check` | Зафиксировать проверку и назначить следующую дату |
| `waiting_complete` | Завершить |
| `waiting_cancel` | Отменить |
| `waiting_review_due` | Обзор просроченных/сегодняшних проверок |
| `person_upsert` | Создать/обновить человека |
| `person_list` | Список людей |
| `person_get` | Человек + его активные ожидания |

## Ежедневный workflow

1. Вызвать `waiting_review_due`
2. Для каждого пункта проверить внешние источники (GitLab, документы)
3. Если статус не виден — пинговать ответственного
4. Зафиксировать результат через `waiting_record_check` с новой датой `next_check`

## Тесты

```bash
go test ./...
```

## Целостность данных и совместимость

- ID — один компонент имени файла: пустые значения, разделители путей,
  управляющие символы и `.`/`..` отклоняются. Файловые операции ограничены
  корнем базы через Go `os.Root`, включая переходы по символическим ссылкам.
- Все экземпляры версии 0.2.2 используют межпроцессную блокировку
  `data/.waiting-mcp.lock`. Файл блокировки не следует удалять во время работы.
  При обновлении перезапустите старые процессы сервера: версии до 0.2.2
  эту блокировку не используют. Ручное редактирование файлов требует
  остановки серверов, иначе внешние изменения могут конфликтовать с записью.
- Новые и изменённые карточки используют формат тела `body-v2`: скрытые
  HTML-комментарии отделяют заметки и сообщения истории от структуры документа.
  Обычные заголовки, кодовые блоки и пробельные символы сохраняются.
  Внутри заметок и сообщений строки, начинающиеся с `\` или
  `<!-- waiting-mcp:`, экранируются дополнительным `\`.
  При ручной правке сохраняйте служебные комментарии и это экранирование.
  Старые версии сервера не должны редактировать файлы `body-v2`.
  Старые файлы читаются без миграции; новая версия тела записывается только
  при изменении конкретной карточки. Перед обновлением сохраните резервную копию.
- `waiting_update`: отсутствие поля оставляет его без изменения; `notes: ""`
  очищает заметку, `tags: []` — теги, `blocker: ""` или `clear_blocker: true`
  — блокер. Для удаления сроков используйте `clear_result_deadline: true`
  и `clear_promised_deadline: true`. Одновременная установка и очистка
  одного поля отклоняется. Ответ содержит сохранённый `updated_at`.
- Дата обзора по умолчанию определяется `WAITING_TIMEZONE` или флагом
  `--timezone` (IANA-имя, например `Europe/Moscow`; по умолчанию `UTC`).
  Это относится и к `waiting_review_due`, и к ресурсу `waiting://due/today`.
  Явный `as_of: YYYY-MM-DD` задаёт календарную дату независимо от зоны.
  Вчерашняя дата проверки даёт `days_overdue: 1`, сегодняшняя — `0`.

Расширенная проверка, включая гонки и отдельные процессы:

```bash
go test -race ./...
```
