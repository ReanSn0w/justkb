# JKB — контракт продукта и API

Этот документ фиксирует границы MVP, модель данных, HTTP API и MCP API. Если реализация и контракт расходятся, контракт считается источником истины до явного изменения.

## 1. Границы MVP

JKB — одна общая kanban-доска для небольшой команды.

В MVP входят:

- один глобальный набор колонок;
- проекты как крупные направления работы;
- карточки с группой, исполнителем, тегами, сроком и вложениями;
- фильтрация по проекту, группе, исполнителю, тегам, колонке и тексту;
- локальное хранение вложений на диске сервера;
- веб-интерфейс с сессионной авторизацией;
- персональные API-токены;
- remote MCP endpoint для работы агентов с карточками;
- журнал изменений карточек.

В MVP не входят несколько досок, уведомления, комментарии, OAuth-провайдер, публичные доски и объектное хранилище.

## 2. Термины и права

### 2.1. Роли

- `admin` управляет пользователями, группами, колонками, проектами и тегами;
- `member` читает всю доску и работает с карточками.

Группы — способ классификации и фильтрации, а не граница видимости. Авторизованный пользователь видит все карточки. Исполнитель не обязан состоять в назначенной карточке группе.

### 2.2. Состояние пользователя

Неактивный пользователь не может войти, использовать сессию или API-токен. Деактивация не удаляет его из истории и не снимает автоматически с карточек.

## 3. Модель данных

Все идентификаторы во внешних API — UUID. Время передаётся в UTC в формате RFC 3339. Поля `created_at` и `updated_at` присутствуют у всех изменяемых сущностей.

### 3.1. Основные сущности

| Сущность | Обязательные поля | Опциональные поля |
|---|---|---|
| User | `id`, `username`, `display_name`, `role`, `is_active` | `email`, `avatar_url` |
| Group | `id`, `name`, `color` | `description` |
| Column | `id`, `name`, `position` | `wip_limit` |
| Project (`Epic` в API) | `id`, `name`, `color`, `is_archived`, `card_count` | `description`, `attachments` |
| Tag | `id`, `name`, `color` | — |
| Card | `id`, `number`, `title`, `description`, `column_id`, `position`, `version`, `is_archived` | `epic_id`, `group_id`, `assignee_id`, `due_at` |
| Attachment | `id`, `original_name`, `media_type`, `size`, `created_by` | `card_id` или `epic_id` |
| Comment | `id`, `body`, `author_user_id`, `created_at`, `updated_at` | `card_id` или `epic_id`, `author_token_id` |
| API token | `id`, `user_id`, `name`, `token_prefix`, `scopes` | `expires_at`, `last_used_at`, `revoked_at` |
| Activity | `id`, `card_id`, `actor_user_id`, `action`, `changes` | `actor_token_id` |

`Card.number` — неизменяемый человекочитаемый номер. Карточка показывается как `JKB-42`.

`Card.version` увеличивается при каждом изменении. Мутирующие операции принимают `expected_version` и возвращают конфликт, если карточка уже изменена.

### 3.2. Связи

- пользователь может состоять в нескольких группах;
- карточка имеет не более одного проекта, одной группы и одного исполнителя;
- карточка может иметь несколько тегов и вложений;
- проект имеет подробное Markdown-описание и может содержать несколько вложений;
- вложение принадлежит ровно одной карточке или одному проекту;
- комментарий принадлежит ровно одной карточке или одному проекту и поддерживает Markdown;
- архивный проект остаётся у существующих карточек, но не предлагается для новых;
- удаление колонки с карточками запрещено до перемещения её карточек.

## 4. HTTP API

### 4.1. Общие правила

- базовый путь: `/api/v1`;
- тела запросов и ответов: JSON, кроме загрузки и скачивания файлов;
- веб-клиент аутентифицируется через `HttpOnly`, `Secure`, `SameSite=Lax` cookie;
- изменяющие cookie-авторизованные запросы защищаются CSRF-токеном;
- списки используют cursor pagination с `limit` не более 100;
- `DELETE` для токенов и вложений возвращает `204 No Content`;
- архивирование карточек и проектов является обратимой мутацией, а не `DELETE`.

Формат успешного списка:

```json
{
  "items": [],
  "next_cursor": null
}
```

Формат ошибки:

```json
{
  "error": {
    "code": "card_version_conflict",
    "message": "Card was changed by another actor",
    "details": {},
    "request_id": "01J..."
  }
}
```

Коды ошибок стабильны и пригодны для машинной обработки. Ожидаемые статусы: `400`, `401`, `403`, `404`, `409`, `413`, `422`, `429`, `500`.

### 4.2. Auth и профиль

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/auth/login` | Создать веб-сессию |
| `POST` | `/auth/logout` | Завершить текущую сессию |
| `GET` | `/auth/me` | Получить профиль, группы и CSRF-токен |
| `PATCH` | `/auth/profile` | Изменить своё отображаемое имя |
| `POST` | `/auth/avatar` | Загрузить свой аватар (`multipart/form-data`, поле `file`) |
| `DELETE` | `/auth/avatar` | Удалить свой аватар |
| `PUT` | `/auth/password` | Сменить свой пароль |

### 4.3. Управление

| Ресурс | Пути | Доступ |
|---|---|---|
| Users | `GET, POST /users`; `GET, PATCH /users/{id}`; `PUT /users/{id}/groups`; `PUT /users/{id}/password` | Чтение — все, изменение — admin |
| Groups | `GET, POST /groups`; `GET, PATCH /groups/{id}` | Чтение — все, изменение — admin |
| Columns | `GET, POST /columns`; `PATCH /columns/{id}`; `POST /columns/reorder`; `DELETE /columns/{id}` | Чтение — все, изменение — admin |
| Projects (`epics`) | `GET, POST /epics`; `GET, PATCH /epics/{id}`; `POST /epics/{id}/attachments` | Чтение — все, изменение — admin; вложения — все |
| Tags | `GET, POST /tags`; `GET, PATCH /tags/{id}` | Чтение — все, изменение — admin |
| Backups | `POST /admin/backups`; `POST /admin/backups/restore` | Только admin |

Первый admin создаётся после миграций командой `jkb-admin user add --role admin --display-name "Administrator" admin`. Эта же команда создаёт обычных пользователей (`member` по умолчанию); пароль вводится и подтверждается интерактивно. Сервер не имеет общедоступного setup endpoint.

`GET /users/{id}/avatar` возвращает аватар авторизованному участнику. Аватары хранятся локально в `avatars` внутри каталога хранилища; принимаются JPEG, PNG, GIF и WebP размером до 5 МБ.

`POST /admin/backups` возвращает `tar.gz` с manifest, data-only дампом PostgreSQL и локальными файлами. `POST /admin/backups/restore` принимает этот архив в `multipart/form-data` (`file`, `confirmation=RESTORE`) и заменяет текущие данные. Restore разрешён только при совпадении версии схемы. Операции выполняются под maintenance-lock; перед restore сервис создаёт внутренний rollback-снимок текущей базы и сохраняет прежний каталог файлов до успешного завершения.

### 4.4. Карточки

| Метод | Путь | Назначение |
|---|---|---|
| `GET` | `/cards` | Список и фильтрация |
| `POST` | `/cards` | Создать карточку |
| `GET` | `/cards/{id}` | Получить карточку с тегами и вложениями |
| `PATCH` | `/cards/{id}` | Изменить поля карточки |
| `POST` | `/cards/{id}/move` | Изменить колонку и/или позицию |
| `POST` | `/cards/{id}/archive` | Архивировать |
| `POST` | `/cards/{id}/restore` | Восстановить |
| `GET` | `/cards/{id}/activity` | Получить журнал изменений |

Фильтры `GET /cards`:

- `epic_id`, включая специальное значение `none`;
- `group_id`, включая `none`;
- `assignee_id`, включая `me` и `none`;
- повторяемый `tag_id`;
- `column_id`;
- `q` для поиска по номеру, заголовку и описанию;
- `archived`, по умолчанию `false`.

Фильтры объединяются условием AND. Несколько `tag_id` означают, что у карточки должен быть каждый указанный тег.

Пример создания:

```json
{
  "title": "Implement PAT authentication",
  "description": "Store only a digest of the token.",
  "column_id": "uuid",
  "epic_id": "uuid",
  "group_id": "uuid",
  "assignee_id": null,
  "tag_ids": ["uuid"],
  "due_at": null
}
```

Пример перемещения:

```json
{
  "expected_version": 7,
  "target_column_id": "uuid",
  "before_card_id": "uuid"
}
```

Вместо `before_card_id` можно передать `after_card_id`; одновременная передача обоих полей запрещена. Без них карточка становится последней в целевой колонке.

### 4.5. Вложения

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/cards/{id}/attachments` | `multipart/form-data` с одним полем `file` |
| `POST` | `/epics/{id}/attachments` | `multipart/form-data` с одним полем `file` |
| `GET` | `/attachments/{id}` | Скачать файл с проверкой авторизации |
| `DELETE` | `/attachments/{id}` | Удалить метаданные и файл |

В базе хранятся только метаданные. Имя файла на диске генерируется сервером и не зависит от пользовательского имени. Корневой каталог и максимальный размер задаются конфигурацией. Расширение и заголовок `Content-Type` не считаются достаточной проверкой типа.

### 4.6. Комментарии

| Метод | Путь | Назначение |
|---|---|---|
| `GET, POST` | `/cards/{id}/comments` | Прочитать или добавить комментарий к карточке |
| `GET, POST` | `/epics/{id}/comments` | Прочитать или добавить комментарий к проекту |

Комментарии поддерживают Markdown. В ответе возвращаются автор и, если комментарий создан агентом, `author_token_id`.

### 4.7. Персональные API-токены

| Метод | Путь | Назначение |
|---|---|---|
| `GET` | `/tokens` | Показать свои токены без секрета |
| `POST` | `/tokens` | Выпустить токен |
| `DELETE` | `/tokens/{id}` | Отозвать свой токен |

Токен имеет формат `jkb_pat_<random>` и не менее 256 бит случайности. Полное значение возвращается ровно один раз в ответе `POST /tokens`. В базе хранятся отображаемый префикс и SHA-256 digest полного токена.

Допустимые scopes:

- `cards:read`;
- `cards:write`.

`cards:write` не подразумевает `cards:read`: для обычного агента выдаются оба scope. Токен наследует роль и активность владельца. Отозванный, просроченный или принадлежащий неактивному пользователю токен недействителен.

## 5. MCP API

### 5.1. Транспорт и auth

- endpoint: `/mcp`;
- transport: stateless Streamable HTTP;
- целевая версия протокола: `2026-07-28`;
- аутентификация: `Authorization: Bearer jkb_pat_...`;
- MCP endpoint не принимает cookie-сессии;
- сервер не выдаёт OAuth-токены в MVP;
- клиент должен уметь передать статический Bearer token.

Реализация опирается на официальные [MCP specification](https://modelcontextprotocol.io/specification/2026-07-28) и [MCP Go SDK](https://go.sdk.modelcontextprotocol.io/). Если целевая версия ещё не поддержана стабильным Go SDK, сервер временно поддерживает последнюю совместимую версию, не меняя схемы инструментов.

### 5.2. Инструменты чтения

| Tool | Аргументы | Результат |
|---|---|---|
| `list_columns` | — | Все колонки по позиции |
| `list_epics` | `include_archived?` | Проекты |
| `get_epic` | `epic_id` | Проект с описанием и количеством задач |
| `list_groups` | — | Группы |
| `list_users` | `group_id?`, `include_inactive?` | Пользователи без чувствительных полей |
| `list_tags` | — | Теги |
| `list_cards` | Фильтры REST API, `limit?`, `cursor?` | Краткие DTO карточек и `next_cursor` |
| `get_card` | `card_id` или `card_number` | Полная карточка с тегами и метаданными вложений |
| `get_card_activity` | `card_id`, `limit?`, `cursor?` | Журнал карточки |
| `list_comments` | Ровно одно из `card_id`, `project_id` | Комментарии карточки или проекта |

Все инструменты чтения требуют `cards:read` и помечаются MCP annotation `readOnlyHint: true`.

`add_comment` принимает ровно одно из `card_id`, `project_id`, а также `body`, и требует `cards:write`.

`create_epic` и `update_epic` требуют `cards:write` и роль `admin` владельца токена.

### 5.3. Инструменты изменения

| Tool | Обязательные аргументы | Опциональные аргументы |
|---|---|---|
| `create_card` | `title`, `column_id` | `description`, `epic_id`, `group_id`, `assignee_id`, `tag_ids`, `due_at` |
| `update_card` | `card_id`, `expected_version`, `patch` | — |
| `move_card` | `card_id`, `expected_version`, `target_column_id` | `before_card_id` или `after_card_id` |
| `assign_card` | `card_id`, `expected_version` | `group_id`, `assignee_id`; `null` снимает назначение |
| `archive_card` | `card_id`, `expected_version` | — |
| `restore_card` | `card_id`, `expected_version` | — |

Все инструменты изменения требуют `cards:write`. Успешная мутация возвращает полную обновлённую карточку, включая новую `version`.

MCP-инструменты вызывают тот же доменный слой, что и HTTP API. Они не дублируют бизнес-правила и не обходят журнал изменений. Каждая MCP-мутация сохраняет `actor_user_id` и `actor_token_id`.

### 5.4. MCP-ошибки

Ожидаемые ошибки возвращаются как ошибка tool result с машинными `code` и `details`. Минимальный набор:

- `unauthenticated`;
- `insufficient_scope`;
- `not_found`;
- `validation_failed`;
- `card_version_conflict`;
- `rate_limited`;
- `internal_error`.

При `card_version_conflict` в `details` возвращаются текущие `version` и карточка, чтобы агент мог перечитать состояние и осознанно повторить операцию.

## 6. Хранение и развёртывание

- приложение работает одним Go-процессом;
- собранный Vue frontend встраивается в Go-бинарник;
- PostgreSQL — единственный внешний сервис;
- вложения хранятся в отдельном persistent volume;
- HTTP API, frontend и MCP слушают один порт;
- для публичного доступа обязателен TLS на reverse proxy;
- `/health/live` проверяет жизнь процесса, `/health/ready` — доступность PostgreSQL и каталога вложений.

Резервная копия полна только при совместном сохранении PostgreSQL и volume вложений.

## 7. Нефункциональные требования

- пароли хешируются Argon2id;
- сессии и API-токены можно отозвать без перезапуска;
- в логи не попадают пароли, cookie, Authorization header и тела файлов;
- login и неуспешная Bearer-аутентификация имеют rate limiting;
- мутация карточки и запись activity выполняются в одной транзакции;
- тихая потеря изменений при конкурентном редактировании недопустима;
- сервис должен корректно завершать активные HTTP-запросы при graceful shutdown;
- миграции базы версионируются и выполняются явно перед запуском новой версии.
