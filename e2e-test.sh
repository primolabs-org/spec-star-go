#!/usr/bin/env bash
set -euo pipefail

# EXIT trap: unconditionally tear down the environment on exit.
# The '|| true' prevents the trap itself from masking the script's real exit code
# when 'docker compose down' fails (e.g., if compose was never brought up).
trap 'docker compose down --remove-orphans >/dev/null 2>&1 || true' EXIT

# --- Prerequisite check ---

missing=()
command -v docker   >/dev/null 2>&1 || missing+=("docker")
docker compose version >/dev/null 2>&1 || missing+=("docker compose")
command -v curl     >/dev/null 2>&1 || missing+=("curl")
command -v jq       >/dev/null 2>&1 || missing+=("jq")

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "ERROR: missing required tools: ${missing[*]}" >&2
  exit 1
fi

# --- Load environment variables ---

if [[ -f .env ]]; then
  # shellcheck disable=SC1091
  source .env
fi

export POSTGRES_DB="${POSTGRES_DB:-wallet}"
export POSTGRES_USER="${POSTGRES_USER:-wallet}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-wallet}"
export POSTGRES_HOST_PORT="${POSTGRES_HOST_PORT:-5432}"
export LAMBDA_HOST_PORT="${LAMBDA_HOST_PORT:-9000}"

# --- Bring up ---

docker compose up --build -d

# --- Wait for PostgreSQL ---

wait_for_postgres() {
  local attempts=0
  local max=30
  echo "Waiting for PostgreSQL..."
  while [[ $attempts -lt $max ]]; do
    if docker compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
      echo "PostgreSQL is ready."
      return
    fi
    attempts=$((attempts + 1))
    sleep 1
  done
  echo "ERROR: PostgreSQL did not become ready within ${max}s" >&2
  exit 1
}

wait_for_postgres

# --- Wait for Lambda RIE ---

INVOKE_URL="http://localhost:${LAMBDA_HOST_PORT}/2015-03-31/functions/function/invocations"

wait_for_lambda() {
  local attempts=0
  local max=30
  local probe='{"version":"2.0","routeKey":"GET /probe","rawPath":"/probe","requestContext":{"http":{"method":"GET","path":"/probe","protocol":"HTTP/1.1","sourceIp":"127.0.0.1","userAgent":"curl"}},"headers":{},"isBase64Encoded":false}'
  echo "Waiting for Lambda RIE..."
  while [[ $attempts -lt $max ]]; do
    if curl -fsS -X POST -d "$probe" "$INVOKE_URL" >/dev/null 2>&1; then
      echo "Lambda RIE is ready."
      return
    fi
    attempts=$((attempts + 1))
    sleep 1
  done
  echo "ERROR: Lambda RIE did not become ready within ${max}s" >&2
  exit 1
}

wait_for_lambda

# --- UUID generation ---

generate_uuid() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
  elif [[ -r /proc/sys/kernel/random/uuid ]]; then
    cat /proc/sys/kernel/random/uuid
  elif command -v openssl >/dev/null 2>&1; then
    local hex
    hex=$(openssl rand -hex 16)
    echo "${hex:0:8}-${hex:8:4}-${hex:12:4}-${hex:16:4}-${hex:20:12}"
  else
    echo "ERROR: cannot generate UUID — install uuidgen or openssl" >&2
    exit 1
  fi
}

# --- Invoke and assert helper ---

invoke_and_assert() {
  local label="$1"
  local payload="$2"
  local expected_status="$3"

  local response
  response=$(curl -sS -X POST -d "$payload" "$INVOKE_URL")

  local status_code
  status_code=$(echo "$response" | jq -r '.statusCode')

  if [[ "$status_code" == "$expected_status" ]]; then
    echo "PASS: ${label}"
  else
    echo "FAIL: ${label} — expected statusCode ${expected_status}, got ${status_code}" >&2
    echo "  Response: ${response}" >&2
    exit 1
  fi
}

# --- Deposit invocation ---

DEPOSIT_ORDER_ID=$(generate_uuid)
DEPOSIT_BODY="{\"client_id\":\"00000000-0000-0000-0000-000000000001\",\"asset_id\":\"00000000-0000-0000-0000-000000000101\",\"order_id\":\"${DEPOSIT_ORDER_ID}\",\"amount\":\"10\",\"unit_price\":\"100.00\"}"
DEPOSIT_PAYLOAD=$(cat <<EOF
{
  "version": "2.0",
  "routeKey": "POST /deposits",
  "rawPath": "/deposits",
  "requestContext": {
    "http": {
      "method": "POST",
      "path": "/deposits",
      "protocol": "HTTP/1.1",
      "sourceIp": "127.0.0.1",
      "userAgent": "curl"
    }
  },
  "headers": { "content-type": "application/json" },
  "body": $(echo "$DEPOSIT_BODY" | jq -Rs .),
  "isBase64Encoded": false
}
EOF
)

invoke_and_assert "deposit" "$DEPOSIT_PAYLOAD" "201"

# --- Withdrawal invocation ---

WITHDRAW_ORDER_ID=$(generate_uuid)
WITHDRAW_BODY="{\"client_id\":\"00000000-0000-0000-0000-000000000001\",\"instrument_id\":\"CDB-0001\",\"order_id\":\"${WITHDRAW_ORDER_ID}\",\"desired_value\":\"250.00\"}"
WITHDRAW_PAYLOAD=$(cat <<EOF
{
  "version": "2.0",
  "routeKey": "POST /withdrawals",
  "rawPath": "/withdrawals",
  "requestContext": {
    "http": {
      "method": "POST",
      "path": "/withdrawals",
      "protocol": "HTTP/1.1",
      "sourceIp": "127.0.0.1",
      "userAgent": "curl"
    }
  },
  "headers": { "content-type": "application/json" },
  "body": $(echo "$WITHDRAW_BODY" | jq -Rs .),
  "isBase64Encoded": false
}
EOF
)

invoke_and_assert "withdrawal" "$WITHDRAW_PAYLOAD" "200"

echo "All tests passed."
