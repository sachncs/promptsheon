import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');
const circuitBreakerTrips = new Counter('circuit_breaker_trips');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    rate_limiting: {
      executor: 'constant-vus',
      vus: 50,
      duration: '20s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<5000'],
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/prompts`, { headers });
  
  check(res, {
    'rate limit response is valid': (r) => r.status === 200 || r.status === 429,
    'rate limit header present': (r) => r.status !== 200 || r.headers['X-RateLimit-Remaining'] !== undefined,
  });
  
  requestDuration.add(res.timings.duration);
  
  if (res.status === 200) {
    requestSuccess.add(true);
  } else if (res.status === 429) {
    requestSuccess.add(false);
    circuitBreakerTrips.add(1);
  }
  
  sleep(0.05);
}
