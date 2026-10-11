import http from 'k6/http';
import exec from 'k6/execution';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// The runner validates inputs and archives this exact configuration.
const config = JSON.parse(__ENV.BENCH_CONFIG);
const fixtures = JSON.parse(open(__ENV.BENCH_FIXTURES));
const validRedirects = new Rate('valid_redirects');
const successfulRedirects = new Counter('successful_redirects');
const successfulLatency = new Trend('successful_redirect_duration', true);
const limitedRedirects = new Counter('rate_limited_redirects');
const serverErrors = new Counter('server_error_redirects');
const transportErrors = new Counter('transport_errors');

http.setResponseCallback(http.expectedStatuses(308));

export const options = {
  discardResponseBodies: true,
  maxRedirects: 0,
  // Avoid per-code/URL tags. Status is retained to distinguish 429 and 5xx.
  systemTags: ['method', 'name', 'status', 'scenario', 'expected_response', 'error_code'],
  summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max'],
  scenarios: {
    redirects: {
      executor: 'constant-arrival-rate',
      rate: config.rate,
      timeUnit: '1s',
      duration: `${config.duration_seconds}s`,
      preAllocatedVUs: config.vus,
      maxVUs: config.vus,
      gracefulStop: `${config.timeout_ms + 1000}ms`,
    },
  },
  thresholds: {
    valid_redirects: ['rate==1'],
    successful_redirects: ['count>0'],
    http_req_failed: ['rate==0'],
    dropped_iterations: ['count==0'],
  },
};

export default function () {
  // Deterministic uniform access; each iteration issues exactly one request.
  const fixture = fixtures[exec.scenario.iterationInTest % fixtures.length];
  const response = http.get(`${config.target}/r/${fixture.code}`, {
    redirects: 0,
    timeout: `${config.timeout_ms}ms`,
    tags: { name: 'GET /r/{code}' },
  });
  const valid = check(response, {
    'status is 308': (r) => r.status === 308,
    'Location matches exactly': (r) => r.headers.Location === fixture.destination,
  });
  validRedirects.add(valid);
  limitedRedirects.add(response.status === 429 ? 1 : 0);
  serverErrors.add(response.status >= 500 && response.status <= 599 ? 1 : 0);
  transportErrors.add(response.status === 0 ? 1 : 0);
  // Add zero on failures so an all-failure run still has a counter/threshold.
  successfulRedirects.add(valid ? 1 : 0);
  if (valid) successfulLatency.add(response.timings.duration);
}

export function handleSummary(data) {
  return {
    [__ENV.BENCH_SUMMARY]: JSON.stringify(data, null, 2) + '\n',
    stdout: `Results: ${__ENV.BENCH_SUMMARY}\n` +
      'Check the exit code and thresholds before using latency/throughput.\n',
  };
}
