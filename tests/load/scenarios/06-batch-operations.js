import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.K6_BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.K6_API_KEY || '';

const requestDuration = new Trend('request_duration');
const requestSuccess = new Rate('request_success');
const batchRequests = new Counter('batch_requests');

const headers = {
  'Content-Type': 'application/json',
  ...(API_KEY ? { 'Authorization': `Bearer ${API_KEY}` } : {}),
};

export const options = {
  scenarios: {
    batch_operations: {
      executor: 'constant-vus',
      vus: 5,
      duration: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<3000'],
    request_success: ['rate>0.90'],
  },
};

export default function () {
  const batchSize = 5;
  const requests = [];
  
  for (let i = 0; i < batchSize; i++) {
    const payload = JSON.stringify({
      name: `batch-${__VU}-${__ITER}-${i}`,
      content: `Batch test prompt ${i}`,
      tags: ['batch-test'],
    });
    requests.push({
      method: 'POST',
      url: `${BASE_URL}/api/v1/prompts`,
      body: payload,
      params: { headers },
    });
  }
  
  const responses = http.batch(requests);
  
  let allSuccess = true;
  for (const res of responses) {
    if (res.status !== 201) {
      allSuccess = false;
    }
    requestDuration.add(res.timings.duration);
  }
  
  requestSuccess.add(allSuccess);
  if (allSuccess) {
    batchRequests.add(batchSize);
  }
  
  sleep(1);
}
