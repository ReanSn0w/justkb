# MCP: параметры и ограничения

Источник реализации: `internal/mcpserver`, доменные сервисы `cards`, `board`, `comments`, `archive`. Все идентификаторы — UUID, кроме явно отмеченных альтернатив. Scopes независимы: для обычного агента нужны `cards:read` и `cards:write`. `get_current_user` требует только валидный токен. Чтение требует read, изменения — write; обновление и восстановление проекта дополнительно требуют admin.

| Инструмент          | Параметры и правила                                                                                              | Результат / порядок                                                               |
| ------------------- | ---------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| get_current_user    | без параметров                                                                                                   | user, groups, token_id, token_scopes                                              |
| list_columns        | без параметров                                                                                                   | массив по позиции и ID                                                            |
| list_epics          | include_archived=false                                                                                           | массив проектов по имени                                                          |
| get_epic            | epic_id                                                                                                          | проект, version, restorable_cards                                                 |
| create_epic         | name (не пустое), description?, color=#00d1b2 (#RRGGBB)                                                          | новый активный проект                                                             |
| update_epic         | epic_id, expected_version>=1, name; description?, color=#00d1b2, is_archived=false, restore_mode?                | полная замена состояния проекта; при восстановлении project_only / with_cards     |
| restore_epic        | epic_id, expected_version>=1, mode=project_only/with_cards                                                       | восстановленный проект                                                            |
| list_groups         | без параметров                                                                                                   | массив по имени                                                                   |
| list_users          | group_id?, include_inactive=false                                                                                | массив по отображаемому имени; без секретов                                       |
| list_tags           | без параметров                                                                                                   | массив по имени                                                                   |
| list_cards          | epic_id?, group_id?, assignee_id?, tag_ids?, column_id?, query?, archived=false, limit?, cursor?                 | items, next_cursor; позиция столбца, числовая позиция карточки, ID по возрастанию |
| get_card            | card_id или card_number; при обоих приоритет card_id                                                             | карточка; допускается UUID, JKB-42 или числовой номер                             |
| get_card_activity   | card_id, limit?                                                                                                  | массив последних событий по времени и ID убывающе; без cursor                     |
| list_card_movements | from?, to?, actor_user_id?, assignee_id?, project_id?, group_id?, card_id?, limit?, cursor?                      | period_from, period_to, items, next_cursor; время до секунды и ID убывающе        |
| list_comments       | ровно одно непустое card_id / project_id                                                                         | массив по времени и ID возрастающе; без пагинации                                 |
| add_comment         | ровно одно card_id / project_id, body (не пустой Markdown)                                                       | комментарий; архивные цели read-only                                              |
| create_card         | title (не пустой), column_id; description?, epic_id?, group_id?, assignee_id? / assignee_ids?, tag_ids?, due_at? | карточка первой в столбце; архивный проект недопустим                             |
| update_card         | card_id, expected_version>=1; title?, description?, epic_id?, group_id?, assignee_id? / assignee_ids?, tag_ids?  | обновлённая карточка с новой version; архив read-only                             |
| assign_card         | card_id, expected_version>=1; group_id?, assignee_id? / assignee_ids?                                            | обновлённая карточка; архив read-only                                             |
| move_card           | card_id, expected_version>=1, target_column_id; before_card_id? / after_card_id?                                 | карточка; ссылки только на карточки назначения, обе одновременно запрещены        |
| archive_card        | card_id, expected_version>=1                                                                                     | карточка после архивирования                                                      |
| restore_card        | card_id, expected_version>=1                                                                                     | карточка после восстановления; проект должен быть активен                         |
| list_archive        | type?, project_id?, q?, from?, to?, state?, history=false, batch_id?, limit?, cursor?                            | items, next_cursor; время и ID убывающе                                           |

## Лимиты и обход страниц

| Инструмент          | Default | Максимум | Ноль и отрицательное значение                 |
| ------------------- | ------- | -------- | --------------------------------------------- |
| list_cards          | 50      | 100      | используют default                            |
| get_card_activity   | 50      | 100      | используют default                            |
| list_card_movements | 100     | 500      | используют default                            |
| list_archive        | 30      | 100      | 0 использует default, отрицательное запрещено |

Для страниц копируйте непрозрачный `next_cursor` в следующий запрос с теми же фильтрами. Завершайте обход при пустом/отсутствующем cursor. Увеличение limit выше максимума не заменяет пагинацию. Справочники и комментарии возвращают массивы без курсоров. Activity возвращает только последние события и не позволяет получить полную историю свыше 100 записей.

Для отчёта задайте фиксированные `from` и `to` в RFC3339 с часовым поясом. Интервал `[from, to)`. Без to используется текущее время, без from — to минус 24 часа. После первого запроса можно использовать его `period_from` / `period_to` для следующих страниц. Движения не включают создание карточек; текущие исполнители, проект, группа и названия не являются историческими снимками. Пагинация не обеспечивает снимок БД при параллельных изменениях.

Пример: `list_card_movements({"from":"2026-11-25T00:00:00+03:00","to":"2026-11-26T00:00:00+03:00","limit":500})`; затем повторяйте эти аргументы с полученным cursor, пока он не опустеет.

## Семантика полей

- В list_cards значения epic_id/group_id=`none` выбирают неназначенные карточки; assignee_id принимает UUID, `none`, `me`. Все tag_ids должны присутствовать у карточки. query ищет номер или подстроку заголовка/описания. archived выбирает одно состояние, не оба.
- В движениях actor_user_id — инициатор, assignee_id — любой текущий исполнитель; оба принимают `me`. Для всей доски не передавайте эти фильтры.
- Для update_card/assign_card отсутствующее поле сохраняет значение. Пустые epic_id/group_id снимают связь, пустое description очищает текст. null для этих полей эквивалентен отсутствию. assignee_id=null или пустая строка снимает основного, сохраняя дополнительных; assignee_ids=[] снимает всех. assignee_id и assignee_ids вместе запрещены. Пустой tag_ids очищает теги; null сохраняет. Для создания null/отсутствие опционального поля означает отсутствие значения.
- due_at принимает RFC3339 при создании; update_card не поддерживает его изменение. Строки title/name/body обрезаются по краям и не могут состоять из пробелов. Дополнительных продуктовых maxLength в этих сервисах нет; ограничения транспортного тела не являются лимитом числа записей.
- Архив: type — card/project, state — archived/restored/all; пустое значение означает default. history=false показывает текущий архив, history=true включает прошлые события. state фильтрует текущее состояние сущности. batch_id раскрывает карточки пачки. q ищет по заголовку/номеру. from/to — RFC3339, границы опциональны, при обеих from<to.
- update_epic заменяет полное состояние: отсутствие description очищает его, отсутствие is_archived означает false; перед обновлением перечитайте проект. restore_mode нужен при восстановлении. Для обычного восстановления удобнее restore_epic.

## Ошибки и повторные запросы

Schema validation отклоняет неверные типы и диапазоны до вызова доменного сервиса и возвращает MCP isError с указанием поля и ограничения. Доменная validation возвращает `validation_failed`; для лимита details содержит field, minimum, maximum, default и правило default. Это два слоя проверки, а не обещание одинакового текста ошибок SDK и сервиса.

Исправьте limit и повторите чтение; неверный cursor замените новым обходом с начала. При неверных датах исправьте формат/границы. При insufficient_scope нужен токен с указанным scope; forbidden требует подходящей роли. При card_version_conflict/project_version_conflict/archive_version_conflict перечитайте сущность и пересмотрите изменение, не повторяйте старую мутацию вслепую. Архивные сущности требуют восстановления. database_busy допускает повтор чтения; после неоднозначного сетевого сбоя мутации сначала проверьте состояние.

Выходные значения обработчиков динамические (`any`), поэтому сервер не объявляет фиктивную outputSchema. Используйте фактические DTO выше и structuredContent там, где оно предоставляется SDK; массивы также представлены JSON в text content. Все операции чтения помечены readOnlyHint; annotations не заменяют серверные проверки прав.
