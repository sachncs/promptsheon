import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');
const requestsFailed = new Counter('requests_failed');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    error_injection: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
  },
};

export default function () {
  const errorType = Math.random();
  let res;
  
  if (errorType < 0.2) {
    // Invalid JSON
    res = http.post(`${BASE_URL}/api/v1/prompts`, 'invalid json', { headers });
  } else if (errorType < 0.4) {
    // Missing required fields
    res = http.post(`${BASE_URL}/api/v1/prompts`, JSON.stringify({}), { headers });
  } else if (errorType < 0.6) {
    // Non-existent resource
    res = http.get(`${BASE_URL}/api/v1/prompts/nonexistent-id-12345`, { headers });
  } else if (errorType < 0.8) {
    // Method not allowed
    res = http.put(`${BASE_URL}/api/v1/prompts`, JSON.stringify({ name: 'test' }), { headers });
  } else {
    // Normal request
    res = http.get(`${BASE_URL}/api/v1/prompts`, { headers });
  }
  
  check(res, {
    'status is valid': (r) => r.status >= 200 && r.status < 500,
  });
  
  requestDuration.add(res.timings.duration);
  requestSuccess.add(res.status >= 200 && res.status < 500);
  if (res.status >= 500) {
    requestsFailed.add(1);
  }
  
  sleep(0.2);
}
