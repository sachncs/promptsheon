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
    health_check: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],
    request_success: ['rate>0.95'],
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/health`, { headers });
  
  check(res, {
    'health status is 200': (r) => r.status === 200,
    'health response has status': (r) => {
      const body = JSON.parse(r.body);
      return body.status === 'healthy';
    },
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status === 200);
  
  sleep(0.1);
}
