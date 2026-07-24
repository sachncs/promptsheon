import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');
const sustainedRequests = new Counter('sustained_requests');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    sustained_load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 20 },
        { duration: '60s', target: 20 },
        { duration: '20s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000', 'p(99)<3000'],
    request_success: ['rate>0.95'],
  },
};

export default function () {
  const operation = Math.random();
  let res;
  
  if (operation < 0.8) {
    res = http.get(`${BASE_URL}/api/v1/workspaces`, { headers });
  } else {
    res = http.get(`${BASE_URL}/health`, { headers });
  }
  
  check(res, {
    'sustained load status is valid': (r) => r.status >= 200 && r.status < 300,
    'sustained load response time acceptable': (r) => r.timings.duration < 2000,
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status >= 200 && res.status < 300);
  if (res.status >= 200 && res.status < 300) {
    sustainedRequests.add(1);
  }
  
  sleep(0.5);
}
