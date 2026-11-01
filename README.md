# JKB

JKB — компактная однодосочная Kanban-система для небольшой команды. Она состоит
из одного Go-приложения со встроенным Vue-интерфейсом, PostgreSQL и локального
каталога для файлов. Дополнительно приложение предоставляет HTTP API и remote
MCP endpoint для работы AI-агентов.

Проект сознательно не пытается быть ещё одной Taiga или Jira: одна доска,
минимум инфраструктуры и никаких обязательных S3, Redis, очередей или отдельных
frontend-контейнеров.

## Возможности

- одна Kanban-доска с настраиваемыми колонками и drag-and-drop;
- проекты для группировки и фильтрации задач;
- группы пользователей, исполнители и теги;
- описания и комментарии с безопасным Markdown;
- вложения к задачам и проектам в локальном хранилище;
- профили с отображаемым именем и аватаром;
- роли `admin` и `member`;
- персональные API-токены с отдельными правами чтения и изменения;
- remote MCP для агентов;
- история изменений карточек и optimistic locking;
- health checks, явные миграции и административный CLI;
- создание и восстановление согласованных резервных копий из панели администратора.

## Границы проекта

- В системе одна доска. Проекты не создают отдельные доски, а используются как
  фильтр и логическая область задач.
- Самостоятельной регистрации нет. Пользователей создаёт администратор через
  CLI или административный интерфейс.
- PostgreSQL — единственная внешняя служба.
- Вложения и аватары хранятся на локальном диске. Для нескольких реплик
  понадобится общее файловое хранилище или отдельная реализация storage layer.
- HTTPS завершается на reverse proxy перед приложением.

## Архитектура

```text
Браузер ───────┐
               ├──> JKB (Go HTTP server) ───> PostgreSQL
MCP-клиент ────┘          │
                          └───────────────> локальное хранилище файлов
```

Frontend собирается Vite и встраивается в Go-бинарник. В production достаточно
контейнера JKB, PostgreSQL и volume для файлов.

Основной стек:

- Go 1.26;
- Vue 3, TypeScript, Pinia и Vue Router;
- Bulma без собственного CSS;
- PostgreSQL 17;
- официальный Go SDK для MCP.

## Быстрый запуск через Docker Compose

Понадобятся Docker и Docker Compose.

1. Подготовьте конфигурацию:

   ```sh
   cp .env.example .env
   ```

   Задайте в `.env` длинный случайный `POSTGRES_PASSWORD`. Для публичного
   развёртывания оставьте `JKB_COOKIE_SECURE=true`.

2. Соберите приложение и запустите PostgreSQL:

   ```sh
   docker compose build
   docker compose up -d db
   ```

3. Примените миграции:

   ```sh
   docker compose run --rm app /usr/local/bin/jkb-migrate up
   ```

4. Создайте первого администратора:

   ```sh
   docker compose run --rm app /usr/local/bin/jkb-admin user add \
     --role admin \
     --display-name "Administrator" \
     admin
   ```

   Команда интерактивно запросит пароль и его подтверждение. Пароль не
   передаётся через аргументы и не выводится в терминал.

5. Запустите приложение:

   ```sh
   docker compose up -d app
   ```

По умолчанию JKB будет доступен на [http://localhost:8080](http://localhost:8080).

Проверить состояние контейнеров:

```sh
docker compose ps
curl --fail http://localhost:8080/health/ready
```

## Административный CLI

`jkb-admin` предназначен для bootstrap и операций, которые не требуют отдельного
веб-интерфейса.

Создание обычного пользователя:

```sh
docker compose run --rm app /usr/local/bin/jkb-admin user add \
  --display-name "Ada Lovelace" \
  --email ada@example.test \
  ada
```

Роль по умолчанию — `member`. Для администратора передайте `--role admin`.
Отображаемое имя по умолчанию совпадает с логином, email необязателен.
Сброс пароля с отзывом всех web-сессий пользователя:

```sh
docker compose run --rm app /usr/local/bin/jkb-admin user password admin
```

Просмотр профиля и изменение активности:

```sh
docker compose run --rm app /usr/local/bin/jkb-admin user show admin
docker compose run --rm app /usr/local/bin/jkb-admin user disable alice
docker compose run --rm app /usr/local/bin/jkb-admin user enable alice
```

`user show` выводит ID, логин, отображаемое имя, email, роль, активность и
группы. Хеш пароля, сессии и API-токены не выводятся. Отключить последнего
активного администратора нельзя.

Справка:

```sh
docker compose run --rm app /usr/local/bin/jkb-admin --help
docker compose run --rm app /usr/local/bin/jkb-admin user add --help
docker compose run --rm app /usr/local/bin/jkb-admin user password --help
docker compose run --rm app /usr/local/bin/jkb-admin user show --help
```

## Remote MCP

MCP endpoint находится по адресу:

```text
https://jkb.example.com/mcp
```

Персональный токен выпускается в настройках профиля на вкладке «API-токены».
Клиент должен передавать его в HTTP-заголовке:

```http
Authorization: Bearer jkb_pat_…
```

Доступные scopes:

- `cards:read` — чтение доски, справочников, проектов, задач и комментариев;
- `cards:write` — создание и изменение задач, комментариев и связанных данных.

Инструмент `list_card_movements` позволяет агенту подготовить отчёт о переходах карточек между
колонками за предыдущие 24 часа или произвольный период. Без фильтров отчёт включает всех
пользователей; при необходимости его можно ограничить инициатором, текущим исполнителем, проектом,
группой или конкретной карточкой.

Не добавляйте токены в URL, Git-репозиторий или общие конфигурационные файлы.
Полный перечень MCP tools и формат ошибок описаны в [CONTRACT.md](CONTRACT.md#5-mcp-api).

## HTTP API

- REST API: `/api/v1`;
- remote MCP: `/mcp`;
- liveness: `/health/live`;
- readiness: `/health/ready`.

Веб-интерфейс и REST API используют cookie-сессию и CSRF-защиту. Персональные
API-токены используются MCP endpoint. Контракт сущностей, REST endpoints, ролей
и ошибок находится в [CONTRACT.md](CONTRACT.md).
Агент может определить владельца токена и его группы через `get_current_user`, а также
читать обсуждения карточек и проектов через `list_comments`.

## Конфигурация

### Переменные приложения

| Переменная                 | Обязательна | Значение по умолчанию | Назначение                                   |
| -------------------------- | ----------: | --------------------- | -------------------------------------------- |
| `JKB_DATABASE_URL`         |          да | —                     | PostgreSQL connection URL                    |
| `JKB_STORAGE_PATH`         |         нет | `./storage`           | каталог вложений и аватаров                  |
| `JKB_SYSTEM_NAME`          |         нет | `JKB`                 | имя системы в web-интерфейсе                 |
| `JKB_LISTEN_ADDRESS`       |         нет | `:8080`               | адрес HTTP-сервера                           |
| `JKB_PUBLIC_URL`           |         нет | —                     | зарезервированный внешний origin приложения  |
| `JKB_COOKIE_SECURE`        |         нет | `true`                | отправлять session cookie только через HTTPS |
| `JKB_ATTACHMENT_MAX_BYTES` |         нет | `26214400`            | максимальный размер вложения                 |
| `JKB_SHUTDOWN_TIMEOUT`     |         нет | `15s`                 | таймаут graceful shutdown                    |

Аватар ограничен 5 МБ независимо от `JKB_ATTACHMENT_MAX_BYTES`.
`JKB_SYSTEM_NAME=GostAssistent` меняет название в навигации, на экране
входа и в заголовке вкладки. Технические идентификаторы (`JKB-42`,
`jkb_pat_...`, имена cookie и backup-файлов) остаются стабильными.

### Переменные Compose

| Переменная          | Значение по умолчанию   | Назначение                    |
| ------------------- | ----------------------- | ----------------------------- |
| `POSTGRES_PASSWORD` | —                       | пароль PostgreSQL; обязателен |
| `JKB_PORT`          | `8080`                  | порт приложения на хосте      |
| `JKB_SYSTEM_NAME`   | `JKB`                   | имя системы в web-интерфейсе  |
| `JKB_PUBLIC_URL`    | `http://localhost:8080` | зарезервированный внешний URL |
| `JKB_COOKIE_SECURE` | `true`                  | secure-флаг cookie            |

## Обновление

Перед обновлением сделайте резервную копию, затем:

```sh
docker compose build
docker compose run --rm app /usr/local/bin/jkb-migrate up
docker compose up -d app
```

Миграции выполняются отдельно и не запускаются скрыто при старте приложения.
Это позволяет контролировать порядок deployment. `jkb-migrate down` используйте
только после проверки конкретной rollback-миграции и резервной копии.

## Резервное копирование

Администратор может скачать полный архив или восстановить его на вкладке
«Резервные копии» в системных настройках. Архив содержит data-only дамп
PostgreSQL, manifest с версией схемы, вложения и аватары. Во время snapshot и
restore приложение временно блокирует остальные запросы. Восстановление
принимает только backup с той же версией схемы и предварительно создаёт
внутренний rollback-снимок.

Для внешнего инфраструктурного backup можно по-прежнему копировать оба volume
вручную. PostgreSQL и volume с файлами образуют единый логический snapshot. На
время копирования желательно остановить запись:

```sh
docker compose stop app
docker compose exec -T db pg_dump -U jkb -d jkb -Fc > jkb.dump
docker compose run --rm --no-deps -v "$PWD":/backup app \
  tar -C /var/lib/jkb/attachments -czf /backup/jkb-attachments.tar.gz .
docker compose start app
```

Восстанавливайте базу и файлы в пустое окружение до запуска `app`. Резервная
копия, восстановление которой никогда не проверяли, — это, строго говоря,
довольно оптимистичная гипотеза.

## Локальная разработка

Требования:

- Go 1.26;
- Node.js 24 и npm;
- PostgreSQL 17 или Docker Compose.

Установить frontend-зависимости:

```sh
npm ci
```

Для разработки интерфейса можно оставить backend в Compose и запустить Vite:

```sh
docker compose up -d db app
npm run dev
```

Vite проксирует `/api` и `/mcp` на `http://localhost:8080`.

Для локального запуска Go-приложения задайте `JKB_DATABASE_URL`, примените
миграции и отключите secure cookie только для HTTP-разработки:

```sh
go run ./cmd/jkb-migrate up
JKB_COOKIE_SECURE=false go run ./cmd/jkb
```

`JKB_DATABASE_URL` лучше передавать через локальное окружение или менеджер
секретов, а не записывать в команды и shell history.

Production frontend собирается и встраивается автоматически через Dockerfile.
Для ручной сборки единого бинарника:

```sh
npm run build
go build ./cmd/jkb
```

## Проверки

Backend:

```sh
go test ./...
```

Frontend:

```sh
npm run lint
npm run typecheck
npm test
npm run build
```

Integration-тесты с PostgreSQL запускаются при наличии
`JKB_TEST_DATABASE_URL`. Тестовая база очищается, поэтому никогда не указывайте
production database.

## Структура репозитория

```text
cmd/                    исполняемые Go-команды
  jkb/                  HTTP-сервер
  jkb-admin/            административный CLI
  jkb-migrate/          миграции
internal/               backend-пакеты и встроенный frontend build
migrations/             SQL-миграции Goose
src/                    Vue-приложение
CONTRACT.md             контракт продукта, REST и MCP
compose.yaml            локальное и компактное production-развёртывание
```

## Ориентир по ресурсам

В smoke-тесте на arm64 от 31 октября 2026 года сразу после запуска приложение
использовало около 72 МиБ памяти, PostgreSQL — около 28 МиБ. Это ориентир, а не
SLA: расход зависит от архитектуры, кэша PostgreSQL, allocator и реальной
нагрузки.
