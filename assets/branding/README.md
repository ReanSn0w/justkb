# Иконка JKB

`kanban-icon.png` — выбранный пользователем вариант: три заполненных столбца разной высоты, над каждым — скруглённый прямоугольник без заливки. Это сохранённая копия `kanban-icon-concept-v3.png`; промпт редактирования находится в `icon-v3-prompt.txt`.

Этот PNG служит визуальным образцом. Геометрия реализована в `internal/frontend/branding.go`: сервер формирует SVG, PNG и ICO с цветом фона из `JKB_ICON_COLOR`. Синие пиксели эскиза не перекрашиваются, поэтому на сглаженных краях не остаётся исходного цвета. Выполненный план находится в `plans/done/navigation-transitions-and-mobile-app.md`.

## Предыдущие варианты

`kanban-icon-concept.png` — первый эскиз, созданный встроенным imagegen; он не выбран для приложения. Растровые эскизы сохранены как история разработки. Генерация параметризованных иконок описана в `plans/done/navigation-transitions-and-mobile-app.md`.

## Промпт первого варианта

Use case: logo-brand. Create one finished square app icon for JKB, a Kanban productivity web app. Trello-inspired visual simplicity and clarity, but an original three-column Kanban mark, not the Trello two-panel logo. Flat vector-like design, precisely aligned bold white rounded rectangular task cards arranged in three vertical columns with distinct lengths, compact balanced silhouette, instantly legible at favicon size. Solid rich cobalt blue #2563EB full-bleed square background, single white foreground, no gradients, shadows, outlines, textures, lettering, border, outer rounded corners or mockup. All mark geometry inside central 65 percent so mobile OS masks cannot crop it. Output just the single icon, front facing, no presentation sheet. The design will later be recreated as SVG with configurable background color.
