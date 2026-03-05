import http from 'k6/http';
import { check, sleep } from 'k6';

const baseUrl = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const token = __ENV.GATEWAY_TOKEN || '';

export const options = {
  vus: Number(__ENV.VUS || 20),
  duration: __ENV.DURATION || '60s',
  thresholds: {
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(50)<250', 'p(95)<1200'],
  },
};

function authHeaders() {
  const headers = {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

export default function () {
  const headers = authHeaders();
  const endpoints = [
    '/v1/admin/control-center/overview',
    '/v1/admin/control-center/agents?limit=50&offset=0',
    '/v1/admin/control-center/runs/live?limit=100',
    '/v1/admin/control-center/tasks/kanban',
    '/v1/admin/control-center/governance',
    '/v1/admin/control-center/knowledge',
    '/v1/admin/control-center/delegation-map',
    '/v1/admin/control-center/cost',
    '/v1/admin/control-center/health',
    '/v1/admin/control-center/slo-alerts',
  ];
  const idx = Math.floor(Math.random() * endpoints.length);
  const res = http.get(`${baseUrl}${endpoints[idx]}`, {
    headers,
    timeout: '15s',
    tags: { scenario: 'control_center' },
  });
  check(res, {
    'status is 200': (r) => r.status === 200,
  });
  sleep(0.2);
}
