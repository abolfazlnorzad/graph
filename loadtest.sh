#!/usr/bin/env bash
#
# loadtest.sh — Simple load test for Graph Task Manager API
#
# Usage:
#   ./loadtest.sh [BASE_URL] [CONCURRENCY] [REQUESTS]
#
# Prerequisites:
#   - curl (always available)
#   - hey or ab (optional, for advanced metrics)
#
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
CONCURRENCY="${2:-10}"
REQUESTS="${3:-100}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  Graph Task Manager — Load Test${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo -e "  Target:      ${GREEN}${BASE_URL}${NC}"
echo -e "  Concurrency: ${GREEN}${CONCURRENCY}${NC}"
echo -e "  Requests:    ${GREEN}${REQUESTS}${NC}"
echo ""

# Phase 1: Seed test data
echo -e "${YELLOW}[Phase 1] Seeding test tasks...${NC}"
TASK_IDS=()
for i in $(seq 1 5); do
    RESP=$(curl -s -X POST "${BASE_URL}/tasks" \
        -H "Content-Type: application/json" \
        -d "{\"title\":\"Load Test Task ${i}\",\"status\":\"TODO\",\"assignee\":\"load-tester\"}")
    ID=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['result']['id'])" 2>/dev/null || echo "")
    if [ -n "$ID" ]; then
        TASK_IDS+=("$ID")
        echo -e "  ${GREEN}✓${NC} Created task #${ID}"
    fi
done
echo ""

# Phase 2: Warm-up
echo -e "${YELLOW}[Phase 2] Warm-up (10 sequential requests)...${NC}"
for i in $(seq 1 10); do
    curl -s -o /dev/null "${BASE_URL}/tasks?page_number=1&page_size=10"
done
echo -e "  ${GREEN}✓${NC} Warm-up complete"
echo ""

# Phase 3: Load test — GET /tasks
echo -e "${YELLOW}[Phase 3] Load test: GET /tasks (list)${NC}"
echo -e "  ${CONCURRENCY} concurrent × ${REQUESTS} total"
echo ""

if command -v hey &>/dev/null; then
    echo -e "  Using ${CYAN}hey${NC}"
    hey -n "${REQUESTS}" -c "${CONCURRENCY}" "${BASE_URL}/tasks?page_number=1&page_size=10"
elif command -v ab &>/dev/null; then
    echo -e "  Using ${CYAN}ab${NC}"
    ab -n "${REQUESTS}" -c "${CONCURRENCY}" -q "${BASE_URL}/tasks?page_number=1&page_size=10"
else
    echo -e "  Using ${CYAN}curl${NC} (sequential)"
    START_TIME=$(python3 -c "import time; print(time.time())")
    for i in $(seq 1 "${REQUESTS}"); do
        curl -s -o /dev/null "${BASE_URL}/tasks?page_number=1&page_size=10"
    done
    END_TIME=$(python3 -c "import time; print(time.time())")
    DURATION=$(python3 -c "print(f'{${END_TIME} - ${START_TIME}:.2f}')")
    echo -e "  ${GREEN}✓${NC} Completed ${REQUESTS} requests in ${DURATION}s"
    echo -e "  Throughput: $(python3 -c "print(f'{${REQUESTS} / (${END_TIME} - ${START_TIME}):.1f}')") req/s"
fi
echo ""

# Phase 4: Load test — GET /tasks/:id
if [ ${#TASK_IDS[@]} -gt 0 ]; then
    TEST_ID="${TASK_IDS[0]}"
    echo -e "${YELLOW}[Phase 4] Load test: GET /tasks/${TEST_ID} (single task)${NC}"
    if command -v hey &>/dev/null; then
        hey -n "${REQUESTS}" -c "${CONCURRENCY}" "${BASE_URL}/tasks/${TEST_ID}"
    elif command -v ab &>/dev/null; then
        ab -n "${REQUESTS}" -c "${CONCURRENCY}" -q "${BASE_URL}/tasks/${TEST_ID}"
    else
        START_TIME=$(python3 -c "import time; print(time.time())")
        for i in $(seq 1 "${REQUESTS}"); do
            curl -s -o /dev/null "${BASE_URL}/tasks/${TEST_ID}"
        done
        END_TIME=$(python3 -c "import time; print(time.time())")
        DURATION=$(python3 -c "print(f'{${END_TIME} - ${START_TIME}:.2f}')")
        echo -e "  ${GREEN}✓${NC} Completed ${REQUESTS} requests in ${DURATION}s"
    fi
    echo ""
fi

# Phase 5: Cleanup
echo -e "${YELLOW}[Phase 5] Cleaning up...${NC}"
for ID in "${TASK_IDS[@]}"; do
    curl -s -o /dev/null -X DELETE "${BASE_URL}/tasks/${ID}"
done
echo -e "  ${GREEN}✓${NC} Cleaned up ${#TASK_IDS[@]} tasks"
echo ""

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  Load Test Complete${NC}"
echo -e "${CYAN}========================================${NC}"
