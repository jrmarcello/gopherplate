#!/bin/bash
# =============================================================================
# Kibana Setup Script — Gopherplate
#
# Creates Data Views and imports the dashboard with all visualizations.
# Run after Elasticsearch and Kibana are healthy.
#
# Usage:
#   ./setup_kibana.sh                          # default: http://localhost:5601
#   KIBANA_URL=http://kibana:5601 ./setup_kibana.sh
# =============================================================================

set -euo pipefail

KIBANA_URL="${KIBANA_URL:-http://localhost:5601}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAX_RETRIES=30
RETRY_INTERVAL=5

# --- Step 1: Wait for Kibana to be ready ---
echo "Waiting for Kibana at $KIBANA_URL ..."
for i in $(seq 1 $MAX_RETRIES); do
  if curl -s "$KIBANA_URL/api/status" | grep -q '"overall":{"level":"available"'; then
    echo "Kibana is ready!"
    break
  fi
  if [ "$i" -eq "$MAX_RETRIES" ]; then
    echo "ERROR: Kibana not available after $((MAX_RETRIES * RETRY_INTERVAL))s"
    exit 1
  fi
  echo "  Attempt $i/$MAX_RETRIES — retrying in ${RETRY_INTERVAL}s..."
  sleep "$RETRY_INTERVAL"
done

echo ""
echo "Setting up Kibana Data Views and Dashboard..."
echo "=============================================="

# --- Helper to create a data view (idempotent) ---
create_data_view() {
  local title="$1"
  local name="$2"
  echo -n "  Creating Data View '$name' ($title)... "
  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$KIBANA_URL/api/data_views/data_view" \
    -H 'kbn-xsrf: true' \
    -H 'Content-Type: application/json' \
    -d "{
    \"data_view\": {
       \"title\": \"$title\",
       \"name\": \"$name\",
       \"timeFieldName\": \"@timestamp\"
    }
  }")
  HTTP_CODE=$(echo "$RESPONSE" | tail -1)
  if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "409" ]; then
    echo "OK (HTTP $HTTP_CODE)"
  else
    echo "WARN (HTTP $HTTP_CODE) — may already exist or check Kibana logs"
  fi
}

# --- Step 2: Create Data Views for all 3 signal types ---
echo ""
echo "Step 1/4: Creating Data Views"
echo "-----------------------------"
create_data_view "otel-traces-*"   "OTel Traces"
create_data_view "otel-logs-*"     "OTel Logs"
create_data_view "otel-metrics-*"  "OTel Metrics"

# --- Step 2: Generate Dashboard ---
echo ""
echo "Step 2/4: Generating Dashboard"
echo "------------------------------"

DASHBOARD_FILE="/tmp/dashboard.ndjson"
GENERATE_SCRIPT="$SCRIPT_DIR/generate_dashboard.py"

if [ ! -f "$GENERATE_SCRIPT" ]; then
  echo "ERROR: Dashboard generator not found at $GENERATE_SCRIPT"
  exit 1
fi

echo -n "  Generating dashboard NDJSON... "
python3 "$GENERATE_SCRIPT" > "$DASHBOARD_FILE"
echo "OK ($(wc -c < "$DASHBOARD_FILE" | tr -d ' ') bytes)"

# --- Step 3: Import Dashboard ---
echo ""
echo "Step 3/4: Importing Dashboard"
echo "-----------------------------"

echo -n "  Importing from $(basename "$DASHBOARD_FILE")... "
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$KIBANA_URL/api/saved_objects/_import?overwrite=true" \
  -H "kbn-xsrf: true" \
  --form file=@"$DASHBOARD_FILE")
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

SUCCESS_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('successCount', '?'))" 2>/dev/null || echo "?")
echo "OK (HTTP $HTTP_CODE, $SUCCESS_COUNT objects imported)"

# --- Step 4: Alerting Rules (optional) ---
echo ""
echo "Step 4/4: Setting up Alerting Rules"
echo "------------------------------------"

RULES_SCRIPT="$SCRIPT_DIR/generate_rules.py"
SKIP_RULES="${SKIP_RULES:-false}"

if [ "$SKIP_RULES" = "true" ]; then
  echo "  Skipped (SKIP_RULES=true)"
elif [ ! -f "$RULES_SCRIPT" ]; then
  echo "  WARN: Rules script not found at $RULES_SCRIPT"
  echo "  Skipping alerting rules setup"
else
  echo "  Running: python3 $(basename "$RULES_SCRIPT") --apply"
  KIBANA_URL="$KIBANA_URL" python3 "$RULES_SCRIPT" --apply 2>&1 | sed 's/^/  /'
  RULES_EXIT=$?
  if [ "$RULES_EXIT" -ne 0 ]; then
    echo "  WARN: Rules setup exited with code $RULES_EXIT"
    echo "  Dashboard is ready, but alerting rules may need manual setup."
    echo "  Run: python3 $RULES_SCRIPT --apply"
  fi
fi

# --- Done ---
echo ""
echo "=============================================="
echo "Kibana setup complete!"
echo ""
echo "  Dashboard: $KIBANA_URL/app/dashboards#/view/boilerplate-dashboard"
echo "  Discover:  $KIBANA_URL/app/discover"
echo "  Rules:     $KIBANA_URL/app/management/insightsAndAlerting/triggersActions/rules"
echo ""
echo "  Data Views created:"
echo "    - OTel Traces  -> otel-traces-*"
echo "    - OTel Logs    -> otel-logs-*"
echo "    - OTel Metrics -> otel-metrics-*"
echo ""
echo "  Note: Generate traffic first to populate the dashboard."
echo ""
