# TASKS

## Итерация 1 (MVP Control Center)

1. [x] Уточнить контракт `control-center` API (overview/agents/runs/tasks).
2. [x] Зафиксировать роли и матрицу прав для UI и HTTP (`admin/operator/viewer`).
3. [x] Добавить endpoint `GET /v1/admin/control-center/overview` с агрегатами.
4. [x] Добавить endpoint `GET /v1/admin/control-center/agents` с фильтрами и пагинацией.
5. [x] Добавить endpoint `GET /v1/admin/control-center/runs/live` для активных запусков.
6. [x] Добавить endpoint `GET /v1/admin/control-center/tasks/kanban` с колонками и счетчиками.
7. [x] Реализовать индексы в БД под новые выборки (agents/runs/tasks/delegations).
8. [x] Добавить серверные тесты на RBAC для всех новых endpoint.
9. [x] Подключить `Overview` к `overview` endpoint без fallback-расхождений.
10. [x] Сделать страницу `Fleet Overview` (агенты, статус, активность, ошибки, последние действия).
11. [x] Сделать route-guard по ролям для новых страниц control center.
12. [x] Добавить e2e smoke-тест на авторизацию и загрузку control center.

## Итерация 2 (Operations)

13. [x] Добавить страницу `Operations` с live-runs таблицей и действиями (`retry/abort`).
14. [x] Добавить поток WS-событий `run.updated` и `task.updated`.
15. [x] Реализовать клиентский store для realtime-обновлений control center.
16. [x] Добавить страницу `Kanban` с колонками: Backlog/Assigned/In Progress/Review/Done/Blocked.
17. [x] Добавить массовые операции в Kanban (reassign/pause/escalate).
18. [x] Ввести SLA-поля задачи и авто-эскалацию при `Blocked > threshold`.
19. [x] Добавить журнал действий оператора (кто и что изменил по задачам).
20. [x] Добавить интеграционные тесты на lifecycle задачи и эскалации.

## Итерация 3 (Enterprise)

21. [x] Добавить страницу `Governance` (policy alerts, approval queue, access violations).
22. [x] Добавить страницу `Knowledge` (источники, freshness, coverage gaps).
23. [x] Реализовать `Delegation Map` (граф передач задач между агентами).
24. [x] Добавить cost-аналитику по агентам/командам/каналам.
25. [x] Ввести health score роя (ошибки, latency, SLA, stuck tasks).
26. [x] Вынести тяжелые агрегаты в materialized views/rollups.
27. [x] Добавить фоновые обновления агрегатов и метрики свежести.
28. [x] Настроить алерты SLO (p95/error rate/queue age/cost anomalies).
29. [x] Провести нагрузочное профилирование control-center endpoint’ов.
30. [x] Зафиксировать runbook эксплуатации и инцидентные сценарии.

## Критерии готовности (Definition of Done)

31. Все endpoint покрыты RBAC-тестами и контрактными тестами.
32. UI не показывает недоступные разделы и блокирует прямой переход по URL.
33. p95 для `overview` и `agents list` укладывается в целевой SLO.
34. Все ключевые таблицы имеют индексы и explain-plan без full scan на hot-path.
35. Есть документация API и операторский runbook в репозитории.
