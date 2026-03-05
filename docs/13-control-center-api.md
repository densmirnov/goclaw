# Control Center API (Managed)

## Назначение

Набор агрегированных endpoint для админского центра управления корпоративным роем агентов.

Базовый префикс: `/v1/admin/control-center`

Требование по роли: `admin`.

## Endpoint

1. `GET /v1/admin/control-center`
2. `GET /v1/admin/control-center/overview`
3. `GET /v1/admin/control-center/agents`
4. `GET /v1/admin/control-center/runs/live`
5. `GET /v1/admin/control-center/tasks/kanban`

## Контракты

### 1) Overview

`GET /v1/admin/control-center/overview`

Response:

```json
{
  "agents": [
    {
      "id": "uuid",
      "agent_key": "sales-bot",
      "display_name": "Sales Bot",
      "status": "active",
      "owner_id": "user-123",
      "last_action": "delegated lead scoring"
    }
  ],
  "channel_total": 12,
  "channel_enabled": 9,
  "errors": [
    {
      "id": "trace-uuid",
      "agent_id": "agent-uuid",
      "name": "handoff run",
      "error": "timeout",
      "created_at": "2026-03-05T18:00:00+07:00"
    }
  ],
  "recent_actions": [
    {
      "id": "trace-uuid",
      "agent_id": "agent-uuid",
      "name": "resolve ticket #223",
      "status": "completed",
      "created_at": "2026-03-05T18:01:00+07:00"
    }
  ]
}
```

### 2) Agents

`GET /v1/admin/control-center/agents?limit=50&offset=0&search=&status=&owner_id=`

Response:

```json
{
  "agents": [],
  "total": 0,
  "limit": 50,
  "offset": 0,
  "filters": {
    "search": "",
    "status": "",
    "owner_id": ""
  }
}
```

### 3) Live Runs

`GET /v1/admin/control-center/runs/live?limit=100`

Response:

```json
{
  "runs": [
    {
      "id": "trace-uuid",
      "agent_id": "agent-uuid",
      "user_id": "user-123",
      "session_key": "agent:...",
      "name": "process inbox",
      "channel": "telegram",
      "status": "running",
      "start_time": "2026-03-05T18:05:00+07:00"
    }
  ],
  "total": 1,
  "limit": 100
}
```

### 4) Kanban

`GET /v1/admin/control-center/tasks/kanban?team_id=<uuid>`

Если `team_id` не передан, агрегируются задачи всех команд.

Response:

```json
{
  "columns": {
    "pending": [],
    "in_progress": [],
    "blocked": [],
    "completed": []
  },
  "meta": {
    "team_count": 3,
    "team_id": ""
  }
}
```

## Ошибки

1. `401 unauthorized` — неверный bearer token.
2. `403 forbidden` — недостаточная роль.
3. `400 bad request` — невалидные query params (например `team_id`).
4. `500 internal server error` — ошибка чтения агрегатов.
