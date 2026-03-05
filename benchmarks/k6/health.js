import http from 'k6/http';
import { check } from 'k6';

const baseUrl = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 20),
  duration: __ENV.DURATION || '60s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(50)<100', 'p(95)<350'],
  },
};

export default function () {
  const res = http.get(`${baseUrl}/health`, {
    tags: { scenario: 'health' },
    timeout: '10s',
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
  });
}
