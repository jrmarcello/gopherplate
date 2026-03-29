import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ============================================
// CONFIGURATION
// ============================================

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Custom metrics
const errorRate = new Rate('errors');
const createUserDuration = new Trend('create_user_duration');
const getUserDuration = new Trend('get_user_duration');
const listUsersDuration = new Trend('list_users_duration');

// ============================================
// SCENARIOS
// ============================================
// Usage: k6 run --env SCENARIO=smoke tests/load/scenarios.js

const SCENARIO = __ENV.SCENARIO || 'smoke';

const allScenarios = {
  // Smoke test: basic validation (5 users, 30s)
  smoke: {
    executor: 'constant-vus',
    vus: 5,
    duration: '30s',
    exec: 'smokeTest',
    tags: { scenario: 'smoke' },
  },

  // Load test: progressive ramp (up to 50 users)
  load: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 10 },   // Warm up
      { duration: '1m', target: 30 },    // Normal load
      { duration: '1m', target: 50 },    // Peak
      { duration: '30s', target: 10 },   // Cool down
      { duration: '30s', target: 0 },    // Ramp down
    ],
    exec: 'loadTest',
    tags: { scenario: 'load' },
  },

  // Stress test: find system limits (up to 200 users)
  stress: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 50 },
      { duration: '30s', target: 100 },
      { duration: '30s', target: 150 },
      { duration: '30s', target: 200 },
      { duration: '30s', target: 0 },
    ],
    exec: 'stressTest',
    tags: { scenario: 'stress' },
  },

  // Spike test: sudden burst (0 → 100 instantly)
  spike: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '10s', target: 5 },    // Baseline
      { duration: '5s', target: 100 },   // Spike!
      { duration: '30s', target: 100 },  // Hold spike
      { duration: '10s', target: 5 },    // Recover
      { duration: '10s', target: 0 },    // Ramp down
    ],
    exec: 'spikeTest',
    tags: { scenario: 'spike' },
  },
};

// Select only the specified scenario
const selectedScenario = {};
selectedScenario[SCENARIO] = allScenarios[SCENARIO];

export const options = {
  scenarios: selectedScenario,

  thresholds: {
    http_req_failed: ['rate<0.01'],           // Less than 1% errors
    http_req_duration: ['p(95)<500'],         // 95th percentile < 500ms
    create_user_duration: ['p(95)<800'],    // Create < 800ms
    get_user_duration: ['p(95)<200'],       // Get < 200ms
    list_users_duration: ['p(95)<300'],    // List < 300ms
    errors: ['rate<0.01'],
  },
};

// ============================================
// HELPERS
// ============================================

function randomEmail() {
  const vuId = __VU || 0;
  const iter = __ITER || 0;
  const ts = Date.now();
  const rand = Math.random().toString(36).substring(2, 8);
  return `lt_${vuId}_${iter}_${ts}_${rand}@test.com`;
}

function randomName() {
  const names = ['Alice', 'Bob', 'Carlos', 'Diana', 'Eduardo', 'Fernanda', 'Gabriel', 'Helena'];
  const surnames = ['Silva', 'Santos', 'Oliveira', 'Souza', 'Lima', 'Pereira', 'Costa', 'Ferreira'];
  const name = names[Math.floor(Math.random() * names.length)];
  const surname = surnames[Math.floor(Math.random() * surnames.length)];
  return `Load Test ${name} ${surname}`;
}

const headers = { 'Content-Type': 'application/json' };

// ============================================
// API OPERATIONS
// ============================================

function createUser() {
  const payload = JSON.stringify({
    name: randomName(),
    email: randomEmail(),
  });

  const res = http.post(`${BASE_URL}/users`, payload, { headers });

  createUserDuration.add(res.timings.duration);

  const success = check(res, {
    'create: status is 201': (r) => r.status === 201,
    'create: has data.id': (r) => {
      try { return r.json('data.id') !== undefined; } catch (e) { return false; }
    },
  });

  errorRate.add(!success);

  try {
    return res.json('data.id');
  } catch (e) {
    return null;
  }
}

function getUser(id) {
  const res = http.get(`${BASE_URL}/users/${id}`, { headers });

  getUserDuration.add(res.timings.duration);

  const success = check(res, {
    'get: status is 200': (r) => r.status === 200,
    'get: has data': (r) => {
      try { return r.json('data') !== undefined; } catch (e) { return false; }
    },
  });

  errorRate.add(!success);
  return success;
}

function listUsers(page = 1, limit = 10) {
  const res = http.get(`${BASE_URL}/users?page=${page}&limit=${limit}`, { headers });

  listUsersDuration.add(res.timings.duration);

  const success = check(res, {
    'list: status is 200': (r) => r.status === 200,
    'list: has data': (r) => {
      try { return r.json('data') !== undefined; } catch (e) { return false; }
    },
  });

  errorRate.add(!success);
  return success;
}

function updateUser(id) {
  const payload = JSON.stringify({
    name: `${randomName()} Updated`,
    email: randomEmail(),
  });

  const res = http.put(`${BASE_URL}/users/${id}`, payload, { headers });

  const success = check(res, {
    'update: status is 200': (r) => r.status === 200,
  });

  errorRate.add(!success);
  return success;
}

function deleteUser(id) {
  const res = http.del(`${BASE_URL}/users/${id}`, null, { headers });

  const success = check(res, {
    'delete: status is 200': (r) => r.status === 200,
  });

  errorRate.add(!success);
  return success;
}

function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  check(res, { 'health: ok': (r) => r.status === 200 });
}

// ============================================
// SCENARIO EXECUTORS
// ============================================

// Smoke: basic CRUD cycle validation
export function smokeTest() {
  group('Smoke Test', () => {
    healthCheck();
    sleep(0.5);

    listUsers(1, 5);
    sleep(0.5);

    const id = createUser();
    if (id) {
      sleep(0.3);
      getUser(id);
    }
  });
  sleep(1);
}

// Load: realistic traffic distribution
export function loadTest() {
  group('Load Test - CRUD Operations', () => {
    // Realistic distribution: 40% reads, 30% creates, 20% updates, 10% deletes
    const rand = Math.random();

    if (rand < 0.4) {
      // 40% - List + Get (read-heavy)
      listUsers(1, 10);
      sleep(0.2);
      const id = createUser();
      if (id) {
        sleep(0.1);
        getUser(id);
      }
    } else if (rand < 0.7) {
      // 30% - Create
      createUser();
    } else if (rand < 0.9) {
      // 20% - Create + Update
      const id = createUser();
      if (id) {
        sleep(0.1);
        updateUser(id);
      }
    } else {
      // 10% - Create + Delete
      const id = createUser();
      if (id) {
        sleep(0.1);
        deleteUser(id);
      }
    }
  });
  sleep(0.5);
}

// Stress: heavy write load to find limits
export function stressTest() {
  group('Stress Test - Heavy Load', () => {
    const rand = Math.random();

    if (rand < 0.6) {
      // 60% - Create (heavy writes)
      createUser();
    } else if (rand < 0.8) {
      // 20% - Create + Get
      const id = createUser();
      if (id) {
        sleep(0.1);
        getUser(id);
      }
    } else if (rand < 0.95) {
      // 15% - List
      listUsers(1, 20);
    } else {
      // 5% - Health check
      healthCheck();
    }
  });
  sleep(0.2);
}

// Spike: sudden burst of traffic
export function spikeTest() {
  group('Spike Test - Sudden Load', () => {
    const id = createUser();
    if (id) {
      getUser(id);
    }
    listUsers(1, 5);
  });
  sleep(0.3);
}

// ============================================
// SETUP & TEARDOWN
// ============================================

export function setup() {
  console.log(`Starting load test against ${BASE_URL}`);
  console.log(`Scenario: ${SCENARIO}`);

  const res = http.get(`${BASE_URL}/health`);
  if (res.status !== 200) {
    throw new Error(`API not available at ${BASE_URL} (status: ${res.status})`);
  }

  console.log('API is healthy, starting tests...');
  return { startTime: Date.now() };
}

export function teardown(data) {
  const duration = ((Date.now() - data.startTime) / 1000).toFixed(2);
  console.log(`Load test completed in ${duration}s`);
}
