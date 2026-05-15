import http from 'k6/http';
import grpc from 'k6/net/grpc';
import exec from 'k6/execution';
import crypto from 'k6/crypto';
import encoding from 'k6/encoding';
import { SharedArray } from 'k6/data';
import { Rate, Trend } from 'k6/metrics';

const apisixBaseURL = (__ENV.APISIX_BASE_URL || 'http://127.0.0.1:9080').replace(/\/+$/, '');
const seckillGrpcAddr = __ENV.SECKILL_GRPC_ADDR || '127.0.0.1:8098';
const protoRoot = __ENV.PROTO_ROOT || '/work/proto';
const jwtAccessSecret = (__ENV.JWT_ACCESS_SECRET || '').trim();
if (!jwtAccessSecret) {
  throw new Error('JWT_ACCESS_SECRET is required');
}

const runId = __ENV.RUN_ID || `${Date.now()}`;
const mainRate = toPositiveInt(__ENV.MAIN_RATE, 600);
const mainDuration = __ENV.MAIN_DURATION || '60s';
const warmupRate = toNonNegativeInt(__ENV.WARMUP_RATE, 150);
const warmupDuration = __ENV.WARMUP_DURATION || '15s';
const preAllocatedVUs = toPositiveInt(__ENV.PRE_ALLOCATED_VUS, Math.max(mainRate, warmupRate, 200));
const maxVUs = toPositiveInt(__ENV.MAX_VUS, Math.max(preAllocatedVUs * 2, 1000));
const activityCount = toPositiveInt(__ENV.ACTIVITY_COUNT, 8);
const hotActivityCount = Math.max(1, Math.min(activityCount, toPositiveInt(__ENV.HOT_ACTIVITY_COUNT, 2)));
const hotPercent = clampPercent(toPositiveInt(__ENV.HOT_PERCENT, 80));
const stockFactor = toPositiveFloat(__ENV.STOCK_FACTOR, 1.25);
const tokenTTLSeconds = toPositiveInt(__ENV.TOKEN_TTL_SECONDS, 7200);
const hotActivityIDsFromEnv = parseIDList(__ENV.HOT_ACTIVITY_IDS || '');
const coldActivityIDsFromEnv = parseIDList(__ENV.COLD_ACTIVITY_IDS || '');
const allowGateway429 = isTrue(__ENV.ALLOW_GATEWAY_429 || '');
const allowBusinessFastFail = isTrue(__ENV.ALLOW_BUSINESS_FAST_FAIL || '');
const expectedFailReasons = new Set(
  (__ENV.EXPECTED_FAIL_REASONS || 'OUT_OF_STOCK,DUPLICATE')
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item.length > 0),
);

const warmupSeconds = parseDurationSeconds(warmupDuration);
const mainSeconds = parseDurationSeconds(mainDuration);
const warmupRequests = warmupRate * warmupSeconds;
const mainRequests = mainRate * mainSeconds;
const warmupTokenPoolSize = Math.max(1000, Math.ceil(Math.max(warmupRequests, warmupRate) * 1.1));
const mainTokenPoolSize = Math.max(5000, Math.ceil(Math.max(mainRequests, mainRate) * 1.1));
const warmupUserBase = 2_000_000;
const mainUserBase = 20_000_000;

const submitDuration = new Trend('seckill_submit_duration');
const submitSuccess = new Rate('seckill_submit_success');
const submitHandled = new Rate('seckill_submit_handled');
const gatewayRejected = new Rate('seckill_submit_gateway_rejected');
const businessFastFail = new Rate('seckill_submit_business_fast_fail');
const seckillClient = new grpc.Client();
if (hotActivityIDsFromEnv.length === 0 && coldActivityIDsFromEnv.length === 0) {
  seckillClient.load([protoRoot], 'seckill.proto');
}

const scenarios = {
  main: {
    executor: 'constant-arrival-rate',
    rate: mainRate,
    timeUnit: '1s',
    duration: mainDuration,
    preAllocatedVUs,
    maxVUs,
    exec: 'submitMain',
    tags: { phase: 'main' },
  },
};

if (warmupRate > 0 && warmupSeconds > 0) {
  scenarios.warmup = {
    executor: 'constant-arrival-rate',
    rate: warmupRate,
    timeUnit: '1s',
    duration: warmupDuration,
    preAllocatedVUs: Math.min(preAllocatedVUs, Math.max(warmupRate * 2, 50)),
    maxVUs: Math.max(Math.min(maxVUs, preAllocatedVUs), Math.max(warmupRate * 3, 100)),
    exec: 'submitWarmup',
    startTime: '0s',
    tags: { phase: 'warmup' },
  };
  scenarios.main.startTime = warmupDuration;
}

export const options = {
  scenarios,
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  thresholds: {
    'seckill_submit_duration{phase:main}': ['p(99)<5000'],
    'seckill_submit_handled{phase:main}': ['rate>0.99'],
  },
};

const warmupTokens = new SharedArray('warmup_tokens', function () {
  return buildTokenPool(warmupUserBase, warmupTokenPoolSize);
});

const mainTokens = new SharedArray('main_tokens', function () {
  return buildTokenPool(mainUserBase, mainTokenPoolSize);
});

export function setup() {
  const totalExpectedRequests = Math.ceil((warmupRequests + mainRequests) * stockFactor);
  const hotTotalRequests = Math.ceil((totalExpectedRequests * hotPercent) / 100);
  const coldTotalRequests = Math.max(totalExpectedRequests - hotTotalRequests, 0);
  const coldActivityCount = Math.max(activityCount - hotActivityCount, 0);

  const hotPerActivityStock = Math.max(1, Math.ceil(hotTotalRequests / hotActivityCount));
  const coldPerActivityStock =
    coldActivityCount > 0 ? Math.max(1, Math.ceil(coldTotalRequests / coldActivityCount)) : 0;

  let hotActivities = hotActivityIDsFromEnv.slice();
  let coldActivities = coldActivityIDsFromEnv.slice();
  if (hotActivities.length === 0 && coldActivities.length === 0) {
    const now = Math.floor(Date.now() / 1000);
    hotActivities = [];
    coldActivities = [];

    seckillClient.connect(seckillGrpcAddr, { plaintext: true });
    try {
      for (let i = 0; i < activityCount; i += 1) {
        const isHot = i < hotActivityCount;
        const stock = isHot ? hotPerActivityStock : coldPerActivityStock;
        const suffix = Number(runId.slice(-6)) + i;
        const resp = seckillClient.invoke('seckill.v1.SeckillService/CreateActivity', {
          activity_name: `k6-apisix-seckill-${runId}-${i}`,
          product_id: 7_000_000 + suffix,
          sku_id: 8_000_000 + suffix,
          seckill_price: 99,
          total_stock: stock,
          start_time: now - 60,
          end_time: now + 3600,
          status: 'ONLINE',
          limit_per_user: 1,
        });
        if (!resp || resp.status !== grpc.StatusOK) {
          throw new Error(`create activity failed: index=${i} resp=${JSON.stringify(resp)}`);
        }
        const activityId = Number(resp.message.activity_id);
        if (!activityId) {
          throw new Error(`create activity returned empty id: index=${i} resp=${JSON.stringify(resp)}`);
        }
        if (isHot) {
          hotActivities.push(activityId);
        } else {
          coldActivities.push(activityId);
        }
      }
    } finally {
      seckillClient.close();
    }
  }

  // Prime APISIX routes and activity cache before the timed phase.
  for (const activityId of hotActivities.concat(coldActivities)) {
    const resp = http.get(`${apisixBaseURL}/api/seckill/activities/${activityId}`, {
      tags: { phase: 'setup' },
      timeout: '5s',
    });
    if (resp.status !== 200) {
      throw new Error(`prime activity route failed: activity=${activityId} status=${resp.status} body=${resp.body}`);
    }
  }

  return {
    hotActivities,
    coldActivities,
    requestedHotPercent: hotPercent,
    hotPerActivityStock,
    coldPerActivityStock,
    totalExpectedRequests,
  };
}

export function submitWarmup(data) {
  submitScenario(data, 'warmup');
}

export function submitMain(data) {
  submitScenario(data, 'main');
}

function submitScenario(data, phase) {
  const iteration = exec.scenario.iterationInTest;
  const activityId = pickActivityId(data, iteration);
  const token = phase === 'warmup' ? warmupTokens[iteration % warmupTokens.length] : mainTokens[iteration % mainTokens.length];

  const resp = http.post(
    `${apisixBaseURL}/api/seckill/submit`,
    JSON.stringify({ activity_id: activityId }),
    {
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      tags: { phase },
      timeout: '5s',
    },
  );

  submitDuration.add(resp.timings.duration, { phase });

  const outcome = classifyResponse(resp);
  submitSuccess.add(outcome.acceptedProcessing, { phase });
  submitHandled.add(outcome.expected, { phase });
  gatewayRejected.add(outcome.gatewayRejected, { phase });
  businessFastFail.add(outcome.businessFastFail, { phase });

  if (!outcome.expected) {
    throw new Error(`submit failed: phase=${phase} status=${resp.status} body=${resp.body}`);
  }
}

function classifyResponse(resp) {
  if (resp.status === 429) {
    return {
      expected: allowGateway429,
      acceptedProcessing: false,
      gatewayRejected: true,
      businessFastFail: false,
    };
  }

  if (resp.status !== 200) {
    return {
      expected: false,
      acceptedProcessing: false,
      gatewayRejected: false,
      businessFastFail: false,
    };
  }

  const body = resp.json();
  if (!body || body.code !== 0 || !body.data) {
    return {
      expected: false,
      acceptedProcessing: false,
      gatewayRejected: false,
      businessFastFail: false,
    };
  }

  if (typeof body.data.request_no === 'string' && body.data.request_no.length > 0 && body.data.status === 'PROCESSING') {
    return {
      expected: true,
      acceptedProcessing: true,
      gatewayRejected: false,
      businessFastFail: false,
    };
  }

  if (allowBusinessFastFail && body.data.status === 'FAILED' && expectedFailReasons.has(String(body.data.fail_reason || '').trim())) {
    return {
      expected: true,
      acceptedProcessing: false,
      gatewayRejected: false,
      businessFastFail: true,
    };
  }

  return {
    expected: false,
    acceptedProcessing: false,
    gatewayRejected: false,
    businessFastFail: false,
  };
}

function pickActivityId(data, iteration) {
  const hotActivities = data.hotActivities || [];
  const coldActivities = data.coldActivities || [];
  const bucket = iteration % 100;
  const useHot = hotActivities.length > 0 && (coldActivities.length === 0 || bucket < data.requestedHotPercent);
  if (useHot) {
    return hotActivities[iteration % hotActivities.length];
  }
  return coldActivities[iteration % coldActivities.length];
}

function buildTokenPool(userBase, size) {
  const tokens = new Array(size);
  const now = Math.floor(Date.now() / 1000);
  for (let i = 0; i < size; i += 1) {
    tokens[i] = makeAccessToken(userBase + i + 1, now);
  }
  return tokens;
}

function makeAccessToken(userId, issuedAt) {
  const header = { alg: 'HS256', typ: 'JWT' };
  const payload = {
    user_id: userId,
    token_type: 'access',
    iat: issuedAt,
    exp: issuedAt + tokenTTLSeconds,
  };
  const encodedHeader = encoding.b64encode(JSON.stringify(header), 'rawurl');
  const encodedPayload = encoding.b64encode(JSON.stringify(payload), 'rawurl');
  const signingInput = `${encodedHeader}.${encodedPayload}`;
  const signature = crypto.hmac('sha256', jwtAccessSecret, signingInput, 'base64rawurl');
  return `${signingInput}.${signature}`;
}

function toPositiveInt(value, defaultValue) {
  const parsed = Number.parseInt(value, 10);
  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed;
  }
  return defaultValue;
}

function toNonNegativeInt(value, defaultValue) {
  const parsed = Number.parseInt(value, 10);
  if (Number.isFinite(parsed) && parsed >= 0) {
    return parsed;
  }
  return defaultValue;
}

function toPositiveFloat(value, defaultValue) {
  const parsed = Number.parseFloat(value);
  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed;
  }
  return defaultValue;
}

function clampPercent(value) {
  return Math.max(0, Math.min(100, value));
}

function parseDurationSeconds(raw) {
  const match = /^(\d+)([smh])$/.exec(raw);
  if (!match) {
    throw new Error(`invalid duration: ${raw}`);
  }
  const value = Number(match[1]);
  switch (match[2]) {
    case 's':
      return value;
    case 'm':
      return value * 60;
    case 'h':
      return value * 3600;
    default:
      throw new Error(`unsupported duration unit: ${raw}`);
  }
}

function parseIDList(raw) {
  if (!raw) {
    return [];
  }
  return raw
    .split(',')
    .map((item) => Number.parseInt(item.trim(), 10))
    .filter((value) => Number.isFinite(value) && value > 0);
}

function isTrue(raw) {
  switch (String(raw).trim().toLowerCase()) {
    case '1':
    case 'true':
    case 'yes':
    case 'on':
      return true;
    default:
      return false;
  }
}
