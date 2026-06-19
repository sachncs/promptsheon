import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');
const promptsCreated = new Counter('prompts_created');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    prompt_write: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 10 },
        { duration: '30s', target: 10 },
        { duration: '10s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    request_success: ['rate>0.90'],
  },
};

export default function () {
  const payload = JSON.stringify({
    name: `load-test-prompt-${Date.now()}-${__VU}-${__ITER}`,
    content: 'You are a helpful assistant. Please help with: {{query}}',
    tags: ['load-test'],
  });
  
  const res = http.post(`${BASE_URL}/api/v1/prompts`, payload, { headers });
  
  check(res, {
    'create prompt status is 201': (r) => r.status === 201,
    'create prompt returns id': (r) => {
      const body = JSON.parse(r.body);
      return body.id !== '';
    },
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status === 201);
  if (res.status === 201) {
    promptsCreated.add(1);
  }
  
  sleep(1);
}
