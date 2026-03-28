import grpc from 'k6/net/grpc';
import { Trend, Rate } from 'k6/metrics';
import exec from 'k6/execution';

const protoRoot = __ENV.PROTO_ROOT || 'D:/workspace/go/DouyinMall/backend/idl';
const seckillAddr = __ENV.SECKILL_ADDR || '127.0.0.1:8098';
const runId = __ENV.RUN_ID || `${Date.now()}`;
const vus = Number(__ENV.VUS || 50);
const rate = Number(__ENV.RATE || 50);
const duration = __ENV.DURATION || '20s';
const preAllocatedVUs = Number(__ENV.PRE_ALLOCATED_VUS || vus);
const maxVUs = Number(__ENV.MAX_VUS || Math.max(vus * 2, preAllocatedVUs));

export const options = {
  scenarios: {
    submit_seckill: {
      executor: 'constant-arrival-rate',
      rate,
      timeUnit: '1s',
      duration,
      preAllocatedVUs,
      maxVUs,
    },
  },
  thresholds: {
    seckill_submit_duration: ['p(99)<5000'],
    seckill_submit_success: ['rate>0.99'],
  },
};

const seckillClient = new grpc.Client();
seckillClient.load([protoRoot], 'seckill.proto');

const submitDuration = new Trend('seckill_submit_duration');
const submitSuccess = new Rate('seckill_submit_success');

function estimateTotalRequests() {
  const match = /^(\d+)([smh])$/.exec(duration);
  if (!match) {
    return rate * 20;
  }
  const value = Number(match[1]);
  const unit = match[2];
  const seconds = unit === 's' ? value : unit === 'm' ? value * 60 : value * 3600;
  return rate * seconds;
}

export function setup() {
  const totalStock = estimateTotalRequests() + 50;
  const now = Math.floor(Date.now() / 1000);

  seckillClient.connect(seckillAddr, { plaintext: true });
  const createResp = seckillClient.invoke('seckill.v1.SeckillService/CreateActivity', {
    activity_name: `k6-seckill-${runId}`,
    product_id: 2_000_000 + Number(runId.slice(-6)),
    sku_id: 3_000_000 + Number(runId.slice(-6)),
    seckill_price: 99,
    total_stock: totalStock,
    start_time: now - 60,
    end_time: now + 1800,
    status: 'ONLINE',
    limit_per_user: 1,
  });
  seckillClient.close();

  if (createResp && createResp.status !== grpc.StatusOK) {
    throw new Error(`create activity failed: ${JSON.stringify(createResp)}`);
  }

  return {
    activityId: Number(createResp.message.activity_id),
    totalStock,
  };
}

export default function (data) {
  const userId = 2_000_000 + exec.scenario.iterationInTest + 1;
  seckillClient.connect(seckillAddr, { plaintext: true });
  const res = seckillClient.invoke('seckill.v1.SeckillService/SubmitSeckill', {
    activity_id: data.activityId,
    user_id: userId,
  });
  seckillClient.close();

  submitDuration.add(res.timings.duration);
  const success =
    res.status === grpc.StatusOK &&
    typeof res.message.request_no === 'string' &&
    res.message.request_no.length > 0 &&
    res.message.status === 'PROCESSING';
  submitSuccess.add(success);

  if (!success) {
    throw new Error(`submit seckill failed: ${JSON.stringify(res)}`);
  }
}
