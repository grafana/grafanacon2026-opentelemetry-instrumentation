import http from 'k6/http';
import { check } from 'k6';
import { randomItem } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// Usage:
//   k6 run load-test.js
//   k6 run -e BASE_URL=http://localhost:8080 load-test.js

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  vus: 4,
  duration: '5m',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<2000'],
  },
};

const RESTAURANTS  = ['f6081920', '07192a31', '182a3b42', '293b4c53', '3a4c5d64', '4b5d6e75', '5c6e7f86', '6d7f8097', '7e8091a8', '8f91a2b9'];
const NEIGHBORHOODS = ['Gràcia', 'El Born', 'Barceloneta', 'Eixample', 'Poble Sec', 'Barri Gòtic'];
const TERMS        = ['patatas', 'croquetas', 'anchoas', 'vermouth', 'bacallà', 'gambas', 'vegan', 'pintxo', 'tortilla', 'boquerones'];
const RATINGS      = ['3.5', '4.0', '4.5'];

// Repetition = weight; evaluated lazily per iteration via functions
const PATHS = [
  () => `/search?q=${encodeURIComponent(randomItem(TERMS))}`,              // 30%
  () => `/search?q=${encodeURIComponent(randomItem(TERMS))}`,
  () => `/search?q=${encodeURIComponent(randomItem(TERMS))}`,
  () => `/search?neighborhood=${encodeURIComponent(randomItem(NEIGHBORHOODS))}`, // 20%
  () => `/search?neighborhood=${encodeURIComponent(randomItem(NEIGHBORHOODS))}`,
  () => `/search?q=${encodeURIComponent(randomItem(TERMS))}&neighborhood=${encodeURIComponent(randomItem(NEIGHBORHOODS))}`, // 10%
  () => `/search?min_rating=${randomItem(RATINGS)}`,                       // 10%
  () => `/search?q=${encodeURIComponent(randomItem(TERMS))}&min_rating=${randomItem(RATINGS)}`, // 10%
  () => `/restaurants/${randomItem(RESTAURANTS)}`,                         // 20%
  () => `/restaurants/${randomItem(RESTAURANTS)}`,
];

export default function () {
  const res = http.get(`${BASE_URL}${randomItem(PATHS)()}`);
  check(res, { 'status 200': r => r.status === 200 });
}
