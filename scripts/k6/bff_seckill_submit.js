import http from 'k6/http';
import { Trend, Rate } from 'k6/metrics';
import exec from 'k6/execution';

const baseURL = __ENV.BFF_BASE_URL || 'http://127.0.0.1:8080';
const vus = Number(__ENV.VUS || 100);
const iterations = Number(__ENV.ITERATIONS || 1000);
const activityCount = Number(__ENV.ACTIVITY_COUNT || 1);
const runId = __ENV.RUN_ID || `${Date.now()}`;

export const options = {
  scenarios: {
    seckill_submit: {
      executor: 'shared-iterations',
      vus,
      iterations,
      maxDuration: '5m',
    },
  },
  thresholds: {
    bff_seckill_submit_duration: ['p(99)<5000'],
    bff_seckill_submit_success: ['rate>0.99'],
  },
};

const submitDuration = new Trend('bff_seckill_submit_duration');
const submitSuccess = new Rate('bff_seckill_submit_success');

export function setup() {
  const now = Math.floor(Date.now() / 1000);
  const stock = Math.max(1, Math.floor(iterations / activityCount / 4));
  const activities = [];

  for (let i = 0; i < activityCount; i += 1) {
    const suffix = Number(runId.slice(-6)) + i;
    const payload = JSON.stringify({
      activity_name: `k6-bff-seckill-${runId}-${i}`,
      product_id: 5_000_000 + suffix,
      sku_id: 6_000_000 + suffix,
      seckill_price: 99,
      total_stock: stock,
      start_time: now - 60,
      end_time: now + 1800,
      status: 'ONLINE',
      limit_per_user: 1,
    });
    const resp = http.post(`${baseURL}/trade/seckill/activities`, payload, {
      headers: { 'Content-Type': 'application/json' },
    });
    if (resp.status !== 200) {
      throw new Error(`create activity failed: index=${i} status=${resp.status} body=${resp.body}`);
    }
    const body = resp.json();
    if (body.code !== 0) {
      throw new Error(`create activity business failed: index=${i} body=${resp.body}`);
    }
    activities.push(body.data.activity_id);
  }

  return {
    activities,
    stock,
  };
}

export default function (data) {
  const userId = 2_000_000 + exec.scenario.iterationInTest + 1;
  const activityId = data.activities[exec.scenario.iterationInTest % data.activities.length];
  const payload = JSON.stringify({
    user_id: userId,
    activity_id: activityId,
  });
  const resp = http.post(`${baseURL}/trade/seckill/submit`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  submitDuration.add(resp.timings.duration);
  const ok =
    resp.status === 200 &&
    resp.json('code') === 0 &&
    typeof resp.json('data.status') === 'string';
  submitSuccess.add(ok);

  if (!ok) {
    throw new Error(`submit seckill failed: status=${resp.status} body=${resp.body}`);
  }
}
