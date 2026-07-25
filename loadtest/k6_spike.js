/**
 * k6 Spike Test — Graph Task Manager API
 *
 * Simulates sudden traffic spikes (flash sale, viral post, etc.)
 * Tests: does the system recover after a massive burst?
 *
 * Phases:
 *   1. Baseline    30s  →  5 VUs (normal traffic)
 *   2. Spike       10s  → 300 VUs (sudden burst)
 *   3. Sustain     30s  → 300 VUs (keep pressure)
 *   4. Recovery    30s  →  5 VUs (post-spike)
 *   5. Cooldown    20s  →  0 VUs
 *
 * Usage:
 *   k6 run loadtest/k6_spike.js
 *   BASE_URL=http://host:port k6 run loadtest/k6_spike.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Gauge } from 'k6/metrics';
import {
    BASE_URL,
    HEADERS,
    generateTaskPayload,
    extractTaskId,
} from './config.js';

const errorRate = new Rate('errors');
const reqDuration = new Trend('request_duration', true);
const activeVUs = new Gauge('active_vus');

export const options = {
    scenarios: {
        spike: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '30s', target: 5 },     // baseline
                { duration: '5s', target: 300 },     // spike ramp
                { duration: '30s', target: 300 },    // sustain spike
                { duration: '5s', target: 5 },       // recover ramp
                { duration: '30s', target: 5 },      // post-spike
                { duration: '10s', target: 0 },      // cooldown
            ],
            exec: 'spikeScenario',
        },
    },
    thresholds: {
        http_reqs: ['rate>10'],
        // Relax thresholds — spike test expects degradation
        http_req_duration: ['p(95)<8000'],
        errors: ['rate<0.40'],
    },
};

const createdTaskIds = [];

export function spikeScenario() {
    activeVUs.add(__VU);

    // Alternate between read-heavy and write operations
    const rand = Math.random();

    if (rand < 0.6) {
        // 60% reads
        if (rand < 0.3 && createdTaskIds.length > 0) {
            // GET single
            const id = createdTaskIds[createdTaskIds.length - 1];
            const res = http.get(`${BASE_URL}/tasks/${id}`, {
                headers: HEADERS,
                tags: { name: 'GET /tasks/:id' },
                timeout: '15s',
            });
            check(res, { 'get: ok': (r) => r.status === 200 });
            errorRate.add(res.status !== 200);
            reqDuration.add(res.timings.duration);
        } else {
            // LIST
            const res = http.get(
                `${BASE_URL}/tasks?page_number=1&page_size=10`,
                { headers: HEADERS, tags: { name: 'GET /tasks' }, timeout: '15s' }
            );
            check(res, { 'list: ok': (r) => r.status === 200 });
            errorRate.add(res.status !== 200);
            reqDuration.add(res.timings.duration);
        }
    } else if (rand < 0.85) {
        // 25% creates
        const res = http.post(`${BASE_URL}/tasks`, generateTaskPayload(), {
            headers: HEADERS,
            tags: { name: 'POST /tasks' },
            timeout: '15s',
        });
        const passed = check(res, { 'create: ok': (r) => r.status === 201 });
        errorRate.add(!passed);
        reqDuration.add(res.timings.duration);
        if (passed) {
            const id = extractTaskId(res.body);
            if (id) createdTaskIds.push(id);
        }
    } else {
        // 15% deletes
        if (createdTaskIds.length > 0) {
            const id = createdTaskIds.pop();
            const res = http.del(`${BASE_URL}/tasks/${id}`, null, {
                headers: HEADERS,
                tags: { name: 'DELETE /tasks/:id' },
                timeout: '15s',
            });
            check(res, { 'delete: ok': (r) => r.status === 204 });
            errorRate.add(res.status !== 204);
            reqDuration.add(res.timings.duration);
        }
    }

    sleep(Math.random() * 0.5 + 0.1); // 100-600ms think time
}

export function handleSummary(data) {
    const m = data.metrics;
    const summary = {
        timestamp: new Date().toISOString(),
        test_type: 'spike',
        max_vus: m.vus_max?.values?.value || 0,
        total_requests: m.http_reqs?.values?.count || 0,
        req_per_sec: (m.http_reqs?.values?.rate || 0).toFixed(2),
        error_rate: `${((m.errors?.values?.rate || 0) * 100).toFixed(2)}%`,
        latency_p50: `${(m.http_req_duration?.values?.['p(50)'] || 0).toFixed(1)}ms`,
        latency_p95: `${(m.http_req_duration?.values?.['p(95)'] || 0).toFixed(1)}ms`,
        latency_p99: `${(m.http_req_duration?.values?.['p(99)'] || 0).toFixed(1)}ms`,
        latency_max: `${(m.http_req_duration?.values?.max || 0).toFixed(1)}ms`,
    };

    console.log('\n' + '═'.repeat(50));
    console.log('  SPIKE TEST RESULTS');
    console.log('═'.repeat(50));
    Object.entries(summary).forEach(([k, v]) => {
        console.log(`  ${k.padEnd(20)} ${v}`);
    });
    console.log('═'.repeat(50));

    return {
        'loadtest/results_spike_summary.json': JSON.stringify(summary, null, 2),
        stdout: '',
    };
}
