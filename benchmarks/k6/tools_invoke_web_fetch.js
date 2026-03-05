import http from 'k6/http';
import { check } from 'k6';

const baseUrl = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const token = __ENV.GATEWAY_TOKEN || '';
const targetUrl = __ENV.WEB_FETCH_TARGET || 'https://example.com';

const headers = {
  'Content-Type': 'application/json',
};

if (token) {
  headers.Authorization = `Bearer ${token}`;
}

const body = JSON.stringify({
  tool: 'web_fetch',
  args: {
    url: targetUrl,
    extractMode: 'text',
    maxChars: Number(__ENV.MAX_CHARS || 2000),
  },
});

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '60s',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(50)<800', 'p(95)<2500'],
  },
};

export default function () {
  const res = http.post(`${baseUrl}/v1/tools/invoke`, body, {
    headers,
    tags: { scenario: 'tools_invoke_web_fetch' },
    timeout: '30s',
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'has result': (r) => {
      try {
        const data = r.json();
        return !!(data && data.result && typeof data.result.output === 'string');
      } catch (_e) {
        return false;
      }
    },
  });
}
