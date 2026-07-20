# waiting-mcp

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
│   └── {id}.md
└── people/
    └── {id}.md
```

## Сборка

```bash
go build -o waiting-mcp ./cmd/waiting-mcp
```

## Запуск

### stdio (для Cursor)

```bash
WAITING_DATA_DIR=./data ./waiting-mcp --stdio
```

### HTTP

```bash
export WAITING_DATA_DIR=./data
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
| `waiting_list` | Список с фильтрами |
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
