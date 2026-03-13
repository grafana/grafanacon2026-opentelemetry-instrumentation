'use strict';

const express      = require('express');
const path         = require('path');
const cookieParser = require('cookie-parser');
const ejsLayouts   = require('express-ejs-layouts');
const crypto       = require('crypto');
const winston  = require('winston');

const logger = winston.createLogger({
  level: process.env.LOG_LEVEL || 'info',
  format: winston.format.json(),
  defaultMeta: { service: 'tapas-frontend' },
  transports: [new winston.transports.Console()],
});

// CHAOS: blocks the Node.js event loop for ~1-2s per request on the search
// route. Because Node is single-threaded, every concurrent request stalls
// until the sync work finishes — the more traffic, the worse it gets.
function chaosSlowNode() {
  const v = process.env.CHAOS_MODE;
  if (v === 'true' || v === '1') {
    crypto.pbkdf2Sync('chaos', 'salt', 2000000, 64, 'sha512');
  }
}

const app = express();
const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8080';

app.set('view engine', 'ejs');
app.set('views', path.join(__dirname, 'views'));
app.set('layout', 'layout');
app.use(ejsLayouts);
app.use(express.static(path.join(__dirname, 'public')));
app.use(express.urlencoded({ extended: true }));
app.use(cookieParser());

// ── Helpers ──────────────────────────────────────────────────────────────────

const NEIGHBORHOODS = ['Gràcia', 'El Born', 'Barceloneta', 'Eixample', 'Poble Sec', 'Barri Gòtic'];

function safeParseArray(str) {
  if (!str) return [];
  try { return JSON.parse(str); } catch { return []; }
}

function toArray(v) {
  return Array.isArray(v) ? v : (v ? [v] : []);
}

function renderPage(res, view, locals) {
  res.render(view, { title: 'Barcelona Tapas Finder', ...locals });
}

async function backendGet(path, headers = {}) {
  return fetch(`${BACKEND_URL}${path}`, { headers });
}

async function backendPost(path, body, headers = {}) {
  return fetch(`${BACKEND_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...headers },
    body: JSON.stringify(body),
  });
}

async function backendPut(path, body, headers = {}) {
  return fetch(`${BACKEND_URL}${path}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...headers },
    body: JSON.stringify(body),
  });
}

async function backendDelete(path, headers = {}) {
  return fetch(`${BACKEND_URL}${path}`, { method: 'DELETE', headers });
}

// Middleware: log incoming requests
app.use((req, res, next) => {
  res.on('finish', () => {
    logger.info('request', { method: req.method, path: req.path, status: res.statusCode });
  });
  next();
});

// Middleware: attach currentUser to all requests
app.use((req, res, next) => {
  try {
    req.currentUser = req.cookies.tapas_user ? JSON.parse(req.cookies.tapas_user) : null;
  } catch {
    req.currentUser = null;
  }
  res.locals.currentUser = req.currentUser;
  next();
});

// Proxy /api/* to backend — lets browser fetch /api/... without CORS issues,
// and also lets the photo <img> tag work via the same origin.
app.use('/api', async (req, res) => {
  const url = `${BACKEND_URL}/api${req.url}`;
  const headers = { ...req.headers };
  delete headers['host'];
  if (req.currentUser) {
    headers['user-id'] = req.currentUser.id;
  }

  try {
    const upstream = await fetch(url, {
      method: req.method,
      headers,
      body: ['GET', 'HEAD'].includes(req.method) ? undefined : req,
      duplex: 'half',
    });
    res.status(upstream.status);
    upstream.headers.forEach((v, k) => {
      if (!['transfer-encoding', 'connection'].includes(k.toLowerCase())) {
        res.setHeader(k, v);
      }
    });
    const buf = await upstream.arrayBuffer();
    res.end(Buffer.from(buf));
  } catch (err) {
    logger.error('backend proxy error', { url, error: err.message });
    res.status(502).json({ error: 'backend unavailable' });
  }
});

// ── Public routes ─────────────────────────────────────────────────────────────

// Home — top-rated restaurants
app.get('/', async (req, res) => {
  try {
    const r = await backendGet('/api/restaurants?min_rating=0');
    const data = await r.json();
    const restaurants = (data.restaurants || []).slice(0, 6);
    renderPage(res, 'index', { restaurants });
  } catch {
    renderPage(res, 'index', { restaurants: [] });
  }
});

// Search
app.get('/search', async (req, res) => {
  chaosSlowNode();
  const { q, neighborhood, min_rating, open_at, options } = req.query;
  const params = new URLSearchParams();
  if (q)            params.set('q', q);
  if (neighborhood) params.set('neighborhood', neighborhood);
  if (min_rating)   params.set('min_rating', min_rating);
  if (open_at)      params.set('open_at', open_at);
  if (options)      params.set('options', options);

  try {
    const r = await backendGet(`/api/restaurants?${params}`);
    const data = await r.json();
    renderPage(res, 'search', {
      restaurants: data.restaurants || [],
      filters: req.query,
      neighborhoods: NEIGHBORHOODS,
      error: null,
    });
  } catch {
    renderPage(res, 'search', {
      restaurants: [],
      filters: req.query,
      neighborhoods: NEIGHBORHOODS,
      error: 'Could not reach the backend.',
    });
  }
});

// Restaurant detail
app.get('/restaurants/:id', async (req, res) => {
  try {
    const headers = {};
    if (req.currentUser) headers['user-id'] = req.currentUser.id;

    const [rRes, ratingsRes] = await Promise.all([
      backendGet(`/api/restaurants/${req.params.id}`, headers),
      backendGet(`/api/restaurants/${req.params.id}/ratings`),
    ]);

    if (rRes.status === 404) {
      return renderPage(res.status(404), 'error', { status: 404, message: 'Restaurant not found.' });
    }
    if (!rRes.ok) {
      const err = await rRes.json().catch(() => ({}));
      return renderPage(res.status(rRes.status), 'error', {
        status: rRes.status,
        message: err.error || 'Could not load restaurant.',
      });
    }

    const restaurant = await rRes.json();
    const ratingsData = await ratingsRes.json();

    renderPage(res, 'restaurant', {
      restaurant,
      ratings: ratingsData.ratings || [],
    });
  } catch {
    renderPage(res.status(502), 'error', { status: 502, message: 'Could not load restaurant.' });
  }
});

// Favourites — requires logged-in user
app.get('/favorites', async (req, res) => {
  if (!req.currentUser) {
    return res.redirect('/login');
  }
  try {
    const r = await backendGet(
      `/api/users/me/favorites`,
      { 'user-id': req.currentUser.id },
    );
    const data = await r.json();
    renderPage(res, 'favorites', {
      restaurants: data.restaurants || [],
      error: null,
    });
  } catch {
    renderPage(res, 'favorites', { restaurants: [], error: 'Could not load favourites.' });
  }
});

// Login — GET
app.get('/login', (req, res) => {
  renderPage(res, 'login', { error: null });
});

// Login — POST (look up by username, set cookie)
app.post('/login', async (req, res) => {
  const { username } = req.body;
  if (!username) {
    return renderPage(res, 'login', { error: 'Please enter your username.' });
  }
  try {
    const r = await backendGet(`/api/users/by-username/${encodeURIComponent(username)}`);
    if (r.status === 404) {
      return renderPage(res, 'login', { error: 'User not found.' });
    }
    if (!r.ok) {
      return renderPage(res, 'login', { error: 'Login failed. Please try again.' });
    }
    const user = await r.json();
    const encoded = encodeURIComponent(JSON.stringify(user));
    res.setHeader('Set-Cookie', `tapas_user=${encoded}; Path=/; HttpOnly; SameSite=Lax`);
    res.redirect('/');
  } catch {
    renderPage(res, 'login', { error: 'Could not reach the backend.' });
  }
});

// Signup — GET
app.get('/signup', (_req, res) => {
  renderPage(res, 'signup', { error: null });
});

// Signup — POST (create user, then log in)
app.post('/signup', async (req, res) => {
  const { username } = req.body;
  if (!username) {
    return renderPage(res, 'signup', { error: 'Username is required.' });
  }
  try {
    const r = await backendPost('/api/users', { username });
    if (r.status === 409) {
      return renderPage(res, 'signup', { error: 'Username already taken.' });
    }
    if (!r.ok) {
      return renderPage(res, 'signup', { error: 'Could not create account. Please try again.' });
    }
    const user = await r.json();
    const encoded = encodeURIComponent(JSON.stringify(user));
    res.setHeader('Set-Cookie', `tapas_user=${encoded}; Path=/; HttpOnly; SameSite=Lax`);
    res.redirect('/');
  } catch {
    renderPage(res, 'signup', { error: 'Could not reach the backend.' });
  }
});

// Logout
app.get('/logout', (req, res) => {
  res.setHeader('Set-Cookie', 'tapas_user=; Path=/; Max-Age=0');
  res.redirect('/login');
});

// ── Admin routes ──────────────────────────────────────────────────────────────

function requireAdmin(req, res, next) {
  if (!req.currentUser || !req.currentUser.is_admin) {
    return renderPage(res.status(403), 'error', {
      status: 403,
      message: 'Admin access required.',
    });
  }
  next();
}

app.get('/admin', requireAdmin, async (req, res) => {
  const r = await backendGet('/api/restaurants');
  const data = await r.json();
  renderPage(res, 'admin/index', {
    restaurants: data.restaurants || [],
    message: req.query.message || null,
  });
});

app.get('/admin/restaurants/new', requireAdmin, (req, res) => {
  renderPage(res, 'admin/form', { restaurant: null, error: null });
});

app.post('/admin/restaurants/new', requireAdmin, async (req, res) => {
  const { name, address, neighborhood, description, options, hours, tapas_menu } = req.body;
  const body = { name, address, neighborhood, description, options: toArray(options), hours: safeParseArray(hours), tapas_menu: safeParseArray(tapas_menu) };

  const r = await backendPost('/api/restaurants', body, { 'user-id': req.currentUser.id });

  if (!r.ok) {
    const err = await r.json().catch(() => ({}));
    return renderPage(res, 'admin/form', { restaurant: null, error: err.error || 'Could not create restaurant.' });
  }

  const created = await r.json();
  res.redirect(`/admin?message=Restaurant "${created.name}" created.`);
});

app.get('/admin/restaurants/:id/edit', requireAdmin, async (req, res) => {
  const r = await backendGet(`/api/restaurants/${req.params.id}`, { 'user-id': req.currentUser.id });
  if (r.status === 404) return renderPage(res.status(404), 'error', { status: 404, message: 'Not found.' });
  const restaurant = await r.json();
  renderPage(res, 'admin/form', { restaurant, error: null });
});

app.post('/admin/restaurants/:id/edit', requireAdmin, async (req, res) => {
  const { id } = req.params;
  const { name, address, neighborhood, description, options, hours, tapas_menu } = req.body;

  const body = {};
  if (name)         body.name = name;
  if (address)      body.address = address;
  if (neighborhood) body.neighborhood = neighborhood;
  if (description)  body.description = description;
  if (options)      body.options = toArray(options);
  if (hours)        body.hours = safeParseArray(hours);
  if (tapas_menu)   body.tapas_menu = safeParseArray(tapas_menu);

  const r = await backendPut(`/api/restaurants/${id}`, body, { 'user-id': req.currentUser.id });

  if (!r.ok) {
    const err = await r.json().catch(() => ({}));
    const rest = await backendGet(`/api/restaurants/${id}`, { 'user-id': req.currentUser.id });
    const restaurant = await rest.json();
    return renderPage(res, 'admin/form', { restaurant, error: err.error || 'Could not update.' });
  }

  res.redirect('/admin?message=Restaurant updated.');
});

app.post('/admin/restaurants/:id/delete', requireAdmin, async (req, res) => {
  await backendDelete(`/api/restaurants/${req.params.id}`, { 'user-id': req.currentUser.id });
  res.redirect('/admin?message=Restaurant deleted.');
});

app.post('/admin/restaurants/:id/photos/:photoId/delete', requireAdmin, async (req, res) => {
  await backendDelete(
    `/api/restaurants/${req.params.id}/photos/${req.params.photoId}`,
    { 'user-id': req.currentUser.id },
  );
  res.redirect(`/admin/restaurants/${req.params.id}/edit`);
});

// ── 404 catch-all ─────────────────────────────────────────────────────────────
app.use((req, res) => {
  renderPage(res.status(404), 'error', { status: 404, message: 'Page not found.' });
});

// ── Start ──────────────────────────────────────────────────────────────────────
const PORT = process.env.PORT || 3000;

if (require.main === module) {
  app.listen(PORT, () => logger.info(`frontend listening on :${PORT}`));
}

module.exports = app;
