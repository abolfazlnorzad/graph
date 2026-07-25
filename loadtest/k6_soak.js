/**
 * k6 Soak Test — Graph Task Manager API
 *
 * Extended duration test to detect memory leaks, connection pool exhaustion,
 * and gradual degradation over time.
 *
 * Phases:
 *   1. Ramp up     20s  → 0 → 20 VUs
 *   2. Sustain     10m  → 20 VUs constant
 *   3. Ramp down   20s  → 20 → 0 VUs
 *
 * Usage:
 *   k6 run loadtest/k6_soak.js
 *   BASE_URL=http://host:port k6 run loadtest/k6_soak.js
 *
 * What to watch for:
 *   - Memory growth over time (check container stats or pprof)
 *   - Response time drift (p95 should stay stable)
 *   - Error rate increase over time
 *   - Connection pool exhaustion in PostgreSQL/Redis
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

const errorRate = new Rate('errors');
const reqDuration = new Trend('request_duration', true);
const createDuration = new Trend('task_create_duration', true);
const getDuration = new Trend('task_get_duration', true);
const listDuration = new Trend('task_list_duration', true);
const activeVUs = new Gauge('active_vus');
const totalReqs = new Counter('total_requests');

export const options = {
    scenarios: {
        soak: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '20s', target: 20 },   // ramp up
                { duration: '10m', target: 20 },   // sustain 10 minutes
                { duration: '20s', target: 0 },    // ramp down
            ],
            exec: 'soakScenario',
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<600'],
        errors: ['rate<0.03'],
        http_reqs: ['rate>10'],
    },
};

const createdTaskIds = [];

export function soakScenario() {
    activeVUs.add(__VU);
    totalReqs.add(1);

    const rand = Math.random();

    if (rand < 0.5) {
        // 50% — LIST (DB + cache)
        group('List Tasks', () => {
            const page = Math.floor(Math.random() * 10) + 1;
            const res = http.get(
                `${BASE_URL}/tasks?page_number=${page}&page_size=10`,
                { headers: HEADERS, tags: { name: 'GET /tasks' }, timeout: '10s' }
            );
            check(res, { 'list: 200': (r) => r.status === 200 });
            errorRate.add(res.status !== 200);
            listDuration.add(res.timings.duration);
            reqDuration.add(res.timings.duration);
        });
        sleep(0.3);
    } else if (rand < 0.75) {
        // 25% — GET single (cache)
        group('Get Task', () => {
            if (createdTaskIds.length > 0) {
                const id = createdTaskIds[Math.floor(Math.random() * createdTaskIds.length)];
                const res = http.get(`${BASE_URL}/tasks/${id}`, {
                    headers: HEADERS,
                    tags: { name: 'GET /tasks/:id' },
                    timeout: '5s',
                });
                check(res, { 'get: 200': (r) => r.status === 200 });
                errorRate.add(res.status !== 200);
                getDuration.add(res.timings.duration);
                reqDuration.add(res.timings.duration);
            }
        });
        sleep(0.2);
    } else if (rand < 0.9) {
        // 15% — CREATE
        group('Create Task', () => {
            const res = http.post(`${BASE_URL}/tasks`, generateTaskPayload(), {
                headers: HEADERS,
                tags: { name: 'POST /tasks' },
                timeout: '10s',
            });
            const passed = check(res, { 'create: 201': (r) => r.status === 201 });
            errorRate.add(!passed);
            createDuration.add(res.timings.duration);
            reqDuration.add(res.timings.duration);
            if (passed) {
                const id = extractTaskId(res.body);
                if (id) createdTaskIds.push(id);
            }
        });
        sleep(0.3);
    } else {
        // 10% — DELETE (keeps dataset bounded)
        group('Delete Task', () => {
            if (createdTaskIds.length > 50) {
                const id = createdTaskIds.shift();
                const res = http.del(`${BASE_URL}/tasks/${id}`, null, {
                    headers: HEADERS,
                    tags: { name: 'DELETE /tasks/:id' },
                    timeout: '5s',
                });
                check(res, { 'delete: 204': (r) => r.status === 204 });
                errorRate.add(res.status !== 204);
                reqDuration.add(res.timings.duration);
            }
        });
        sleep(0.2);
    }

    sleep(0.5);
}

export function handleSummary(data) {
    const m = data.metrics;
    const summary = {
        timestamp: new Date().toISOString(),
        test_type: 'soak',
        max_vus: m.vus_max?.values?.value || 0,
        total_requests: m.http_reqs?.values?.count || 0,
        req_per_sec: (m.http_reqs?.values?.rate || 0).toFixed(2),
        error_rate: `${((m.errors?.values?.rate || 0) * 100).toFixed(2)}%`,
        latency_avg: `${(m.http_req_duration?.values?.avg || 0).toFixed(1)}ms`,
        latency_p50: `${(m.http_req_duration?.values?.['p(50)'] || 0).toFixed(1)}ms`,
        latency_p95: `${(m.http_req_duration?.values?.['p(95)'] || 0).toFixed(1)}ms`,
        latency_p99: `${(m.http_req_duration?.values?.['p(99)'] || 0).toFixed(1)}ms`,
        latency_max: `${(m.http_req_duration?.values?.max || 0).toFixed(1)}ms`,
    };

    console.log('\n' + '═'.repeat(50));
    console.log('  SOAK TEST RESULTS');
    console.log('═'.repeat(50));
    Object.entries(summary).forEach(([k, v]) => {
        console.log(`  ${k.padEnd(20)} ${v}`);
    });
    console.log('═'.repeat(50));

    return {
        'loadtest/results_soak_summary.json': JSON.stringify(summary, null, 2),
        stdout: '',
    };
}
