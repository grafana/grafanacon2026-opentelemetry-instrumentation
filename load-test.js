import http from 'k6/http';
import { check } from 'k6';
import exec from 'k6/execution';
import { randomItem } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// Usage:
//   k6 run load-test.js
//   k6 run -e BASE_URL=http://localhost:8080 load-test.js

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  vus: 4,
  duration: '8760h',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<2000'],
  },
};

// Seed users from db/init.sql — username-only auth, no passwords.
// Admin is excluded to keep traffic shaped like regular-user browsing.
const USERS = ['alice', 'bob', 'carla', 'david'];

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
  () => `/favorites`,                                                      // 10% — auth-only
];

function login() {
  const username = USERS[(exec.vu.idInTest - 1) % USERS.length];
  http.post(`${BASE_URL}/login`, { username });
  // k6 clears session cookies between iterations, so we log in each iteration
  // and verify the session cookie was actually set (a 200 error page from a
  // transient backend hiccup still leaves the jar empty).
  const ok = !!http.cookieJar().cookiesForURL(`${BASE_URL}/`).tapas_user;
  check(null, { 'login ok': () => ok });
  return ok;
}

export default function () {
  if (!login()) return;
  const res = http.get(`${BASE_URL}${randomItem(PATHS)()}`);
  check(res, { 'status 200': r => r.status === 200 });
}
