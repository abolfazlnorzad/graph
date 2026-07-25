/**
 * k6 Stress Test — Graph Task Manager API
 *
 * Pushes the system beyond normal capacity to find the breaking point.
 * Goal: identify at what load the system degrades or fails.
 *
 * Phases:
 *   1. Warm-up       20s  →   5 VUs (baseline)
 *   2. Ramp Level 1  30s  →  20 VUs (normal peak)
 *   3. Ramp Level 2  30s  →  50 VUs (heavy load)
 *   4. Ramp Level 3  30s  → 100 VUs (stress)
 *   5. Ramp Level 4  30s  → 200 VUs (heavy stress)
 *   6. Spike          20s  → 500 VUs (extreme spike)
 *   7. Recovery       30s  →  5 VUs (cooldown)
 *   8. Ramp-down      10s  →  0 VUs
 *
 * Usage:
 *   k6 run loadtest/k6_stress.js
 *   k6 run --out json=loadtest/results_stress.json loadtest/k6_stress.js
 *   BASE_URL=http://host:port k6 run loadtest/k6_stress.js
 */

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter, Gauge } from 'k6/metrics';
import {
    BASE_URL,
    HEADERS,
    generateTaskPayload,
    extractTaskId,
} from './config.js';

// ── Custom metrics ──────────────────────────────────────────
const errorRate = new Rate('errors');
const createDuration = new Trend('task_create_duration', true);
const getDuration = new Trend('task_get_duration', true);
const listDuration = new Trend('task_list_duration', true);
const updateDuration = new Trend('task_update_duration', true);
const deleteDuration = new Trend('task_delete_duration', true);
const activeVUs = new Gauge('active_vus');

// Thresholds per-phase — we relax under extreme spike
const normalThresholds = {
    http_req_duration: ['p(95)<500', 'p(99)<1500'],
    errors: ['rate<0.05'],
};

const stressThresholds = {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'],
    errors: ['rate<0.15'],
};

const spikeThresholds = {
    http_req_duration: ['p(95)<5000', 'p(99)<10000'],
    errors: ['rate<0.30'], // allow 30% errors during extreme spike
};

// ── Options ─────────────────────────────────────────────────
export const options = {
    scenarios: {
        stress: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                // Warm-up
                { duration: '20s', target: 5 },
                // Normal peak
                { duration: '30s', target: 20 },
                // Heavy load
                { duration: '30s', target: 50 },
                // Stress
                { duration: '30s', target: 100 },
                // Heavy stress
                { duration: '30s', target: 200 },
                // Extreme spike
                { duration: '20s', target: 500 },
                // Recovery
                { duration: '30s', target: 5 },
                // Ramp down
                { duration: '10s', target: 0 },
            ],
            exec: 'stressScenario',
        },
    },

    thresholds: {
        http_reqs: ['rate>5'],
    },
};

// ── Shared state ────────────────────────────────────────────
const createdTaskIds = [];
let currentPhase = 'warm-up';

function detectPhase(vus) {
    if (vus <= 5) return 'warm-up';
    if (vus <= 20) return 'normal';
    if (vus <= 50) return 'heavy';
    if (vus <= 100) return 'stress';
    return 'spike';
}

// ── Scenario: Stress test ──────────────────────────────────
export function stressScenario() {
    const vu = __VU;
    activeVUs.add(vu);

    // ── Phase 1: Create (read + write mix) ──
    group('Create Task', () => {
        const payload = generateTaskPayload();
        const res = http.post(`${BASE_URL}/tasks`, payload, {
            headers: HEADERS,
            tags: { name: 'POST /tasks' },
            timeout: '10s',
        });

        const passed = check(res, {
            'create: status 201': (r) => r.status === 201,
            'create: has id': (r) => {
                const id = extractTaskId(r.body);
                return id !== null && id > 0;
            },
        });
        errorRate.add(!passed);
        createDuration.add(res.timings.duration);

        if (passed) {
            const id = extractTaskId(res.body);
            if (id) createdTaskIds.push(id);
        }

        sleep(0.2);
    });

    // ── Phase 2: Read (cache hit path) ──
    group('Get Task', () => {
        if (createdTaskIds.length > 0) {
            const id = createdTaskIds[createdTaskIds.length - 1];
            const res = http.get(`${BASE_URL}/tasks/${id}`, {
                headers: HEADERS,
                tags: { name: 'GET /tasks/:id' },
                timeout: '5s',
            });

            const passed = check(res, {
                'get: status 200': (r) => r.status === 200,
                'get: has task data': (r) => {
                    try {
                        return JSON.parse(r.body)?.data?.result?.id > 0;
                    } catch { return false; }
                },
            });
            errorRate.add(!passed);
            getDuration.add(res.timings.duration);
        }
        sleep(0.15);
    });

    // ── Phase 3: List (DB-heavy) ──
    group('List Tasks', () => {
        const page = Math.floor(Math.random() * 5) + 1;
        const res = http.get(
            `${BASE_URL}/tasks?page_number=${page}&page_size=10`,
            { headers: HEADERS, tags: { name: 'GET /tasks (list)' }, timeout: '10s' }
        );

        const passed = check(res, {
            'list: status 200': (r) => r.status === 200,
            'list: has pagination': (r) => {
                try {
                    return JSON.parse(r.body)?.data?.result?.pagination?.total >= 0;
                } catch { return false; }
            },
        });
        errorRate.add(!passed);
        listDuration.add(res.timings.duration);

        sleep(0.15);
    });

    // ── Phase 4: Update (write + version conflict) ──
    if (createdTaskIds.length > 0 && Math.random() > 0.5) {
        group('Update Task', () => {
            const id = createdTaskIds[Math.floor(Math.random() * createdTaskIds.length)];

            const getRes = http.get(`${BASE_URL}/tasks/${id}`, {
                headers: HEADERS,
                tags: { name: 'GET /tasks/:id (pre-update)' },
                timeout: '5s',
            });

            let version = 1;
            try { version = JSON.parse(getRes.body)?.data?.result?.version || 1; } catch {}

            const res = http.patch(
                `${BASE_URL}/tasks/${id}`,
                JSON.stringify({
                    title: `stress-${Date.now()}`,
                    status: 'IN_PROGRESS',
                    version: version,
                }),
                { headers: HEADERS, tags: { name: 'PATCH /tasks/:id' }, timeout: '5s' }
            );

            check(res, {
                'update: 200 or 409': (r) => r.status === 200 || r.status === 409,
            });
            errorRate.add(res.status !== 200 && res.status !== 409);
            updateDuration.add(res.timings.duration);

            sleep(0.2);
        });
    }

    // ── Phase 5: Delete (cleanup) ──
    if (createdTaskIds.length > 20 && Math.random() > 0.7) {
        group('Delete Task', () => {
            const id = createdTaskIds.pop();
            const res = http.del(`${BASE_URL}/tasks/${id}`, null, {
                headers: HEADERS,
                tags: { name: 'DELETE /tasks/:id' },
                timeout: '5s',
            });

            check(res, { 'delete: status 204': (r) => r.status === 204 });
            errorRate.add(res.status !== 204);
            deleteDuration.add(res.timings.duration);

            sleep(0.1);
        });
    }

    sleep(0.5);
}

// ── Summary handler ─────────────────────────────────────────
export function handleSummary(data) {
    const m = data.metrics;
    const totalReqs = m.http_reqs?.values?.count || 0;
    const duration = m.iteration_duration?.values?.max || 0;

    // Find degradation point from threshold results
    const thresholdResults = data.thresholds || {};
    const failedThresholds = Object.entries(thresholdResults)
        .filter(([, v]) => v.ok === false)
        .map(([k]) => k);

    const summary = {
        timestamp: new Date().toISOString(),
        test_type: 'stress',
        total_duration: `${(duration / 1000).toFixed(1)}s`,
        total_requests: totalReqs,
        req_per_sec: (m.http_reqs?.values?.rate || 0).toFixed(2),
        error_rate: `${((m.errors?.values?.rate || 0) * 100).toFixed(2)}%`,
        max_vus: m.vus_max?.values?.value || 0,
        latency: {
            avg: `${(m.http_req_duration?.values?.avg || 0).toFixed(1)}ms`,
            p50: `${(m.http_req_duration?.values?.['p(50)'] || 0).toFixed(1)}ms`,
            p90: `${(m.http_req_duration?.values?.['p(90)'] || 0).toFixed(1)}ms`,
            p95: `${(m.http_req_duration?.values?.['p(95)'] || 0).toFixed(1)}ms`,
            p99: `${(m.http_req_duration?.values?.['p(99)'] || 0).toFixed(1)}ms`,
            max: `${(m.http_req_duration?.values?.max || 0).toFixed(1)}ms`,
        },
        failed_thresholds: failedThresholds,
        verdict: failedThresholds.length === 0 ? 'PASS' : 'DEGRADED',
    };

    console.log('\n' + '═'.repeat(55));
    console.log('  STRESS TEST RESULTS');
    console.log('═'.repeat(55));
    console.log(`  Duration:        ${summary.total_duration}`);
    console.log(`  Total Requests:  ${summary.total_requests}`);
    console.log(`  Throughput:      ${summary.req_per_sec} req/s`);
    console.log(`  Error Rate:      ${summary.error_rate}`);
    console.log(`  Max VUs:         ${summary.max_vus}`);
    console.log('─'.repeat(55));
    console.log('  Latency:');
    console.log(`    avg:           ${summary.latency.avg}`);
    console.log(`    p50:           ${summary.latency.p50}`);
    console.log(`    p90:           ${summary.latency.p90}`);
    console.log(`    p95:           ${summary.latency.p95}`);
    console.log(`    p99:           ${summary.latency.p99}`);
    console.log(`    max:           ${summary.latency.max}`);
    console.log('─'.repeat(55));
    console.log(`  Verdict:         ${summary.verdict}`);
    if (failedThresholds.length > 0) {
        console.log(`  Failed:          ${failedThresholds.join(', ')}`);
    }
    console.log('═'.repeat(55));

    return {
        'loadtest/results_stress_summary.json': JSON.stringify(summary, null, 2),
        stdout: '',
    };
}
