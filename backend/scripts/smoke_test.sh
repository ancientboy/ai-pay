#!/usr/bin/env bash
set -euo pipefail

FRONTEND_BASE_URL="${FRONTEND_BASE_URL:-http://127.0.0.1:3000}"

echo "[smoke] base=${FRONTEND_BASE_URL}"
run_id="$(date +%s)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT
openssl genpkey -algorithm Ed25519 -out "${tmp_dir}/agent_priv.pem" >/dev/null 2>&1
openssl pkey -in "${tmp_dir}/agent_priv.pem" -pubout -outform DER -out "${tmp_dir}/agent_pub.der" >/dev/null 2>&1
agent_pub_b64="$(tail -c 32 "${tmp_dir}/agent_pub.der" | base64 | tr -d '\n')"
agent_priv_key="${tmp_dir}/agent_priv.pem"

sign_payload() {
  local payload="$1"
  local payload_file="${tmp_dir}/payload_$(date +%s%N).txt"
  printf '%s' "$payload" > "${payload_file}"
  openssl pkeyutl -sign -inkey "${agent_priv_key}" -rawin -in "${payload_file}" | base64 | tr -d '\n'
}

json_get() {
  local json="$1"
  local expr="$2"
  JSON_INPUT="$json" python3 - <<PY
import json, os
obj = json.loads(os.environ["JSON_INPUT"])
value = obj
for part in "${expr}".split("."):
    if part == "":
        continue
    if part.endswith("]") and "[" in part:
        key, idx = part[:-1].split("[")
        value = value[key][int(idx)]
    else:
        value = value[part]
if isinstance(value, (dict, list)):
    import json as _json
    print(_json.dumps(value, ensure_ascii=False))
else:
    print(value)
PY
}

assert_contains() {
  local text="$1"
  local expected="$2"
  local message="$3"
  if [[ "$text" != *"$expected"* ]]; then
    echo "[smoke][fail] ${message}"
    echo "expected contains: ${expected}"
    echo "actual: ${text}"
    exit 1
  fi
  echo "[smoke][ok] ${message}"
}

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  if [[ "$actual" != "$expected" ]]; then
    echo "[smoke][fail] ${message}"
    echo "expected: ${expected}"
    echo "actual:   ${actual}"
    exit 1
  fi
  echo "[smoke][ok] ${message}"
}

header_redirect="$(curl -I -sS "${FRONTEND_BASE_URL}/dashboard")"
assert_contains "$header_redirect" "307" "未登录访问 dashboard 重定向"
assert_contains "$header_redirect" "/login?next=%2Fdashboard" "重定向目标正确"

login_resp="$(curl -i -sS -X POST "${FRONTEND_BASE_URL}/api/auth/login" \
  -H 'content-type: application/json' \
  -d '{"username":"admin","password":"admin123"}')"
assert_contains "$login_resp" "set-cookie: ai_pay_session=" "登录返回 session cookie"

agent="did:gusd:agent:smoke_$(date +%s)"
register_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/agent/did/register" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\",\"didPubKey\":\"${agent_pub_b64}\"}")"
assert_eq "$(json_get "$register_resp" "code")" "0" "注册 Agent 成功"

verify_msg="did-verify-${run_id}"
verify_sig="$(sign_payload "${verify_msg}")"
verify_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/agent/did/verify" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\",\"message\":\"${verify_msg}\",\"signature\":\"${verify_sig}\"}")"
assert_eq "$(json_get "$verify_resp" "code")" "0" "DID 验签接口可用"

openssl genpkey -algorithm Ed25519 -out "${tmp_dir}/agent_priv_new.pem" >/dev/null 2>&1
openssl pkey -in "${tmp_dir}/agent_priv_new.pem" -pubout -outform DER -out "${tmp_dir}/agent_pub_new.der" >/dev/null 2>&1
agent_pub_new_b64="$(tail -c 32 "${tmp_dir}/agent_pub_new.der" | base64 | tr -d '\n')"
update_ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
update_proof_payload="${agent}|${agent_pub_new_b64}|${update_ts}"
update_proof_sig="$(sign_payload "${update_proof_payload}")"
update_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/agent/did/update" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\",\"newDidPubKey\":\"${agent_pub_new_b64}\",\"signTimestamp\":\"${update_ts}\",\"proofSignature\":\"${update_proof_sig}\"}")"
assert_eq "$(json_get "$update_resp" "code")" "0" "DID 公钥更新成功"
agent_priv_key="${tmp_dir}/agent_priv_new.pem"

openssl pkeyutl -sign -inkey "${tmp_dir}/agent_priv_new.pem" -rawin -in <(printf '%s' "${verify_msg}") | base64 | tr -d '\n' > "${tmp_dir}/verify_new_sig.txt"
verify_new_sig="$(cat "${tmp_dir}/verify_new_sig.txt")"
verify_after_update_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/agent/did/verify" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\",\"message\":\"${verify_msg}\",\"signature\":\"${verify_new_sig}\"}")"
assert_eq "$(json_get "$verify_after_update_resp" "code")" "0" "更新后新公钥可验签"

create_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/account/create" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\"}")"
assert_eq "$(json_get "$create_resp" "code")" "0" "开户成功"
va_account_id="$(json_get "$create_resp" "data.VAAccountID")"

agent_to="did:gusd:agent:smoke_to_$(date +%s)"
register_to_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/agent/did/register" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent_to}\",\"didPubKey\":\"${agent_pub_b64}\"}")"
assert_eq "$(json_get "$register_to_resp" "code")" "0" "转入 Agent 注册成功"

create_to_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/account/create" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent_to}\"}")"
assert_eq "$(json_get "$create_to_resp" "code")" "0" "转入账户开户成功"
va_to_account_id="$(json_get "$create_to_resp" "data.VAAccountID")"

recharge_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/fund/recharge" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: rch-smoke-${run_id}" \
  -d "{\"vaAccountId\":\"${va_account_id}\",\"amount\":\"100\"}")"
assert_eq "$(json_get "$recharge_resp" "code")" "0" "充值成功"

authorize_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/authorize/payment/set" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\",\"singleLimit\":\"50\",\"dailyLimit\":\"100\",\"whitelist\":[\"m1\"]}")"
assert_eq "$(json_get "$authorize_resp" "code")" "0" "授权规则设置成功"

authorize_update_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/authorize/payment/update" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\",\"singleLimit\":\"50\",\"dailyLimit\":\"100\",\"whitelist\":[\"m1\"]}")"
assert_eq "$(json_get "$authorize_update_resp" "code")" "0" "授权规则更新成功"

expired_pay_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/payment/x402/pay" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-smoke-expired-${run_id}" \
  -H 'X-Sign-Timestamp: 2020-01-01T00:00:00Z' \
  -d "{\"payerDid\":\"${agent}\",\"merchantId\":\"m1\",\"amount\":\"10\",\"signature\":\"sig\"}")"
assert_eq "$(json_get "$expired_pay_resp" "code")" "PAY-001" "过期签名时间戳被拦截"

ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
sig_payload="${agent}|m1|10|idem-smoke-ok-1-${run_id}|${ts}"
sig_value="$(sign_payload "${sig_payload}")"
pay_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/payment/x402/pay" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-smoke-ok-1-${run_id}" \
  -H "X-Sign-Timestamp: ${ts}" \
  -d "{\"payerDid\":\"${agent}\",\"merchantId\":\"m1\",\"amount\":\"10\",\"signature\":\"${sig_value}\"}")"
assert_eq "$(json_get "$pay_resp" "code")" "0" "支付成功"
tx_id="$(json_get "$pay_resp" "data.transactionId")"

status_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/payment/status/query?transactionId=${tx_id}")"
assert_eq "$(json_get "$status_resp" "code")" "0" "支付状态查询成功"

balance_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/account/balance/query?accountId=${va_account_id}")"
assert_eq "$(json_get "$balance_resp" "code")" "0" "余额查询成功"
assert_eq "$(json_get "$balance_resp" "data.balance")" "90" "余额扣减正确"

ledger_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/account/ledger/query?accountId=${va_account_id}")"
assert_eq "$(json_get "$ledger_resp" "code")" "0" "流水查询成功"

daily_1="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/payment/x402/pay" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-smoke-daily-1-${run_id}" \
  -H "X-Sign-Timestamp: ${ts}" \
  -d "{\"payerDid\":\"${agent}\",\"merchantId\":\"m1\",\"amount\":\"45\",\"signature\":\"$(sign_payload "${agent}|m1|45|idem-smoke-daily-1-${run_id}|${ts}")\"}")"
assert_eq "$(json_get "$daily_1" "code")" "0" "日限额测试第1笔成功"

daily_2="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/payment/x402/pay" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-smoke-daily-2-${run_id}" \
  -H "X-Sign-Timestamp: ${ts}" \
  -d "{\"payerDid\":\"${agent}\",\"merchantId\":\"m1\",\"amount\":\"45\",\"signature\":\"$(sign_payload "${agent}|m1|45|idem-smoke-daily-2-${run_id}|${ts}")\"}")"
assert_eq "$(json_get "$daily_2" "code")" "0" "日限额测试第2笔成功（打满日限额）"

daily_3="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/payment/x402/pay" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-smoke-daily-3-${run_id}" \
  -H "X-Sign-Timestamp: ${ts}" \
  -d "{\"payerDid\":\"${agent}\",\"merchantId\":\"m1\",\"amount\":\"1\",\"signature\":\"$(sign_payload "${agent}|m1|1|idem-smoke-daily-3-${run_id}|${ts}")\"}")"
assert_eq "$(json_get "$daily_3" "code")" "PAY-002" "日限额超限被拦截"

freeze_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/authorize/freeze" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\"}")"
assert_eq "$(json_get "$freeze_resp" "code")" "0" "授权规则冻结成功"

blocked_after_freeze="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/payment/x402/pay" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-smoke-freeze-block-${run_id}" \
  -H "X-Sign-Timestamp: ${ts}" \
  -d "{\"payerDid\":\"${agent}\",\"merchantId\":\"m1\",\"amount\":\"1\",\"signature\":\"$(sign_payload "${agent}|m1|1|idem-smoke-freeze-block-${run_id}|${ts}")\"}")"
assert_eq "$(json_get "$blocked_after_freeze" "code")" "PAY-002" "冻结后支付被拦截"

activate_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/authorize/activate" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\"}")"
assert_eq "$(json_get "$activate_resp" "code")" "0" "授权规则恢复成功"

post_activate_update="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/authorize/payment/update" \
  -H 'content-type: application/json' \
  -d "{\"agentDid\":\"${agent}\",\"singleLimit\":\"50\",\"dailyLimit\":\"1000\",\"whitelist\":[\"m1\"]}")"
assert_eq "$(json_get "$post_activate_update" "code")" "0" "恢复后授权规则可更新"

recharge_after_activate="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/fund/recharge" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: rch-smoke-reactivate-${run_id}" \
  -d "{\"vaAccountId\":\"${va_account_id}\",\"amount\":\"10\"}")"
assert_eq "$(json_get "$recharge_after_activate" "code")" "0" "恢复后补充余额成功"

interest_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/account/interest/query?accountId=${va_account_id}")"
assert_eq "$(json_get "$interest_resp" "code")" "0" "利息查询接口可用"

topup_cfg_set_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/account/va/topup/config" \
  -H 'content-type: application/json' \
  -d "{\"accountId\":\"${va_account_id}\",\"autoTopupEnabled\":true,\"thresholdAmount\":\"20\",\"targetAmount\":\"100\"}")"
assert_eq "$(json_get "$topup_cfg_set_resp" "code")" "0" "VA 自动充值配置设置成功"

topup_cfg_get_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/account/va/topup/config?accountId=${va_account_id}")"
assert_eq "$(json_get "$topup_cfg_get_resp" "code")" "0" "VA 自动充值配置查询成功"

transfer_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/account/va/transfer" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-va-transfer-${run_id}" \
  -d "{\"fromAccountId\":\"${va_account_id}\",\"toAccountId\":\"${va_to_account_id}\",\"amount\":\"5\"}")"
assert_eq "$(json_get "$transfer_resp" "code")" "0" "VA 转账成功"

transfer_idem_retry_resp="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/account/va/transfer" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-va-transfer-${run_id}" \
  -d "{\"fromAccountId\":\"${va_account_id}\",\"toAccountId\":\"${va_to_account_id}\",\"amount\":\"5\"}")"
assert_eq "$(json_get "$transfer_idem_retry_resp" "code")" "0" "VA 转账幂等重试成功"

pay_after_activate="$(curl -sS -X POST "${FRONTEND_BASE_URL}/api/backend/payment/x402/pay" \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: idem-smoke-activate-pass-${run_id}" \
  -H "X-Sign-Timestamp: ${ts}" \
  -d "{\"payerDid\":\"${agent}\",\"merchantId\":\"m1\",\"amount\":\"1\",\"signature\":\"$(sign_payload "${agent}|m1|1|idem-smoke-activate-pass-${run_id}|${ts}")\"}")"
assert_eq "$(json_get "$pay_after_activate" "code")" "0" "恢复后支付成功"

overview_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/metrics/overview")"
assert_eq "$(json_get "$overview_resp" "code")" "0" "概览指标接口可用"

agent_list_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/agent/list")"
assert_eq "$(json_get "$agent_list_resp" "code")" "0" "Agent 列表接口可用"

recharge_list_resp="$(curl -sS "${FRONTEND_BASE_URL}/api/backend/fund/recharge/list?accountId=${va_account_id}&limit=5")"
assert_eq "$(json_get "$recharge_list_resp" "code")" "0" "充值列表接口可用"

echo "[smoke] all checks passed"
