# RBAC Matrix (WS + HTTP)

## Роли

1. `viewer`: чтение.
2. `operator`: чтение + операционные изменения.
3. `admin`: полный доступ.

## HTTP (ключевые зоны)

1. `viewer`
- `GET /v1/traces`
- `GET /v1/traces/{id}`
- `GET /v1/delegations`
- `GET /v1/delegations/{id}`
- `GET /v1/agents`
- `GET /v1/agents/{id}`
- `GET /v1/agents/{id}/shares`
- `GET /v1/skills`
- `GET /v1/skills/{id}`
- `GET /v1/agents/{id}/skills`

2. `operator`
- Все `viewer`
- Операции по агентам/скиллам/каналам (create/update/delete)
- `GET /v1/providers` и `GET /v1/providers/{id}/models`
- `POST /v1/providers/{id}/verify`
- `GET /v1/tools/builtin`
- `GET /v1/tools/custom`

3. `admin`
- Все `operator`
- `POST/PUT/DELETE /v1/providers*`
- `POST/PUT/DELETE /v1/mcp*` (включая grants/review)
- `POST/PUT/DELETE /v1/tools/custom*`
- `PUT /v1/tools/builtin/{name}`
- `GET /v1/admin/control-center*`

## UI маршруты (frontend guards)

1. `viewer`
- `Overview`, `Chat`, `Sessions`, `Traces`, `Usage`, `Logs`, `Delegations`

2. `operator`
- Все `viewer`
- `Agents`, `Teams`, `Channels`, `Skills`, `Cron`, `Approvals`

3. `admin`
- Все `operator`
- `Providers`, `Config`, `Custom Tools`, `Built-in Tools`, `MCP`, `Nodes`, `TTS`
