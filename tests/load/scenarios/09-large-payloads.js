import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');
const largePayloads = new Counter('large_payloads');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    large_payloads: {
      executor: 'constant-vus',
      vus: 5,
      duration: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<5000'],
    request_success: ['rate>0.90'],
  },
};

export default function () {
  const content = 'x'.repeat(10000);
  const payload = JSON.stringify({
    name: `large-payload-${__VU}-${__ITER}`,
    content: content,
    tags: ['large-test', 'x'.repeat(100)],
    metadata: {
      description: 'A'.repeat(5000),
      instructions: 'B'.repeat(5000),
    },
  });
  
  const res = http.post(`${BASE_URL}/api/v1/prompts`, payload, { headers });
  
  check(res, {
    'large payload status is 201': (r) => r.status === 201,
    'large payload response time acceptable': (r) => r.timings.duration < 5000,
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status === 201);
  if (res.status === 201) {
    largePayloads.add(1);
  }
  
  sleep(1);
}
