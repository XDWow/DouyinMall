#!/bin/sh
set -eu

ADMIN_URL="${APISIX_ADMIN_URL:-http://127.0.0.1:9180}"
ADMIN_KEY="${APISIX_ADMIN_KEY:?APISIX_ADMIN_KEY is required}"
APISIX_CONTAINER_NAME="${APISIX_CONTAINER_NAME:-apisix}"
APISIX_DOCKER_NETWORK="${APISIX_DOCKER_NETWORK:-douyin-mall_mall-net}"
SECKILL_SUBMIT_LIMIT_REQ_RATE="${SECKILL_SUBMIT_LIMIT_REQ_RATE:-0}"
SECKILL_SUBMIT_LIMIT_REQ_BURST="${SECKILL_SUBMIT_LIMIT_REQ_BURST:-0}"
SECKILL_SUBMIT_LIMIT_REQ_KEY="${SECKILL_SUBMIT_LIMIT_REQ_KEY:-remote_addr}"
SECKILL_SUBMIT_LIMIT_COUNT="${SECKILL_SUBMIT_LIMIT_COUNT:-0}"
SECKILL_SUBMIT_LIMIT_COUNT_WINDOW="${SECKILL_SUBMIT_LIMIT_COUNT_WINDOW:-60}"
SECKILL_SUBMIT_LIMIT_COUNT_KEY="${SECKILL_SUBMIT_LIMIT_COUNT_KEY:-remote_addr}"

connect_network_if_needed() {
  if ! command -v docker >/dev/null 2>&1; then
    return
  fi

  if ! docker inspect "${APISIX_CONTAINER_NAME}" >/dev/null 2>&1; then
    echo "APISIX container ${APISIX_CONTAINER_NAME} not found, skip docker network connect."
    return
  fi

  if docker inspect "${APISIX_CONTAINER_NAME}" \
    --format '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' \
    | grep -Fx "${APISIX_DOCKER_NETWORK}" >/dev/null 2>&1; then
    echo "APISIX already attached to docker network ${APISIX_DOCKER_NETWORK}."
    return
  fi

  echo "Connecting ${APISIX_CONTAINER_NAME} to docker network ${APISIX_DOCKER_NETWORK}..."
  docker network connect "${APISIX_DOCKER_NETWORK}" "${APISIX_CONTAINER_NAME}"
}

put_route() {
  id="$1"
  payload="$2"
  curl -fsS -X PUT "${ADMIN_URL}/apisix/admin/routes/${id}" \
    -H "X-API-KEY: ${ADMIN_KEY}" \
    -H "Content-Type: application/json" \
    -d "${payload}" >/dev/null
}

delete_route_if_exists() {
  id="$1"
  if curl -fsS "${ADMIN_URL}/apisix/admin/routes/${id}" -H "X-API-KEY: ${ADMIN_KEY}" >/dev/null 2>&1; then
    curl -fsS -X DELETE "${ADMIN_URL}/apisix/admin/routes/${id}" \
      -H "X-API-KEY: ${ADMIN_KEY}" >/dev/null
  fi
}

build_seckill_submit_plugins() {
  extra=''
  if [ "${SECKILL_SUBMIT_LIMIT_REQ_RATE}" -gt 0 ]; then
    extra="${extra},
    \"limit-req\": {
      \"rate\": ${SECKILL_SUBMIT_LIMIT_REQ_RATE},
      \"burst\": ${SECKILL_SUBMIT_LIMIT_REQ_BURST},
      \"nodelay\": true,
      \"rejected_code\": 429,
      \"key_type\": \"var\",
      \"key\": \"${SECKILL_SUBMIT_LIMIT_REQ_KEY}\"
    }"
  fi
  if [ "${SECKILL_SUBMIT_LIMIT_COUNT}" -gt 0 ]; then
    extra="${extra},
    \"limit-count\": {
      \"count\": ${SECKILL_SUBMIT_LIMIT_COUNT},
      \"time_window\": ${SECKILL_SUBMIT_LIMIT_COUNT_WINDOW},
      \"rejected_code\": 429,
      \"key_type\": \"var\",
      \"key\": \"${SECKILL_SUBMIT_LIMIT_COUNT_KEY}\"
    }"
  fi

  cat <<EOF
{
  "proxy-rewrite": {
    "headers": {
      "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
    }
  }${extra}
}
EOF
}

connect_network_if_needed

echo "Waiting for APISIX admin API..."
attempt=0
while [ "${attempt}" -lt 30 ]; do
  if curl -fsS "${ADMIN_URL}/apisix/admin/routes" -H "X-API-KEY: ${ADMIN_KEY}" >/dev/null 2>&1; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done

if [ "${attempt}" -ge 30 ]; then
  echo "APISIX admin API is not ready: ${ADMIN_URL}" >&2
  exit 1
fi

# Clean up conflicting legacy routes so the current gateway mapping wins deterministically.
delete_route_if_exists "seckill-submit"

SECKILL_SUBMIT_PLUGINS="$(build_seckill_submit_plugins)"

put_route 1000 '{
  "uri": "/gateway/healthz",
  "methods": ["GET"],
  "plugins": {
    "proxy-rewrite": {
      "uri": "/healthz"
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "bff-service:8080": 1 }
  }
}'

put_route 1001 '{
  "uri": "/api/auth/*",
  "methods": ["POST", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "bff-service:8080": 1 }
  }
}'

put_route 1002 '{
  "uri": "/api/agent*",
  "methods": ["GET", "POST", "DELETE", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "bff-service:8080": 1 }
  }
}'

put_route 1003 '{
  "uri": "/api/cart*",
  "methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "cart-service:18099": 1 }
  }
}'

put_route 1004 '{
  "uri": "/api/products*",
  "methods": ["GET", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "product-service:18096": 1 }
  }
}'

put_route 1005 '{
  "uri": "/api/checkout*",
  "methods": ["POST", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "checkout-service:18086": 1 }
  }
}'

put_route 1006 '{
  "uri": "/api/orders*",
  "methods": ["GET", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "order-service:18095": 1 }
  }
}'

put_route 1007 '{
  "uri": "/api/seckill/activities/*",
  "methods": ["GET", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "seckill-service:18098": 1 }
  }
}'

put_route 1008 "$(cat <<EOF
{
  "uri": "/api/seckill/submit",
  "methods": ["POST", "OPTIONS"],
  "plugins": ${SECKILL_SUBMIT_PLUGINS},
  "upstream": {
    "type": "roundrobin",
    "nodes": { "seckill-service:18098": 1 }
  }
}
EOF
)"

put_route 1009 '{
  "uri": "/api/seckill/result",
  "methods": ["GET", "OPTIONS"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "seckill-service:18098": 1 }
  }
}'

put_route 1010 '{
  "uri": "/payment/wechat/callback",
  "methods": ["POST"],
  "upstream": {
    "type": "roundrobin",
    "nodes": { "payment-service:8093": 1 }
  }
}'

put_route 1011 '{
  "uri": "/payment/alipay/callback",
  "methods": ["POST"],
  "upstream": {
    "type": "roundrobin",
    "nodes": { "payment-service:8093": 1 }
  }
}'

put_route 1012 '{
  "uri": "/api/pay/alipay/notify",
  "methods": ["POST"],
  "upstream": {
    "type": "roundrobin",
    "nodes": { "payment-service:8093": 1 }
  }
}'

put_route 1013 '{
  "uri": "/api/pay/alipay/page",
  "methods": ["GET", "POST", "OPTIONS"],
  "upstream": {
    "type": "roundrobin",
    "nodes": { "payment-service:8093": 1 }
  }
}'

put_route 1014 '{
  "uri": "/pay/success",
  "methods": ["GET"],
  "upstream": {
    "type": "roundrobin",
    "nodes": { "payment-service:8093": 1 }
  }
}'

echo "APISIX routes initialized."
