# Spec Delta

## Purpose

Ця можливість надає локальний MCP сервер для читання та редагування RDL через стабільний набір інструментів, доступний клієнтам по stdio.

## ADDED Requirements

### Requirement: Запуск і обмін MCP повідомленнями
Сервер SHALL запускатися як Go-бінарник без Python у середовищі користувача та обмінюватися MCP повідомленнями через stdin/stdout відповідно до погодженої з клієнтом версії протоколу. Діагностика SHALL надходити у stderr або файл, а не у stdout.

#### Scenario: Підключення клієнта
- **WHEN** MCP-клієнт запускає бінарник і надсилає `initialize`
- **THEN** сервер завершує ініціалізацію, оголошує підтримку tools і приймає подальші `tools/list` та `tools/call`

#### Scenario: Повідомлення без відповіді
- **WHEN** клієнт надсилає `notifications/initialized`
- **THEN** сервер не записує відповідь у stdout і залишається готовим до наступного запиту

#### Scenario: Керування логуванням
- **WHEN** користувач задає `RDL_MCP_LOG_LEVEL` або `RDL_MCP_LOG_FILE`
- **THEN** сервер застосовує ці параметри для діагностики без домішок до MCP-повідомлень у stdout

### Requirement: Стабільний каталог інструментів
Сервер SHALL оголошувати 15 наявних назв tools із сумісними обов'язковими й необов'язковими аргументами: `describe_rdl_report`, `get_rdl_datasets`, `get_rdl_parameters`, `get_rdl_columns`, `validate_rdl`, `update_column_header`, `update_column_width`, `update_column_format`, `add_column`, `remove_column`, `update_stored_procedure`, `add_dataset_field`, `remove_dataset_field`, `add_parameter`, `update_parameter`.

#### Scenario: Отримання каталогу
- **WHEN** клієнт викликає `tools/list`
- **THEN** він отримує всі 15 інструментів із JSON Schema аргументів, включно з потрібним `filepath` і параметрами, які вже описані Python-сервером

### Requirement: Сумісні результати викликів
Для коректного виклику сервер SHALL повертати MCP tool result з текстовим JSON, який зберігає ключі, типи значень і домовлені значення за замовчуванням наявного Python tool result. Відсутність необов'язкового аргументу SHALL зберігати поточну семантику: `field_limit=0`, `width="1in"`, `auto_adjust_page_width=true`; відсутні `field_pattern`, `format_string`, `footer_expression`, `prompt` і `default_value` SHALL лишатися невказаними.

#### Scenario: Читання звіту
- **WHEN** клієнт викликає `describe_rdl_report` із коректним `filepath`
- **THEN** відповідь містить текстовий JSON із поточними полями опису звіту

#### Scenario: Відсутній необов'язковий аргумент
- **WHEN** клієнт викликає `get_rdl_datasets` без `field_limit`
- **THEN** сервер повертає кількість полів без їхнього повного списку, як і Python-реалізація

### Requirement: Помилки не руйнують MCP сеанс
Сервер SHALL повідомляти про невідомий метод чи інструмент та некоректні аргументи за правилами MCP/JSON-RPC, а помилки окремого виклику SHALL не завершувати наступні виклики в тому самому сеансі. Невалідний JSON на stdio SHALL завершувати сеанс відповідно до поведінки офіційного MCP Go SDK; діагностика SHALL надходити у stderr або файл.

#### Scenario: Некоректний виклик
- **WHEN** клієнт викликає tool без обов'язкового `filepath`
- **THEN** він отримує помилку цього виклику, а наступний коректний виклик обробляється

#### Scenario: Невалідний JSON у stdio
- **WHEN** клієнт надсилає рядок із невалідним JSON
- **THEN** сервер завершує поточний stdio-сеанс із діагностикою поза stdout; подальші виклики потребують нового запуску процесу

### Requirement: Актуальна команда встановлення
Опубліковані інструкції й метадані MCP registry SHALL вказувати на підтримуваний спосіб отримати та запустити Go-бінарник замість Python `uvx`/PyPI команди.

#### Scenario: Налаштування нового клієнта
- **WHEN** користувач налаштовує MCP-клієнт за README або записом registry після переходу
- **THEN** вказана команда запускає Go-сервер через stdio без Python
