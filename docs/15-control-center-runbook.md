# 15. Control Center Runbook

## Цель
Операционный контроль корпоративного роя агентов: SLA, ошибки, нагрузка, стоимость, свежесть агрегатов.

## Основные endpoint'ы
- `GET /v1/admin/control-center/overview`
- `GET /v1/admin/control-center/health`
- `GET /v1/admin/control-center/slo-alerts`
- `GET /v1/admin/control-center/freshness`
- `GET /v1/admin/control-center/governance`
- `GET /v1/admin/control-center/tasks/kanban`
- `GET /v1/admin/control-center/tasks/actions`

## SLA и эскалация
- Переменная: `GOCLAW_BLOCKED_ESCALATION_SEC` (по умолчанию `1800` сек).
- Логика: задача в `blocked` с `blocked_at` старше порога получает `escalated_at` + `escalation_reason=blocked_threshold`.
- Ручные действия оператора пишутся в `team_task_operator_actions`.

## Rollups и свежесть
- Материализованные представления:
  - `cc_agent_trace_rollup`
  - `cc_team_task_rollup`
- Фоновое обновление включено в managed-инстансе:
  - `GOCLAW_ROLLUP_REFRESH_SEC` (по умолчанию `60`).
- Таблица состояния обновлений: `control_center_rollup_state`.
- Проверка свежести: `GET /v1/admin/control-center/freshness`.

## SLO Alerts
- Endpoint: `GET /v1/admin/control-center/slo-alerts`.
- Тюнинг через env:
  - `GOCLAW_SLO_P95_MS` (по умолчанию `5000`)
  - `GOCLAW_SLO_ERROR_RATE` (по умолчанию `0.03`)

## Нагрузочный профиль
- Сценарий: `benchmarks/k6/control_center.js`.
- Запуск:
  - `make bench-control-center BASE_URL=http://127.0.0.1:8080 GATEWAY_TOKEN=<token> VUS=30 DURATION=90s`
- Результаты складываются в `benchmarks/results/<timestamp>`.

## Инцидентный сценарий
1. Проверить `swarm-health` и `slo-alerts`.
2. Если деградация по latency: проверить `runs/live`, `traces`, горячие endpoint'ы из k6 summary.
3. Если рост ошибок: открыть `governance`, отфильтровать access/policy нарушения.
4. Если застревание задач: `tasks/kanban` + массовые операции `pause/reassign/escalate`.
5. Проверить `freshness`; при stale выполнить ручной refresh materialized views.

## Ручной refresh rollups
```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY cc_agent_trace_rollup;
REFRESH MATERIALIZED VIEW CONCURRENTLY cc_team_task_rollup;
INSERT INTO control_center_rollup_state(name,last_refresh_at)
VALUES ('cc_agent_trace_rollup', NOW()), ('cc_team_task_rollup', NOW())
ON CONFLICT (name) DO UPDATE SET last_refresh_at = EXCLUDED.last_refresh_at;
```
