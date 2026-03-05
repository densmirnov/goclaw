import http from 'k6/http';
import { check } from 'k6';

const baseUrl = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const token = __ENV.GATEWAY_TOKEN || '';
const userId = __ENV.BENCH_USER_ID || 'bench-user';
const agentId = __ENV.BENCH_AGENT_ID || 'default';
const model = __ENV.BENCH_MODEL || `goclaw:${agentId}`;
const message = __ENV.BENCH_MESSAGE || 'Напиши одно короткое предложение без вызова инструментов.';

const headers = {
  'Content-Type': 'application/json',
  'X-GoClaw-User-Id': userId,
  'X-GoClaw-Agent-Id': agentId,
};

if (token) {
  headers.Authorization = `Bearer ${token}`;
}

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.DURATION || '60s',
  thresholds: {
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(50)<1800', 'p(95)<4500'],
  },
};

export default function () {
  const body = JSON.stringify({
    model,
    stream: false,
    messages: [{ role: 'user', content: message }],
  });

  const res = http.post(`${baseUrl}/v1/chat/completions`, body, {
    headers,
    tags: { scenario: 'chat_completions' },
    timeout: '45s',
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'has assistant content': (r) => {
      try {
        const data = r.json();
        return !!(
          data &&
          Array.isArray(data.choices) &&
          data.choices[0] &&
          data.choices[0].message &&
          typeof data.choices[0].message.content === 'string'
        );
      } catch (_e) {
        return false;
      }
    },
  });
}
