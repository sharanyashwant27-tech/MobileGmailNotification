const API = '/api/v1';

const state = {
  accessToken: localStorage.getItem('access_token') || '',
  refreshToken: localStorage.getItem('refresh_token') || '',
  user: null,
};

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => [...document.querySelectorAll(sel)];

function saveTokens(access, refresh) {
  state.accessToken = access;
  state.refreshToken = refresh;
  localStorage.setItem('access_token', access);
  localStorage.setItem('refresh_token', refresh);
}

function clearTokens() {
  state.accessToken = '';
  state.refreshToken = '';
  state.user = null;
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
}

async function api(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (state.accessToken) headers.Authorization = `Bearer ${state.accessToken}`;
  let res = await fetch(`${API}${path}`, { ...options, headers });
  if (res.status === 401 && state.refreshToken) {
    const refreshed = await refreshSession();
    if (refreshed) {
      headers.Authorization = `Bearer ${state.accessToken}`;
      res = await fetch(`${API}${path}`, { ...options, headers });
    }
  }
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(body?.error?.message || `Request failed (${res.status})`);
  }
  return body.data;
}

async function refreshSession() {
  try {
    const res = await fetch(`${API}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: state.refreshToken }),
    });
    const body = await res.json();
    if (!res.ok) return false;
    saveTokens(body.data.access_token, body.data.refresh_token);
    state.user = body.data.user;
    return true;
  } catch {
    return false;
  }
}

function showAuth(error = '') {
  $('#auth-view').classList.remove('hidden');
  $('#main-view').classList.add('hidden');
  const el = $('#auth-error');
  if (error) {
    el.hidden = false;
    el.textContent = error;
  } else {
    el.hidden = true;
  }
}

function showMain() {
  $('#auth-view').classList.add('hidden');
  $('#main-view').classList.remove('hidden');
  $('#user-label').textContent = state.user?.email || '';
  applyDark(!!state.user?.dark_mode);
}

function applyDark(on) {
  document.documentElement.classList.toggle('dark', on);
  const cb = $('#dark-mode');
  if (cb) cb.checked = on;
}

async function bootstrap() {
  if (!state.accessToken) {
    showAuth();
    return;
  }
  try {
    state.user = await api('/auth/me');
    showMain();
    await Promise.all([loadNotifications(), loadAccounts(), loadSettings()]);
  } catch (e) {
    clearTokens();
    showAuth(e.message);
  }
}

$$('.tab').forEach((btn) => {
  btn.addEventListener('click', () => {
    $$('.tab').forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');
    const tab = btn.dataset.tab;
    $('#login-form').classList.toggle('hidden', tab !== 'login');
    $('#register-form').classList.toggle('hidden', tab !== 'register');
    $('#login-extra').classList.toggle('hidden', tab !== 'login');
  });
});

let qrPollTimer = null;
let qrSessionId = null;

function closeQRModal() {
  if (qrPollTimer) {
    clearInterval(qrPollTimer);
    qrPollTimer = null;
  }
  qrSessionId = null;
  $('#qr-modal').classList.add('hidden');
}

async function renderQR(scanURL) {
  const box = $('#qr-code');
  box.innerHTML = '';
  if (window.QRCode && typeof QRCode.toCanvas === 'function') {
    const canvas = document.createElement('canvas');
    await QRCode.toCanvas(canvas, scanURL, {
      width: 200,
      margin: 1,
      color: { dark: '#152033', light: '#ffffff' },
    });
    box.appendChild(canvas);
  } else {
    const img = document.createElement('img');
    img.alt = 'Gmail login QR code';
    img.src = `https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(scanURL)}`;
    box.appendChild(img);
  }
}

async function startQRLogin() {
  const statusEl = $('#qr-status');
  const hintEl = $('#qr-hint');
  statusEl.textContent = 'Creating secure session…';
  hintEl.textContent = '';
  $('#qr-modal').classList.remove('hidden');

  try {
    const data = await api('/auth/qr/session', { method: 'POST' });
    qrSessionId = data.session_id;
    $('#qr-open-link').href = data.scan_url;
    await renderQR(data.scan_url);
    statusEl.textContent = 'Waiting for scan…';

    const host = new URL(data.scan_url, window.location.origin).hostname;
    if (host === 'localhost' || host === '127.0.0.1') {
      hintEl.textContent =
        'Localhost QR links only work on this computer. Use “Open link on this device”, or set APP_BASE_URL to your LAN IP for phone scanning.';
    } else {
      hintEl.textContent = 'Keep this window open until Google sign-in finishes on your phone.';
    }

    if (qrPollTimer) clearInterval(qrPollTimer);
    qrPollTimer = setInterval(async () => {
      try {
        const st = await api(`/auth/qr/${qrSessionId}/status`);
        if (st.status === 'approved') {
          clearInterval(qrPollTimer);
          qrPollTimer = null;
          statusEl.textContent = 'Signed in — welcome!';
          saveTokens(st.access_token, st.refresh_token);
          state.user = st.user;
          closeQRModal();
          showMain();
          await Promise.all([loadNotifications(), loadAccounts(), loadSettings()]);
        } else if (st.status === 'expired' || st.status === 'failed' || st.status === 'consumed') {
          clearInterval(qrPollTimer);
          qrPollTimer = null;
          statusEl.textContent = 'Session ended. Generate a new QR code.';
        } else {
          statusEl.textContent = 'Waiting for scan…';
        }
      } catch (e) {
        statusEl.textContent = e.message || 'Waiting…';
      }
    }, 2000);
  } catch (e) {
    statusEl.textContent = e.message || 'Could not start QR login';
  }
}

$('#btn-qr-login').addEventListener('click', startQRLogin);
$('#btn-qr-refresh').addEventListener('click', startQRLogin);
$$('[data-close-qr]').forEach((el) => el.addEventListener('click', closeQRModal));
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeQRModal();
});

$('#login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  try {
    const data = await api('/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        email: fd.get('email'),
        password: fd.get('password'),
      }),
    });
    saveTokens(data.access_token, data.refresh_token);
    state.user = data.user;
    showMain();
    await Promise.all([loadNotifications(), loadAccounts(), loadSettings()]);
  } catch (err) {
    showAuth(err.message);
  }
});

$('#register-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  try {
    const data = await api('/auth/register', {
      method: 'POST',
      body: JSON.stringify({
        email: fd.get('email'),
        password: fd.get('password'),
        display_name: fd.get('display_name'),
      }),
    });
    saveTokens(data.access_token, data.refresh_token);
    state.user = data.user;
    showMain();
    await Promise.all([loadNotifications(), loadAccounts(), loadSettings()]);
  } catch (err) {
    showAuth(err.message);
  }
});

$('#btn-logout').addEventListener('click', async () => {
  try { await api('/auth/logout', { method: 'POST' }); } catch (_) {}
  clearTokens();
  showAuth();
});

$$('.nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    $$('.nav-btn').forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');
    $$('.page').forEach((p) => p.classList.add('hidden'));
    $(`#page-${btn.dataset.page}`).classList.remove('hidden');
  });
});

async function loadNotifications() {
  const list = $('#notif-list');
  list.innerHTML = '<div class="empty">Loading…</div>';
  try {
    const items = await api('/notifications?limit=50') || [];
    if (!items.length) {
      list.innerHTML = '<div class="empty">No notifications yet</div>';
      return;
    }
    list.innerHTML = items.map((n) => `
      <article class="card ${n.is_read ? '' : 'unread'}" data-id="${n.id}">
        <h3>${escapeHtml(n.subject || '(no subject)')}</h3>
        <p>${escapeHtml(n.from_address || '')}</p>
        <p>${escapeHtml(n.snippet || '')}</p>
        <div class="meta">${new Date(n.received_at).toLocaleString()}</div>
      </article>
    `).join('');
    list.querySelectorAll('.card').forEach((card) => {
      card.addEventListener('click', async () => {
        try {
          await api(`/notifications/${card.dataset.id}/read`, { method: 'POST' });
          await loadNotifications();
        } catch (_) {}
      });
    });
  } catch (e) {
    list.innerHTML = `<div class="empty">${escapeHtml(e.message)}</div>`;
  }
}

async function loadAccounts() {
  const list = $('#accounts-list');
  list.innerHTML = '<div class="empty">Loading…</div>';
  try {
    const items = await api('/gmail/accounts') || [];
    if (!items.length) {
      list.innerHTML = '<div class="empty">No Gmail accounts linked yet</div>';
      return;
    }
    list.innerHTML = items.map((a) => `
      <article class="card account-row">
        <div>
          <h3>${escapeHtml(a.email)}</h3>
          <p>${a.notifications_on ? 'Notifications on' : 'Notifications off'}</p>
        </div>
        <div>
          <button type="button" class="ghost" data-toggle="${a.id}" data-on="${a.notifications_on}">
            ${a.notifications_on ? 'Disable' : 'Enable'}
          </button>
          <button type="button" class="ghost" data-unlink="${a.id}">Unlink</button>
        </div>
      </article>
    `).join('');
    list.querySelectorAll('[data-toggle]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const enabled = btn.dataset.on !== 'true';
        await api(`/gmail/accounts/${btn.dataset.toggle}/notifications`, {
          method: 'PATCH',
          body: JSON.stringify({ enabled }),
        });
        await loadAccounts();
      });
    });
    list.querySelectorAll('[data-unlink]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        if (!confirm('Unlink this Gmail account?')) return;
        await api(`/gmail/accounts/${btn.dataset.unlink}`, { method: 'DELETE' });
        await loadAccounts();
      });
    });
  } catch (e) {
    list.innerHTML = `<div class="empty">${escapeHtml(e.message)}</div>`;
  }
}

async function loadSettings() {
  try {
    const s = await api('/settings/notifications');
    const form = $('#settings-form');
    form.enabled.checked = !!s.enabled;
    form.only_primary.checked = !!s.only_primary;
    form.include_spam.checked = !!s.include_spam;
    form.quiet_hours_enabled.checked = !!s.quiet_hours_enabled;
    form.quiet_hours_start.value = s.quiet_hours_start || '22:00';
    form.quiet_hours_end.value = s.quiet_hours_end || '07:00';
    form.keyword_filter.value = s.keyword_filter || '';
    form.sender_allowlist.value = s.sender_allowlist || '';
    applyDark(!!state.user?.dark_mode);
  } catch (_) {}
}

$('#btn-refresh-notif').addEventListener('click', loadNotifications);
$('#btn-refresh-accounts').addEventListener('click', loadAccounts);
$('#btn-mark-all').addEventListener('click', async () => {
  await api('/notifications/read-all', { method: 'POST' });
  await loadNotifications();
});

$('#btn-link-gmail').addEventListener('click', async () => {
  const data = await api('/gmail/accounts/link', { method: 'POST' });
  window.open(data.authorization_url, '_blank', 'noopener');
  alert('Finish Google sign-in in the new tab, then click Refresh.');
});

$('#settings-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const form = e.target;
  const payload = {
    enabled: form.enabled.checked,
    only_primary: form.only_primary.checked,
    include_spam: form.include_spam.checked,
    quiet_hours_enabled: form.quiet_hours_enabled.checked,
    quiet_hours_start: form.quiet_hours_start.value || '22:00',
    quiet_hours_end: form.quiet_hours_end.value || '07:00',
    keyword_filter: form.keyword_filter.value || '',
    sender_allowlist: form.sender_allowlist.value || '',
  };
  await api('/settings/notifications', {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
  const dark = form.dark_mode.checked;
  state.user = await api('/auth/me', {
    method: 'PATCH',
    body: JSON.stringify({ dark_mode: dark }),
  });
  applyDark(dark);
  alert('Settings saved');
});

function escapeHtml(s) {
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

bootstrap();
