import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    prompt_read: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 20 },
        { duration: '30s', target: 20 },
        { duration: '10s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    request_success: ['rate>0.95'],
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/prompts`, { headers });
  
  check(res, {
    'list prompts status is 200': (r) => r.status === 200,
    'list prompts returns array': (r) => {
      const body = JSON.parse(r.body);
      return Array.isArray(body);
    },
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status === 200);
  
  sleep(0.5);
}
