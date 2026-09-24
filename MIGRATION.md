# Міграція RDL MCP на Go

## Стан

- OpenSpec change: `migrate-rdl-mcp-to-go` (`openspec/changes/migrate-rdl-mcp-to-go/`).
- На пряме рішення користувача прибрано Python runtime/package, їхні тести й налаштування, PyPI workflow та локальні MCP Registry metadata/workflow; зовнішні записи й релізи не змінювалися.
- Go-контрактні JSON/XML fixtures залишилися в `tests/testdata/rdl/`; Go build і тести не залежать від зовнішнього runtime.
- Паритет за наявними fixtures для 15 tools підтверджувався раніше. Запуск Claude Desktop і VS Code/Copilot у UI та повний синтаксичний паритет довільних `field_pattern` ще не підтверджені.
- Згадки попереднього runtime і команд у таблиці нижче — історія перевірок та інструкція відкату; вони не потрібні для поточного Go runtime.

## Порядок зрізів

1. Зафіксувати Python тести, stdio каталог, результати й XML після кожної операції.
2. Додати Go module і перенести XML, читання, валідацію, колонки, датасети та параметри.
3. Під'єднати офіційний MCP Go SDK, порівняти 15 tools, stdio та помилки.
4. Перевірити Go збірку, vet і race тести, а потім підготувати клієнтські та release artifacts.
5. Прибрати попередній runtime/package після контрактних порівнянь; цей крок виконано за прямим рішенням користувача. Go release і новий MCP Registry запис потребують окремого рішення.

## Перевірки

| Зріз | Команда (cwd: корінь репозиторію) | Результат | Середовище |
|---|---|---|---|
| Python baseline | `python3 -m pytest tests/ -v` | 36 passed in 0.07s | 2026-09-23, macOS arm64, Python 3.14.7, pytest 9.0.2 |
| Python stdio fixtures (історичний запис) | `python3 tests/baseline_stdio.py record` і `python3 tests/baseline_stdio.py verify` | 15 tools, 62 cases, 17 XML outputs; verify passed | fixtures збережені в `tests/testdata/rdl/`; запис сформовано до вилучення runtime |
| Go scaffold | `go build ./...`; `go vet ./...`; `go test -race ./...`; `go mod tidy -diff` | усі команди пройшли; Go тестів ще немає | Go 1.27.1 darwin/arm64, module `github.com/h0rn3t/rdl-mcp` |
| XML RDL 2016 | `go build ./...`; `go vet ./...`; `go test -race ./...`; `gofmt -l cmd/rdl-mcp/main.go internal/rdl/xml.go internal/rdl/xml_contract_test.go` | пройшли, gofmt не вивів файлів | `internal/rdl/xml_contract_test.go`, вкладені namespace й точкова зміна; 2026-09-23 |
| Чотири read-only tools | `go build ./...`; `go vet ./...`; `go test -race ./...`; `golangci-lint run ./...`; `govulncheck ./...`; `go mod tidy -diff` | пройшли; lint: 0 issues; vulnerabilities: none found | `internal/rdl/reader_contract_test.go`, Python stdio baseline; 2026-09-23 |
| Валідація й поля | `python3 tests/baseline_expressions.py verify`; `python3 tests/baseline_stdio.py verify`; `go build ./...`; `go vet ./...`; `go test -race ./...`; `golangci-lint run ./...` | пройшли; 8 expression cases, lint: 0 issues | `internal/rdl/validation_contract_test.go`, valid/bad field/no datasets/no Tablix/malformed XML; 2026-09-23 |
| Колонки Tablix | `python3 tests/baseline_stdio.py verify`; `go build ./...`; `go vet ./...`; `go test -race ./...`; `golangci-lint run ./...` | пройшли; lint: 0 issues | `internal/rdl/columns_contract_test.go`, 16 cases, XML, індекси, default-и, ширина сторінки; 2026-09-23 |
| Датасети й параметри | `python3 tests/baseline_stdio.py verify`; `go build ./...`; `go vet ./...`; `go test -race ./...`; `golangci-lint run ./...` | пройшли; lint: 0 issues | `internal/rdl/data_contract_test.go`, 15 cases, XML, дублікати, відсутні об'єкти; 2026-09-23 |
| MCP каталог | `go build ./...`; `go vet ./...`; `go test -race ./...`; `golangci-lint run ./...`; `govulncheck ./...` | пройшли; 15 schemas збіглись, lint: 0 issues, vulnerabilities: none found | SDK `v1.8.0`, `internal/mcpserver/catalog_contract_test.go`; 2026-09-23 |
| MCP виклики | `go test -race ./internal/mcpserver`; `golangci-lint run ./...`; `go fix -diff ./internal/mcpserver` | пройшли; 15 `tools/call` збіглись за JSON змістом, lint: 0 issues | `internal/mcpserver/call_contract_test.go`, окремий RDL для кожного виклику; 2026-09-23 |
| stdio | `go test ./cmd/rdl-mcp -run 'TestStdioSessionAndEOF|TestMalformedJSONEndsSession' -count=1`; `go build ./...`; `go vet ./...`; `go test -race ./...`; `golangci-lint run ./...` | пройшли; lint: 0 issues | `cmd/rdl-mcp/main_contract_test.go`, handshake, notification, помилки, EOF і чистий stdout; 2026-09-23 |
| Спільний Go gate | `go build ./...`; `go vet ./...`; `go test -race ./...`; `golangci-lint run ./...`; `govulncheck ./...`; `openspec validate migrate-rdl-mcp-to-go` | пройшли; lint: 0 issues; vulnerabilities: none found; OpenSpec valid | увесь Go module і поточний OpenSpec change; 2026-09-23 |
| Бінарники для клієнтської перевірки | `CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -trimpath -ldflags='-s -w'` для darwin/linux/windows × amd64/arm64; локальний stdio smoke `dist/rdl-mcp_darwin_arm64` | 6 крос-компіляцій пройшли; native macOS ARM64: handshake, 15 tools, read, edit, EOF пройшли | `dist/` (gitignored), `SHA256SUMS`; виконання на Linux/Windows і клієнтський UI ще не перевірені |
| Повторна перевірка зрізу 4.1 | 6 крос-компіляцій; `go build ./...`; `go vet ./...`; `go test -race ./...`; `go test ./cmd/rdl-mcp -run 'TestStdioSessionAndEOF|TestMalformedJSONEndsSession' -count=1`; `shasum -a 256 -c dist/SHA256SUMS` | усі збірки, Go gate, stdio smoke і SHA-256 пройшли; Claude Desktop та VS Code/Copilot UI-виклики не перевірені | 2026-09-23; спроба прочитати UI через `System Events` зупинилася на macOS `not allowed assistive access` |
| Go-only cleanup | `go build ./...`; `go vet ./...`; `go test -race -count=1 ./...`; stdio smoke; `go mod tidy -diff` | усі команди пройшли; Go-код, contract fixtures, документація та збірка більше не потребують Python файлів чи інструментів | 2026-09-23; fixtures перенесені в `tests/testdata/rdl/`; старий `server.json` і release workflows вилучені з checkout |
| RDL analyst workflows | `gofmt -l .`; `go build ./...`; `go vet ./...`; `go test -race ./...`; `go fix -diff ./internal/rdl ./internal/mcpserver`; `golangci-lint run ./...`; `go mod tidy -diff`; error/docs checks; pre-review | пройшли; gofmt та go fix без diff; lint: 0 issues | 18 tools; fixtures, пакетний Textbox patch із per-file write status, semantic diff, вибір Tablix, статична перевірка; PoC на тимчасових копіях трьох звітів №10 пройшов; SSRS render недоступний; `govulncheck` не запускався, залежності не змінювалися; 2026-09-23 |
| Повторний Go gate і CI | `go build ./...`; `go vet ./...`; `go test -race ./...`; `go mod tidy -diff`; `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/go.yml .github/workflows/release-artifacts.yml` | усі Go-перевірки та actionlint пройшли | macOS arm64, Go 1.27.1; 2026-09-24 |
| MCPB та registry metadata | `scripts/package-mcpb.sh`; `mcp-publisher validate server.json`; `shasum -a 256 -c dist/SHA256SUMS`; stdio smoke з упакованого launcher | manifest і `server.json` валідні; SHA-256 збігаються; пакет відкрив 18 tools і пройшов читання та редагування тимчасового RDL | MCPB CLI 2.1.2; локальний macOS arm64 пакет, непідписаний; 2026-09-24; клієнтський UI не перевірений |

## Рішення щодо регулярних виразів

- Користувач погодив пряму залежність `github.com/dlclark/regexp2/v2@v2.8.0` для ширшої сумісності `field_pattern`.
- Go-порівняння з Python охоплює lookbehind, backreferences, Python named groups, Unicode `\w`, невалідний .NET-синтаксис, невалідний regex і від'ємний `field_limit`. Python named groups/backreferences перетворюються перед компіляцією; .NET-only named group повертається до Python-поведінки без фільтрації.
- Прийнятих розбіжностей у цих fixtures немає. Повний синтаксичний паритет Python `re` для довільного шаблону ще не доведений; це лишається ризиком до кінцевого parity gate.
- Текст причини пошкодженого XML відрізняється між Python `ElementTree` і Go `encoding/xml`; обидва повертають `valid=false` та одну `issues` з префіксом `XML Parse Error:`. Це прийнята відмінність формулювання, класифікація не змінена.
- Офіційний MCP Go SDK зафіксовано на `v1.8.0`. Транзитивний `golang.org/x/sys` піднято з `v0.41.0` до `v0.44.0` після сканування, яке знайшло не викликану нашим кодом Windows-вразливість у старій версії; повторний `govulncheck ./...` не знайшов вразливостей.
- Користувач прийняв поведінку SDK `v1.8.0` `StdioTransport`: невалідний JSON завершує сеанс без parse-error відповіді; клієнт мусить перезапустити процес. OpenSpec spec і `TestMalformedJSONEndsSession` оновлені. Коректно сформовані невідомі методи та невалідні tool-аргументи не руйнують сеанс.
- SDK повертає відсутній `filepath` як MCP tool result із `isError=true`, тоді як Python повертав JSON-RPC error `-32000`; це прийнята відмінність за правилами офіційного SDK. JSON object key order і додаткові SDK metadata не є змістовою розбіжністю.

## Контрольна точка

- Повторно зібрано шість бінарників для darwin/linux/windows × amd64/arm64. Поточний каталог містить 18 tools: 15 migration-baseline tools і три додані analyst tools; локальний MCPB smoke побачив усі 18 і виконав читання та редагування копії RDL.
- Claude Desktop і VS Code test snippets у `dist/` тепер вказують на ARM64 бінарник цього checkout.
- `mcpb/manifest.json`, локальний непідписаний `dist/rdl-mcp.mcpb` і `server.json` пройшли перевірку; `server.json` містить SHA-256 цього пакета. Пакет і бінарники в `dist/` ігноруються Git.
- Клієнтська перевірка 4.1 і встановлення MCPB у підтримуваному клієнті для 4.2 залишаються відкритими: виклики в Claude Desktop та VS Code/Copilot UI не підтверджені. Попередній запуск UI automation зупинився на macOS `not allowed assistive access`.
- README тепер позначає `uvx rdl-mcp` як старий Python спосіб. Додані Go CI та tag workflow готують Go/MCPB/SHA-256/`server.json` як артефакт GitHub Actions; workflow не створює GitHub Release і не публікує registry запис.
- Новий Go release і MCP Registry запис не публікувалися. Завдання 4.5 потребує окремого рішення користувача.
- Наступна дія: перевірити пакет у Claude Desktop і VS Code/Copilot, після чого закрити 4.1/4.2; публікацію розглядати окремо.
- Прийняті розбіжності: формулювання XML parse error, припинення сеансу після невалідного JSON, форма помилки відсутнього `filepath`, форматування JSON/XML і SDK metadata. Повний синтаксичний паритет Python `re` лишається непідтвердженим ризиком.

## Відкат

- Попередні опубліковані релізи на PyPI не видалялися. Відновіть checkout із ревізії, що містить попередню реалізацію, і поверніть клієнтський запис `command: "uvx"`, `args: ["rdl-mcp"]`.
- Якщо потрібно відновити пакетування з цього checkout, поверніть `pyproject.toml`, `setup.py` та PyPI workflow з тієї ж ревізії. Новий Go release не публікувався.
