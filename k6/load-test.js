import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.AUTH_TOKEN || '';

const errorRate = new Rate('errors');
const casesLatency = new Trend('cases_latency', true);
const searchLatency = new Trend('search_latency', true);
const evidenceLatency = new Trend('evidence_latency', true);
const healthLatency = new Trend('health_latency', true);

const headers = {
  'Content-Type': 'application/json',
  Authorization: `Bearer ${TOKEN}`,
};

export const options = {
  scenarios: {
    // Concurrent users browsing
    browsing: {
      executor: 'constant-vus',
      vus: 25,
      duration: '2m',
      exec: 'browsing',
    },
    // Search under load
    searching: {
      executor: 'constant-vus',
      vus: 10,
      duration: '2m',
      exec: 'searching',
      startTime: '10s',
    },
    // Health check (monitoring)
    monitoring: {
      executor: 'constant-rate',
      rate: 1,
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 2,
      exec: 'healthCheck',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<2000'],
    cases_latency: ['p(95)<200'],
    search_latency: ['p(95)<500'],
    health_latency: ['p(95)<50'],
    errors: ['rate<0.01'],
  },
};

// Scenario: Users browsing cases and evidence
export function browsing() {
  // List cases
  const casesRes = http.get(`${BASE_URL}/api/cases?limit=50`, { headers });
  casesLatency.add(casesRes.timings.duration);
  check(casesRes, {
    'cases status 200': (r) => r.status === 200,
    'cases has data': (r) => {
      const body = r.json();
      return body && body.data !== null;
    },
  }) || errorRate.add(1);

  sleep(1);

  // List evidence for first case
  const casesBody = casesRes.json();
  if (casesBody?.data?.length > 0) {
    const caseId = casesBody.data[0].id;
    const evidenceRes = http.get(
      `${BASE_URL}/api/cases/${caseId}/evidence?limit=50&current_only=true`,
      { headers }
    );
    evidenceLatency.add(evidenceRes.timings.duration);
    check(evidenceRes, {
      'evidence status 200': (r) => r.status === 200,
    }) || errorRate.add(1);
  }

  sleep(2);
}

// Scenario: Search queries
export function searching() {
  const queries = [
    'witness statement',
    'document',
    'photo evidence',
    'report',
    'transcript',
  ];
  const q = queries[Math.floor(Math.random() * queries.length)];

  const res = http.get(`${BASE_URL}/api/search?q=${encodeURIComponent(q)}&limit=50`, {
    headers,
  });
  searchLatency.add(res.timings.duration);
  check(res, {
    'search status 200': (r) => r.status === 200,
    'search returns results': (r) => {
      const body = r.json();
      return body?.data?.hits !== undefined;
    },
  }) || errorRate.add(1);

  sleep(1);
}

// Scenario: Health check monitoring
export function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  healthLatency.add(res.timings.duration);
  check(res, {
    'health status 200': (r) => r.status === 200,
    'health is healthy': (r) => {
      const body = r.json();
      return body?.status === 'healthy';
    },
  }) || errorRate.add(1);
}
