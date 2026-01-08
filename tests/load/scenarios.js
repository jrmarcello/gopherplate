import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ============================================
// CONFIGURAÇÃO
// ============================================

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Métricas customizadas
const errorRate = new Rate('errors');
const createPersonDuration = new Trend('create_person_duration');
const getPersonDuration = new Trend('get_person_duration');
// const listPersonsDuration = new Trend('list_persons_duration'); // DEPRECATED: endpoint returns 410 Gone

// ============================================
// CENÁRIOS DE TESTE
// ============================================
// Uso: k6 run --env SCENARIO=smoke tests/load/scenarios.js

const SCENARIO = __ENV.SCENARIO || 'smoke';

// Define todos os cenários disponíveis
const allScenarios = {
  // Smoke test: validação básica (500 usuários por 30s)
  smoke: {
    executor: 'constant-vus',
    vus: 500,
    duration: '30s',
    exec: 'smokeTest',
    tags: { scenario: 'smoke' },
  },

  // Load test: carga progressiva (100 → 500 → 1000 usuários)
  load: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 500 },   // Ramp-up
      { duration: '30s', target: 100 },   // Aumentar carga
      { duration: '1m', target: 2000 },   // Pico
      { duration: '30s', target: 500 },   // Reduzir
      { duration: '30s', target: 0 },     // Ramp-down
    ],
    exec: 'loadTest',
    tags: { scenario: 'load' },
  },

  // Stress test: encontrar limite do sistema
  stress: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 500 },
      { duration: '30s', target: 1000 },
      { duration: '30s', target: 1500 },
      { duration: '30s', target: 2000 },
      { duration: '30s', target: 2500 },
      { duration: '30s', target: 0 },
    ],
    exec: 'stressTest',
    tags: { scenario: 'stress' },
  },

  // Spike test: pico súbito de usuários
  spike: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '20s', target: 500 },
      { duration: '1m', target: 800 },
      { duration: '10s', target: 3000 }, // Spike!
      { duration: '1m', target: 1000 },
      { duration: '20s', target: 600 },
      { duration: '1m', target: 200 },
      { duration: '10s', target: 0 },
    ],
    exec: 'spikeTest',
    tags: { scenario: 'spike' },
  },
};

// Seleciona apenas o cenário especificado
const selectedScenario = {};
selectedScenario[SCENARIO] = allScenarios[SCENARIO];

export const options = {
  scenarios: selectedScenario,

  // Thresholds de performance
  thresholds: {
    http_req_failed: ['rate<0.01'],        // Menos de 1% de erros
    http_req_duration: ['p(95)<500'],      // 95% das requests < 500ms
    create_person_duration: ['p(95)<800'], // Create < 800ms
    get_person_duration: ['p(95)<200'],    // Get < 200ms
    // list_persons_duration: ['p(95)<300'],  // DEPRECATED: endpoint returns 410 Gone
    errors: ['rate<0.01'],
  },
};

// ============================================
// HELPERS
// ============================================

// Contador global para garantir unicidade
let globalCounter = 0;

function generateCPF() {
  // Gera CPF válido e ÚNICO para testes usando VU ID + iteration + counter
  // Isso evita colisões mesmo com milhares de VUs simultâneos
  const vuId = __VU || 0;
  const iter = __ITER || 0;
  globalCounter++;

  // Combina VU, iteração e contador para criar base única
  const uniqueBase = (vuId * 1000000 + iter * 1000 + globalCounter) % 999999999;
  const baseStr = uniqueBase.toString().padStart(9, '0');
  const cpf = baseStr.split('').map(Number);

  // Calcula primeiro dígito verificador
  let sum = 0;
  for (let i = 0; i < 9; i++) sum += cpf[i] * (10 - i);
  let d1 = 11 - (sum % 11);
  if (d1 >= 10) d1 = 0;
  cpf.push(d1);

  // Calcula segundo dígito verificador
  sum = 0;
  for (let i = 0; i < 10; i++) sum += cpf[i] * (11 - i);
  let d2 = 11 - (sum % 11);
  if (d2 >= 10) d2 = 0;
  cpf.push(d2);

  return cpf.join('');
}

function randomEmail() {
  // Usa VU ID + iteration + timestamp + random para garantir unicidade
  const vuId = __VU || 0;
  const iter = __ITER || 0;
  const timestamp = Date.now();
  const random = Math.random().toString(36).substring(2, 8);
  return `lt_${vuId}_${iter}_${timestamp}_${random}@test.com`;
}

function randomPhone() {
  // Gera telefone brasileiro válido (11 dígitos)
  const ddd = Math.floor(Math.random() * 89) + 11; // DDDs válidos: 11-99
  const number = Math.floor(Math.random() * 900000000) + 100000000;
  return `${ddd}${number}`;
}

function randomAddress() {
  const streets = ['Rua das Flores', 'Av. Paulista', 'Rua Augusta', 'Av. Brasil', 'Rua 7 de Setembro'];
  const neighborhoods = ['Centro', 'Jardins', 'Consolação', 'Pinheiros', 'Vila Mariana'];
  const cities = ['São Paulo', 'Rio de Janeiro', 'Belo Horizonte', 'Curitiba', 'Porto Alegre'];
  const states = ['SP', 'RJ', 'MG', 'PR', 'RS'];

  const idx = Math.floor(Math.random() * streets.length);
  const number = Math.floor(Math.random() * 9999) + 1;
  const zipCode = `${Math.floor(Math.random() * 90000) + 10000}-${Math.floor(Math.random() * 900) + 100}`;

  return {
    street: streets[idx],
    number: String(number),
    complement: Math.random() > 0.5 ? `Apto ${Math.floor(Math.random() * 100) + 1}` : '',
    neighborhood: neighborhoods[idx],
    city: cities[idx],
    state: states[idx],
    zip_code: zipCode,
  };
}

const headers = { 'Content-Type': 'application/json' };

// ============================================
// FUNÇÕES DE TESTE
// ============================================

function createPerson() {
  const payload = JSON.stringify({
    name: `Load Test User ${Date.now()}`,
    document: generateCPF(),
    email: randomEmail(),
    phone: randomPhone(),
    address: randomAddress(),
  });

  const res = http.post(`${BASE_URL}/people`, payload, { headers });

  createPersonDuration.add(res.timings.duration);

  const success = check(res, {
    'create: status is 201': (r) => r.status === 201,
    'create: has id': (r) => r.json('id') !== undefined,
  });

  errorRate.add(!success);

  return res.json('id');
}

function getPerson(id) {
  const res = http.get(`${BASE_URL}/people/${id}`, { headers });

  getPersonDuration.add(res.timings.duration);

  const success = check(res, {
    'get: status is 200': (r) => r.status === 200,
    'get: has name': (r) => r.json('name') !== undefined,
  });

  errorRate.add(!success);
  return success;
}

function updatePerson(id) {
  const payload = JSON.stringify({
    name: `Load Test User Updated ${Date.now()}`,
    phone: randomPhone(),
    email: randomEmail(),
    address: randomAddress(),
  });

  const res = http.put(`${BASE_URL}/people/${id}`, payload, { headers });

  const success = check(res, {
    'update: status is 200': (r) => r.status === 200,
  });

  errorRate.add(!success);
  return success;
}

function deletePerson(id) {
  const res = http.del(`${BASE_URL}/people/${id}`, null, { headers });

  const success = check(res, {
    'delete: status is 200': (r) => r.status === 200,
  });

  errorRate.add(!success);
  return success;
}

// function listPersons(page = 1, limit = 10) {
//   const res = http.get(`${BASE_URL}/people?page=${page}&limit=${limit}`, { headers });

//   listPersonsDuration.add(res.timings.duration);

//   const success = check(res, {
//     'list: status is 200': (r) => r.status === 200,
//     'list: has data array': (r) => Array.isArray(r.json('data')),
//     'list: has pagination': (r) => r.json('pagination') !== undefined,
//   });

//   errorRate.add(!success);
//   return success;
// }

function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  check(res, { 'health: ok': (r) => r.status === 200 });
}

// ============================================
// CENÁRIOS DE EXECUÇÃO
// ============================================

export function smokeTest() {
  group('Smoke Test', () => {
    healthCheck();
    sleep(0.5);

    // listPersons(1, 10); // DEPRECATED: endpoint returns 410 Gone
    // sleep(0.5);

    const id = createPerson();
    if (id) {
      sleep(0.3);
      getPerson(id);
    }
  });
  sleep(1);
}

export function loadTest() {
  group('Load Test - CRUD Operations', () => {
    // Distribuição realista: 60% reads, 25% creates, 10% updates, 5% deletes
    // (List endpoint deprecated, replaced with Get operations)
    const rand = Math.random();

    if (rand < 0.6) {
      // 60% - Create + Get (simulates read-heavy workload)
      const id = createPerson();
      if (id) {
        sleep(0.1);
        getPerson(id);
      }
    } else if (rand < 0.85) {
      // 25% - Create + Update
      const id = createPerson();
      if (id) {
        sleep(0.1);
        updatePerson(id);
      }
    } else {
      // 15% - Create + Delete (simula ciclo de vida)
      const id = createPerson();
      if (id) {
        sleep(0.1);
        deletePerson(id);
      }
    }
  });
  sleep(0.5);
}

export function stressTest() {
  group('Stress Test - Heavy Load', () => {
    const rand = Math.random();

    if (rand < 0.7) {
      // 70% - Create (heavy write load)
      createPerson();
    } else if (rand < 0.9) {
      // 20% - Create + Get
      const id = createPerson();
      if (id) {
        sleep(0.1);
        getPerson(id);
      }
    } else {
      // 10% - Health check
      healthCheck();
    }
  });
  sleep(0.2);
}

export function spikeTest() {
  group('Spike Test - Sudden Load', () => {
    const id = createPerson();
    if (id) {
      getPerson(id);
    }
    // listPersons(1, 10); // DEPRECATED: endpoint returns 410 Gone
  });
  sleep(0.3);
}

// ============================================
// SETUP E TEARDOWN
// ============================================

export function setup() {
  console.log(`🚀 Starting load test against ${BASE_URL}`);

  // Verifica se API está online
  const res = http.get(`${BASE_URL}/health`);
  if (res.status !== 200) {
    throw new Error(`API not available at ${BASE_URL}`);
  }

  console.log('✅ API is healthy, starting tests...');
  return { startTime: Date.now() };
}

export function teardown(data) {
  const duration = ((Date.now() - data.startTime) / 1000).toFixed(2);
  console.log(`✅ Load test completed in ${duration}s`);
}
