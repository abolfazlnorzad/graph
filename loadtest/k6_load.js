/**
 * k6 Load Test — Graph Task Manager API
 *
 * Simulates normal production traffic at a steady rate.
 * Goal: validate that the system handles expected daily load within SLA.
 *
 * Phases:
 *   1. Ramp-up    30s  → 0 → 50 VUs
 *   2. Sustain   90s  → 50 VUs constant
 *   3. Ramp-down  10s  → 50 → 0 VUs
 *
 * Usage:
 *   k6 run loadtest/k6_load.js
 *   k6 run --out json=loadtest/results_load.json loadtest/k6_load.js
 *   BASE_URL=http://host:port k6 run loadtest/k6_load.js
 */

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import {
    BASE_URL,
    HEADERS,
    generateTaskPayload,
    extractTaskId,
    DEFAULT_THRESHOLDS,
} from './config.js';

// ── Custom metrics ──────────────────────────────────────────
const errorRate = new Rate('errors');
const createDuration = new Trend('task_create_duration', true);
const getDuration = new Trend('task_get_duration', true);
const listDuration = new Trend('task_list_duration', true);
const updateDuration = new Trend('task_update_duration', true);
const deleteDuration = new Trend('task_delete_duration', true);
const cacheHits = new Counter('cache_hits');
const cacheMisses = new Counter('cache_misses');

// ── Options ─────────────────────────────────────────────────
export const options = {
    scenarios: {
        // Scenario 1: Mixed read-heavy traffic (realistic)
        mixed_load: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '30s', target: 50 },   // ramp up
                { duration: '90s', target: 50 },   // sustain
                { duration: '10s', target: 0 },    // ramp down
            ],
            exec: 'mixedScenario',
        },
    },

    thresholds: {
        ...DEFAULT_THRESHOLDS,
        task_create_duration: ['p(95)<400'],
        task_get_duration: ['p(95)<200'],
        task_list_duration: ['p(95)<500'],
        errors: ['rate<0.05'],
    },
};

// ── Shared state ────────────────────────────────────────────
const createdTaskIds = [];

// ── Scenario: Mixed read-heavy traffic ──────────────────────
export function mixedScenario() {
    // Each VU cycles through: create → get → list → update → delete
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

        sleep(0.3);
    });

    group('Get Single Task', () => {
        if (createdTaskIds.length === 0) return;

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

        sleep(0.2);
    });

    group('List Tasks', () => {
        const res = http.get(
            `${BASE_URL}/tasks?page_number=1&page_size=10&status=TODO`,
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

        sleep(0.2);
    });

    group('List Tasks with Filters', () => {
        const res = http.get(
            `${BASE_URL}/tasks?page_number=1&page_size=5&status=TODO&assignee=Alice`,
            { headers: HEADERS, tags: { name: 'GET /tasks (filtered)' }, timeout: '10s' }
        );

        check(res, {
            'list-filtered: status 200': (r) => r.status === 200,
        });
        errorRate.add(res.status !== 200);
        listDuration.add(res.timings.duration);

        sleep(0.2);
    });

    group('Update Task', () => {
        if (createdTaskIds.length === 0) return;

        const id = createdTaskIds[createdTaskIds.length - 1];

        // First get current version
        const getRes = http.get(`${BASE_URL}/tasks/${id}`, {
            headers: HEADERS,
            tags: { name: 'GET /tasks/:id (for update)' },
            timeout: '5s',
        });

        let version = 1;
        try {
            version = JSON.parse(getRes.body)?.data?.result?.version || 1;
        } catch { /* use default */ }

        const updatePayload = JSON.stringify({
            title: `updated-${Date.now()}`,
            status: 'IN_PROGRESS',
            version: version,
        });

        const res = http.patch(`${BASE_URL}/tasks/${id}`, updatePayload, {
            headers: HEADERS,
            tags: { name: 'PATCH /tasks/:id' },
            timeout: '5s',
        });

        check(res, {
            'update: status 200 or 409': (r) =>
                r.status === 200 || r.status === 409,
        });
        errorRate.add(res.status !== 200 && res.status !== 409);
        updateDuration.add(res.timings.duration);

        sleep(0.3);
    });

    group('Delete Task', () => {
        if (createdTaskIds.length === 0) return;

        const id = createdTaskIds.pop();
        const res = http.del(`${BASE_URL}/tasks/${id}`, null, {
            headers: HEADERS,
            tags: { name: 'DELETE /tasks/:id' },
            timeout: '5s',
        });

        check(res, {
            'delete: status 204': (r) => r.status === 204,
        });
        errorRate.add(res.status !== 204);
        deleteDuration.add(res.timings.duration);

        sleep(0.2);
    });

    sleep(1); // pause between iterations
}

// ── Summary handler ─────────────────────────────────────────
export function handleSummary(data) {
    const metrics = data.metrics;

    const summary = {
        timestamp: new Date().toISOString(),
        test_type: 'load',
        duration: metrics.iteration_duration?.values?.max
            ? `${(metrics.iteration_duration.values.max / 1000).toFixed(1)}s`
            : 'N/A',
        total_requests: metrics.http_reqs?.values?.count || 0,
        req_per_sec: metrics.http_reqs?.values?.rate?.toFixed(2) || '0',
        error_rate: ((metrics.errors?.values?.rate || 0) * 100).toFixed(2) + '%',
        latency: {
            avg: `${(metrics.http_req_duration?.values?.avg || 0).toFixed(1)}ms`,
            p50: `${(metrics.http_req_duration?.values?.['p(50)'] || 0).toFixed(1)}ms`,
            p95: `${(metrics.http_req_duration?.values?.['p(95)'] || 0).toFixed(1)}ms`,
            p99: `${(metrics.http_req_duration?.values?.['p(99)'] || 0).toFixed(1)}ms`,
            max: `${(metrics.http_req_duration?.values?.max || 0).toFixed(1)}ms`,
        },
        vus: {
            max: metrics.vus_max?.values?.value || 0,
        },
    };

    console.log('\n' + '═'.repeat(50));
    console.log('  LOAD TEST RESULTS');
    console.log('═'.repeat(50));
    console.log(`  Duration:       ${summary.duration}`);
    console.log(`  Total Requests: ${summary.total_requests}`);
    console.log(`  Throughput:     ${summary.req_per_sec} req/s`);
    console.log(`  Error Rate:     ${summary.error_rate}`);
    console.log(`  Max VUs:        ${summary.vus.max}`);
    console.log('─'.repeat(50));
    console.log(`  Latency (avg):  ${summary.latency.avg}`);
    console.log(`  Latency (p50):  ${summary.latency.p50}`);
    console.log(`  Latency (p95):  ${summary.latency.p95}`);
    console.log(`  Latency (p99):  ${summary.latency.p99}`);
    console.log(`  Latency (max):  ${summary.latency.max}`);
    console.log('═'.repeat(50));

    return {
        'loadtest/results_load_summary.json': JSON.stringify(summary, null, 2),
        stdout: '',
    };
}
