import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

export const options = {
  stages: [
    { duration: '1m', target: 20 },
    { duration: '2m', target: 50 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<800'],
    'latency_frontend': ['p(95)<600'],
    'latency_v1': ['p(95)<500'],
    'latency_v2': ['p(95)<500'],
  },
};

const host = __ENV.HOST || 'http://dev.local';
const latencyFrontend = new Trend('latency_frontend');
const latencyV1 = new Trend('latency_v1');
const latencyV2 = new Trend('latency_v2');

const PROFESSIONAL_IDS = [2684, 512, 782, 903, 4102];
const UNIT_IDS = [901, 905, 910, 915, 108];

function randomPick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function buildStartDate(offsetDays = 0) {
  const today = new Date();
  today.setUTCDate(today.getUTCDate() + offsetDays);
  return today.toISOString().slice(0, 10);
}

export default function () {
  const professionalId = randomPick(PROFESSIONAL_IDS);
  const unitId = randomPick(UNIT_IDS);
  const startOffset = Math.floor(Math.random() * 10);
  const startDate = buildStartDate(startOffset);

  const frontendRes = http.get(`${host}/?t=${Date.now()}`);
  latencyFrontend.add(frontendRes.timings.duration);
  check(frontendRes, {
    'front ok': (res) => res.status === 200,
  });

  const params = {
    tags: { service: 'available-schedules' },
  };

  const v1Res = http.get(
    `${host}/v1/appoints/available-schedule?professional_id=${professionalId}&unit_id=${unitId}&start_date=${startDate}`,
    params,
  );
  latencyV1.add(v1Res.timings.duration);
  check(v1Res, {
    'v1 ok': (res) => res.status === 200,
    'v1 payload ok': (res) => res.json('response')?.length >= 1,
  });

  const v2Res = http.get(
    `${host}/v2/appoints/available-schedule?professional_id=${professionalId}&unit_id=${unitId}&start_date=${startDate}`,
    params,
  );
  latencyV2.add(v2Res.timings.duration);
  check(v2Res, {
    'v2 ok': (res) => res.status === 200,
    'v2 payload ok': (res) => res.json('response')?.length >= 1,
  });

  sleep(0.3);
}