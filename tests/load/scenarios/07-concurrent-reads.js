import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');
const cacheHits = new Counter('cache_hits');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    concurrent_reads: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 25 },
        { duration: '40s', target: 25 },
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
    'concurrent read status is 200': (r) => r.status === 200,
    'response time acceptable': (r) => r.timings.duration < 1000,
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status === 200);
  
  if (res.timings.duration < 50) {
    cacheHits.add(1);
  }
  
  sleep(0.1);
}
