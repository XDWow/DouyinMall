import http from 'k6/http';
import { Trend, Rate } from 'k6/metrics';
import exec from 'k6/execution';

const baseURL = __ENV.BFF_BASE_URL || 'http://127.0.0.1:8080';
const iterations = Number(__ENV.ITERATIONS || 200);
const vus = Number(__ENV.VUS || 20);
const runId = __ENV.RUN_ID || `${Date.now()}`;

export const options = {
  scenarios: {
    checkout_place_order: {
      executor: 'shared-iterations',
      vus,
      iterations,
      maxDuration: '5m',
    },
  },
  thresholds: {
    bff_checkout_place_order_duration: ['p(99)<5000'],
    bff_checkout_place_order_success: ['rate>0.99'],
  },
};

const placeDuration = new Trend('bff_checkout_place_order_duration');
const placeSuccess = new Rate('bff_checkout_place_order_success');

export function setup() {
  const payload = JSON.stringify({
    name: `k6-bff-checkout-${runId}`,
    description: 'k6 checkout product through bff',
    picture: 'https://example.com/bff-checkout.png',
    price: 299,
    currency: 'CNY',
    stock: iterations + 20,
  });
  const resp = http.post(`${baseURL}/trade/testing/products`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  if (resp.status !== 200) {
    throw new Error(`seed product failed: status=${resp.status} body=${resp.body}`);
  }
  const body = resp.json();
  if (body.code !== 0) {
    throw new Error(`seed product business failed: ${resp.body}`);
  }
  return { productId: body.data.product_id };
}

export default function (data) {
  const userId = 1_000_000 + exec.scenario.iterationInTest + 1;
  const payload = JSON.stringify({
    user_id: userId,
    items: [{ product_id: data.productId, quantity: 1 }],
    address: {
      receiver_name: 'k6 buyer',
      phone: '13800138000',
      province: 'Shanghai',
      city: 'Shanghai',
      district: 'Pudong',
      street: 'No.1 BFF Road',
      zip_code: '200120',
    },
    payment_method: 'WECHAT_NATIVE',
    currency: 'CNY',
    expected_amount: 299,
  });

  const resp = http.post(`${baseURL}/trade/checkout/place-order`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  placeDuration.add(resp.timings.duration);
  const ok =
    resp.status === 200 &&
    resp.json('code') === 0 &&
    Number(resp.json('data.order_id')) > 0;
  placeSuccess.add(ok);

  if (!ok) {
    throw new Error(`place order failed: status=${resp.status} body=${resp.body}`);
  }
}
