#!/bin/sh
set -eu

ADMIN_URL="${APISIX_ADMIN_URL:-http://apisix:9180}"
ADMIN_KEY="${APISIX_ADMIN_KEY:?APISIX_ADMIN_KEY is required}"

put_route() {
  id="$1"
  payload="$2"
  curl -fsS -X PUT "${ADMIN_URL}/apisix/admin/routes/${id}" \
    -H "X-API-KEY: ${ADMIN_KEY}" \
    -H "Content-Type: application/json" \
    -d "${payload}" >/dev/null
}

echo "Waiting for APISIX admin API..."
attempt=0
while [ "${attempt}" -lt 30 ]; do
  if curl -fsS "${ADMIN_URL}/apisix/admin/routes" -H "X-API-KEY: ${ADMIN_KEY}" >/dev/null 2>&1; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done

put_route 1001 '{
  "uri": "/api/auth/*",
  "methods": ["POST"],
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
  "uri": "/api/cart*",
  "methods": ["GET", "POST", "PUT", "DELETE"],
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

put_route 1003 '{
  "uri": "/api/products*",
  "methods": ["GET"],
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

put_route 1004 '{
  "uri": "/api/search*",
  "methods": ["GET"],
  "plugins": {
    "proxy-rewrite": {
      "headers": {
        "remove": ["X-User-ID", "X-User-Id", "x-user-id", "X-User-Roles", "x-user-roles"]
      }
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": { "search-service:18093": 1 }
  }
}'

put_route 1005 '{
  "uri": "/api/checkout*",
  "methods": ["POST"],
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
  "methods": ["GET"],
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
  "methods": ["GET"],
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

put_route 1008 '{
  "uri": "/api/seckill/submit",
  "methods": ["POST"],
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

put_route 1009 '{
  "uri": "/api/seckill/result",
  "methods": ["GET"],
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

echo "APISIX routes initialized."
