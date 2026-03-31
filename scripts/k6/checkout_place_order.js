import grpc from 'k6/net/grpc';
import { Trend, Rate } from 'k6/metrics';
import exec from 'k6/execution';

const protoRoot = __ENV.PROTO_ROOT || 'D:/workspace/go/DouyinMall/backend/idl';
const checkoutAddr = __ENV.CHECKOUT_ADDR || '127.0.0.1:8086';
const productAddr = __ENV.PRODUCT_ADDR || '127.0.0.1:8096';
const inventoryAddr = __ENV.INVENTORY_ADDR || '127.0.0.1:8094';
const runId = __ENV.RUN_ID || `${Date.now()}`;
const iterations = Number(__ENV.ITERATIONS || 200);
const vus = Number(__ENV.VUS || 20);

export const options = {
  scenarios: {
    place_order: {
      executor: 'shared-iterations',
      vus,
      iterations,
      maxDuration: '5m',
    },
  },
  thresholds: {
    checkout_place_order_duration: ['p(99)<5000'],
    checkout_place_order_success: ['rate>0.99'],
  },
};

const checkoutClient = new grpc.Client();
checkoutClient.load([protoRoot], 'checkout.proto');

const productClient = new grpc.Client();
productClient.load([protoRoot], 'product.proto');

const inventoryClient = new grpc.Client();
inventoryClient.load([protoRoot], 'inventory.proto');

const placeOrderDuration = new Trend('checkout_place_order_duration');
const placeOrderSuccess = new Rate('checkout_place_order_success');

export function setup() {
  productClient.connect(productAddr, { plaintext: true });
  const createResp = productClient.invoke('product.ProductService/CreateProduct', {
    product: {
      name: `k6-checkout-${runId}`,
      description: 'k6 checkout benchmark product',
      picture: 'https://example.com/k6-checkout.png',
      price: 299,
      currency: 'CNY',
      categories: ['k6', 'benchmark'],
      in_stock: true,
      merchantID: 10001,
      merchantName: 'k6-benchmark',
    },
  });
  productClient.close();

  if (createResp && createResp.status !== grpc.StatusOK) {
    throw new Error(`create product failed: ${JSON.stringify(createResp)}`);
  }

  const productId = Number(createResp.message.id);
  inventoryClient.connect(inventoryAddr, { plaintext: true });
  const adjustResp = inventoryClient.invoke('inventory.v1.InventoryService/AdjustStock', {
    reason: `k6_checkout_seed_${runId}`,
    items: [{ product_id: productId, quantity: iterations + 20 }],
  });
  inventoryClient.close();
  if (adjustResp && adjustResp.status !== grpc.StatusOK) {
    throw new Error(`adjust stock failed: ${JSON.stringify(adjustResp)}`);
  }
  if (Number(adjustResp.message.status_code) !== 0) {
    throw new Error(`adjust stock business failed: ${JSON.stringify(adjustResp.message)}`);
  }

  return { productId };
}

export default function (data) {
  const userId = 1_000_000 + exec.scenario.iterationInTest + 1;
  checkoutClient.connect(checkoutAddr, { plaintext: true });
  const res = checkoutClient.invoke('checkout.v1.CheckoutService/PlaceOrder', {
    user_id: userId,
    items: [{ product_id: data.productId, quantity: 1 }],
    address: {
      receiver_name: 'k6 buyer',
      phone: '13800138000',
      province: 'Shanghai',
      city: 'Shanghai',
      district: 'Pudong',
      street: 'No.1 K6 Road',
      zip_code: '200120',
    },
    payment_method: 'WECHAT_NATIVE',
    currency: 'CNY',
    expected_amount: 299,
  });
  checkoutClient.close();

  placeOrderDuration.add(res.timings.duration);
  const success = res.status === grpc.StatusOK && Number(res.message.order_id) > 0;
  placeOrderSuccess.add(success);

  if (!success) {
    throw new Error(`place order failed: ${JSON.stringify(res)}`);
  }
}
