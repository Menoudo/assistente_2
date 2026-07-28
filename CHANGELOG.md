# Changelog

Все значимые изменения в **waiting-mcp** документируются в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
версии следуют [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-07-28

### Added

- Завершённые ожидания переносятся в `data/waiting/done/` при `waiting_complete`.
- Фильтр `include_done` в `waiting_list` для чтения архивных карточек.
- При deploy копируются `README.md` и `CHANGELOG.md` в целевую директорию.

### Changed

- Активные и завершённые карточки хранятся в разных каталогах markdown.

## [0.1.0] - 2026-07-24

### Added

- MCP-сервер **waiting-mcp** с markdown-хранилищем GTD Waiting For.
- CRUD для ожиданий и людей, `waiting_review_due`, история проверок.
- Транспорты stdio и HTTP (Streamable HTTP).
- Скрипт `scripts/deploy.sh` и GitHub Actions workflow для тестов.

[0.2.0]: https://github.com/Menoudo/assistente_2/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Menoudo/assistente_2/releases/tag/v0.1.0
