// Shared configuration for k6 tests
export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const HEADERS = {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
};

// Realistic task payload generator
let taskCounter = 0;
export function generateTaskPayload() {
    taskCounter++;
    const statuses = ['TODO', 'IN_PROGRESS', 'REVIEW', 'DONE'];
    const assignees = ['Alice', 'Bob', 'Charlie', 'Diana', 'Eve'];
    return JSON.stringify({
        title: `k6-task-${taskCounter}-${Date.now()}`,
        description: `Automated load test task created by k6`,
        status: statuses[taskCounter % statuses.length],
        assignee: assignees[taskCounter % assignees.length],
    });
}

// Extract ID from create response
export function extractTaskId(responseBody) {
    try {
        const body = JSON.parse(responseBody);
        return body?.data?.result?.id || body?.data?.task?.id;
    } catch {
        return null;
    }
}

// Custom thresholds shared across tests
export const DEFAULT_THRESHOLDS = {
    http_req_duration: [
        'p(50)<200',   // 50% under 200ms
        'p(95)<500',   // 95% under 500ms
        'p(99)<1000',  // 99% under 1s
    ],
    http_req_failed: ['rate<0.05'], // <5% error rate
    http_reqs: ['rate>10'],         // >10 req/s minimum
};
