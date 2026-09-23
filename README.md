# Сервер RDL MCP

mcp-name: io.github.bethmaloney/rdl-mcp

[![Go 1.27+](https://img.shields.io/badge/Go-1.27%2B-00ADD8.svg)](https://go.dev/doc/install)
[![Ліцензія MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![MCP](https://img.shields.io/badge/MCP-сумісний-green.svg)](https://modelcontextprotocol.io)

RDL MCP дає Claude, GitHub Copilot та іншим MCP-клієнтам інструменти для читання й редагування звітів SSRS у форматі RDL. Сервер працює як самостійний Go-бінарник через stdio.

## Можливості

**Читання звітів:**

- `describe_rdl_report` — огляд структури звіту.
- `get_rdl_datasets` — датасети, поля та збережені процедури; підтримуються обмеження й фільтрація полів.
- `get_rdl_parameters` — параметри звіту.
- `get_rdl_columns` — заголовки, ширини та прив'язки колонок.
- `validate_rdl` — перевірка XML, структури звіту та посилань на поля.

**Редагування звітів:**

- `update_column_header`, `update_column_width`, `update_column_format` — зміна колонок.
- `add_column`, `remove_column` — додавання й вилучення колонок Tablix.
- `update_stored_procedure` — зміна збереженої процедури датасету.
- `add_dataset_field`, `remove_dataset_field` — керування полями датасету.
- `add_parameter`, `update_parameter` — додавання й оновлення параметрів.

## Встановлення

Для збирання потрібен Go 1.27 або новіший. Після публікації Go-версії репозиторію використовуйте один із варіантів нижче.

### Встановити виконуваний файл

Встановіть сервер до каталогу `GOBIN` або, якщо його не задано, до `$GOPATH/bin`:

```sh
go install github.com/h0rn3t/rdl-mcp/cmd/rdl-mcp@latest
```

Переконайтеся, що каталог із `rdl-mcp` є у `PATH`. Для локальної робочої копії репозиторію виконайте команду з його кореня:

```sh
go install ./cmd/rdl-mcp
```

Команда з `@latest` доступна, коли Go-версія опублікована у вказаному Go-модулі.

### Додати сервер до Go-модуля через `go get`

У Go 1.24 і новіших `go get -tool` додає виконуваний інструмент до `go.mod`; запустити його можна командою `go tool`. Створіть окремий модуль або використайте наявний:

```sh
mkdir rdl-mcp-tools
cd rdl-mcp-tools
go mod init example.com/rdl-mcp-tools
go get -tool github.com/h0rn3t/rdl-mcp/cmd/rdl-mcp@latest
go tool rdl-mcp
```

Ця команда реєструє MCP-сервер як інструмент поточного Go-модуля. Звичайний `go get` оновлює залежності в `go.mod`; для глобального встановлення виконуваного файла використовуйте `go install`. Докладніше: [про `go get` та встановлення команд](https://go.dev/doc/go-get-install-deprecation) і [керування інструментами Go](https://go.dev/doc/modules/managing-dependencies).

Щоб MCP-клієнт запускав варіант із `go get -tool`, вкажіть шлях до цього модуля:

```json
{
  "command": "go",
  "args": ["-C", "/шлях/до/rdl-mcp-tools", "tool", "rdl-mcp"]
}
```

### Налаштувати MCP-клієнт

У прикладах нижче `/шлях/до/rdl-mcp` — абсолютний шлях до встановленого бінарника. Для запуску через Go tool див. конфігурацію вище.

<details>
<summary><b>Claude Desktop</b></summary>

Додайте запис у `claude_desktop_config.json`:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "rdl-mcp": {
      "command": "/шлях/до/rdl-mcp",
      "args": []
    }
  }
}
```

</details>

<details>
<summary><b>GitHub Copilot у VS Code</b></summary>

Додайте запис у `.vscode/mcp.json` у робочому просторі або в конфігурацію користувача:

```json
{
  "servers": {
    "rdl-mcp": {
      "type": "stdio",
      "command": "/шлях/до/rdl-mcp",
      "args": []
    }
  }
}
```

Потрібні VS Code та GitHub Copilot Chat із підтримкою MCP.

</details>

Після зміни конфігурації перезапустіть клієнт або перезапустіть MCP-сервер у ньому. Перевірте підключення запитом на кшталт: «Опиши структуру мого файлу report.rdl».

### Налагодження

Сервер читає рівень і шлях журналу з таких змінних середовища:

- `RDL_MCP_LOG_LEVEL`: `DEBUG`, `INFO`, `WARNING` або `ERROR`.
- `RDL_MCP_LOG_FILE`: шлях до файлу журналу.

Діагностика пишеться у stderr або у вказаний файл, щоб не порушувати MCP-обмін через stdout.

## Приклади запитів

- «Які датасети використовує цей звіт?»
- «Зміни ширину колонки Account Number на 2 дюйми».
- «Відформатуй Amount як валюту з двома десятковими знаками».
- «Додай колонку Amount із сумою у підсумковому рядку».
- «Додай колонку Status без виразу у підсумковому рядку».
- «Заміни процедуру основного датасету на V2 і додай поле TaxAmount».
- «Вилучи застарілу колонку Status».
- «Додай параметр Year для фільтрації звіту».

## Довідник інструментів

<details>
<summary>Переглянути аргументи 15 інструментів</summary>

### Читання

- `describe_rdl_report(filepath)` — структура звіту.
- `get_rdl_datasets(filepath, field_limit?, field_pattern?)` — датасети та їхні поля.
  - `field_limit`: `0` — лише кількість полів (типово), `-1` — усі поля, `N` — не більше `N` полів.
  - `field_pattern`: необов'язковий регулярний вираз для назв полів.
- `get_rdl_parameters(filepath)` — параметри.
- `get_rdl_columns(filepath)` — колонки Tablix.
- `validate_rdl(filepath)` — перевірка звіту.

### Редагування

- `update_column_header(filepath, old_header, new_header)` — змінити текст заголовка.
- `update_column_width(filepath, column_index, new_width)` — змінити ширину, наприклад `2.5in`.
- `update_column_format(filepath, column_index, format_string)` — змінити формат, наприклад `#,0.00`, `dd/MM/yyyy` або `C2`.
- `add_column(filepath, column_index, header_text, field_binding, width?, format_string?, footer_expression?)` — додати колонку.
  - `footer_expression` — необов'язковий вираз підсумкового рядка, наприклад `=Sum(Fields!Amount.Value)`, `=Count(Fields!ID.Value)` або `Total:`.
- `remove_column(filepath, column_index)` — вилучити колонку.
- `update_stored_procedure(filepath, dataset_name, new_sproc)` — змінити процедуру датасету.
- `add_dataset_field(filepath, dataset_name, field_name, data_field, type_name)` — додати поле.
- `remove_dataset_field(filepath, dataset_name, field_name)` — вилучити поле.
- `add_parameter(filepath, name, data_type, prompt)` — додати параметр.
- `update_parameter(filepath, name, prompt?, default_value?)` — оновити параметр.

Інструменти редагування повертають результат операції; помилки та бізнес-відмови містять пояснення.

</details>

## Обмеження

- Підтримуються звіти RDL 2016 і таблиці Tablix.
- Matrix та Chart не підтримуються.
- Для складних конструкцій RDL може знадобитися ручне редагування XML.

## Усунення проблем

**Сервер не з'являється у клієнті:**

- Перевірте, що в конфігурації вказаний абсолютний шлях до бінарника.
- Переконайтеся, що Go встановлений, якщо клієнт запускає сервер через `go tool`.
- Перезапустіть MCP-сервер або клієнт після зміни конфігурації.

**Помилка доступу до RDL:**

- Перевірте, що користувач має права читати й записувати файл.
- Для запуску через Go tool перевірте шлях у параметрі `-C`.

## Розробка

Потрібен Go 1.27 або новіший. Основні локальні перевірки:

```sh
go build ./...
go vet ./...
go test -race ./...
```

Пропозиції змін вітаються. Перед відкриттям PR створіть гілку, внесіть зміну та додайте перевірку поведінки.
## Ліцензія

MIT. Докладніше — у файлі [LICENSE](LICENSE).
