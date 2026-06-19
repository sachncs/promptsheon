import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

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
    mixed_read_write: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 15 },
        { duration: '40s', target: 15 },
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
  const isWrite = Math.random() < 0.3;
  let res;
  
  if (isWrite) {
    const payload = JSON.stringify({
      name: `mixed-${__VU}-${__ITER}`,
      content: 'Test prompt for mixed load',
      tags: ['load-test'],
    });
    res = http.post(`${BASE_URL}/api/v1/prompts`, payload, { headers });
  } else {
    res = http.get(`${BASE_URL}/api/v1/prompts`, { headers });
  }
  
  check(res, {
    'status is 2xx': (r) => r.status >= 200 && r.status < 300,
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status >= 200 && res.status < 300);
  
  sleep(0.5);
}
