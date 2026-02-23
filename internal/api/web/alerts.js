const alertsBody = document.getElementById('alerts-body');
const alertCount = document.getElementById('alert-count');
const alertStatus = document.getElementById('alert-status');
const clearAlertsBtn = document.getElementById('clear-alerts');

function esc(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function fmtTime(ts) {
  if (!ts) return '-';
  const date = new Date(ts);
  if (Number.isNaN(date.getTime())) return ts;
  return date.toLocaleString();
}

async function fetchJSON(url, options) {
  const response = await fetch(url, options);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  return response.json();
}

function contextToString(context) {
  if (!context) return '-';
  return Object.keys(context)
    .sort((a, b) => a.localeCompare(b))
    .map((key) => `${key}=${context[key]}`)
    .join(' ');
}

function renderAlerts(data) {
  if (!alertsBody) return;
  const alerts = (data.alerts || []).slice();
  alerts.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
  if (alertCount) {
    alertCount.textContent = `${alerts.length} alerts`;
  }
  if (!alerts.length) {
    alertsBody.innerHTML = '<tr><td colspan="12" class="muted">No alerts yet.</td></tr>';
    return;
  }
  alertsBody.innerHTML = alerts.map((alert) => {
    const metrics = alert.metrics || {};
    return `
      <tr>
        <td>${esc(fmtTime(alert.timestamp))}</td>
        <td>${esc(alert.reader_id)}</td>
        <td>${esc(alert.severity)}</td>
        <td>${esc(alert.alert_type)}</td>
        <td>${esc(alert.window_sec)}</td>
        <td>${esc(alert.score?.toFixed ? alert.score.toFixed(1) : alert.score)}</td>
        <td>${esc((alert.rules || []).join(', '))}</td>
        <td>${esc(metrics.aps?.toFixed ? metrics.aps.toFixed(2) : metrics.aps)}</td>
        <td>${esc(metrics.fr?.toFixed ? metrics.fr.toFixed(2) : metrics.fr)}</td>
        <td>${esc(metrics.uds?.toFixed ? metrics.uds.toFixed(2) : metrics.uds)}</td>
        <td>${esc(metrics.tv?.toFixed ? metrics.tv.toFixed(4) : metrics.tv)}</td>
        <td>${esc(contextToString(alert.context))}</td>
      </tr>
    `;
  }).join('');
}

async function refreshAlerts() {
  try {
    const data = await fetchJSON('/alerts?limit=1000');
    renderAlerts(data);
  } catch (err) {
    if (alertsBody) {
      alertsBody.innerHTML = '<tr><td colspan="12" class="muted">Alerts unavailable.</td></tr>';
    }
  }
}

if (clearAlertsBtn) {
  clearAlertsBtn.addEventListener('click', async () => {
    if (alertStatus) alertStatus.textContent = 'Clearing alerts...';
    try {
      await fetchJSON('/admin/clear', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: 'alerts' }),
      });
      if (alertStatus) alertStatus.textContent = 'Alerts cleared.';
      refreshAlerts();
    } catch (err) {
      if (alertStatus) alertStatus.textContent = 'Failed to clear alerts.';
    }
  });
}

refreshAlerts();
setInterval(refreshAlerts, 2000);
