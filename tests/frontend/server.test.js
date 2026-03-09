'use strict';

/**
 * Frontend happy-path tests.
 * Injects a mock fetch via global.__TEST_FETCH__ so no real backend is needed.
 */

const request = require('supertest');

// ── Mock data ─────────────────────────────────────────────────────────────────

const RESTAURANT = {
  id: 'r1',
  slug: 'la_tasca_del_dragon',
  name: 'La Tasca del Dragon',
  address: 'Carrer del Rec 27',
  neighborhood: 'El Born',
  description: 'A great place.',
  hours: [{ day: 'monday', open: '13:00', close: '23:00' }],
  options: ['vegan'],
  tapas_menu: [{ name: 'Patatas Bravas', price: 6.50, options: ['vegan'] }],
  avg_rating: 4.7,
  rating_count: 5,
  photo_ids: [],
  my_rating: null,
};

const USERS = [
  { id: 'a1f3e2d4', username: 'admin', is_admin: true },
  { id: 'b2c4d5e6', username: 'alice', is_admin: false },
];

function makeResp(body, status = 200) {
  const buf = Buffer.from(JSON.stringify(body));
  return {
    status,
    ok: status >= 200 && status < 300,
    headers: { forEach: () => {}, get: () => null },
    json: () => Promise.resolve(body),
    arrayBuffer: () => Promise.resolve(buf),
  };
}

function mockFetch(url) {
  const u = url.toString();
  if (u.includes('/api/restaurants/r1/ratings')) {
    return Promise.resolve(makeResp({ ratings: [], avg_rating: 4.7, count: 0 }));
  }
  if (u.includes('/api/restaurants/r1')) {
    return Promise.resolve(makeResp(RESTAURANT));
  }
  if (u.includes('/api/restaurants')) {
    return Promise.resolve(makeResp({ restaurants: [RESTAURANT], total: 1 }));
  }
  if (u.includes('/api/users')) {
    return Promise.resolve(makeResp({ users: USERS }));
  }
  return Promise.resolve(makeResp({ error: 'not found' }, 404));
}

// Inject mock fetch before loading server
global.__TEST_FETCH__ = mockFetch;

const app = require('../../frontend/server');

afterAll(done => { delete global.__TEST_FETCH__; done(); });

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('Frontend routes', () => {

  test('GET / returns 200 with HTML', async () => {
    const res = await request(app).get('/');
    expect(res.status).toBe(200);
    expect(res.headers['content-type']).toMatch(/html/);
  });

  test('GET /search returns 200 with results', async () => {
    const res = await request(app).get('/search?q=dragon');
    expect(res.status).toBe(200);
    expect(res.text).toContain('La Tasca del Dragon');
  });

  test('GET /restaurants/:id returns 200 with restaurant name', async () => {
    const res = await request(app).get('/restaurants/r1');
    expect(res.status).toBe(200);
    expect(res.text).toContain('La Tasca del Dragon');
  });

  test('GET /login returns 200 with sign-in form', async () => {
    const res = await request(app).get('/login');
    expect(res.status).toBe(200);
    expect(res.text).toContain('Sign in');
  });

  test('GET /admin without auth returns 403', async () => {
    const res = await request(app).get('/admin');
    expect(res.status).toBe(403);
  });

  test('GET /admin with admin cookie returns 200', async () => {
    const adminUser = JSON.stringify(USERS[0]);
    const cookie = `tapas_user=${encodeURIComponent(adminUser)}`;
    const res = await request(app).get('/admin').set('Cookie', cookie);
    expect(res.status).toBe(200);
    expect(res.text).toContain('La Tasca del Dragon');
  });

});
