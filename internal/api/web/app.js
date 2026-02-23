const statusContent = document.getElementById('status-content');
const statusPill = document.getElementById('status-pill');
const alertList = document.getElementById('alert-list');
const metricsBody = document.getElementById('metrics-body');
const metricsEmpty = document.getElementById('metrics-empty');
const actionStatus = document.getElementById('action-status');

const clearLogsBtn = document.getElementById('clear-logs');
const clearMetricsBtn = document.getElementById('clear-metrics');
const restartBtn = document.getElementById('restart-engine');

function fmtTime(ts) {
  if (!ts) return '-';
  const date = new Date(ts);
  if (Number.isNaN(date.getTime())) return ts;
  return date.toLocaleString();
}

function setActionMessage(message, isError) {
  if (!actionStatus) return;
  actionStatus.textContent = message || '';
  actionStatus.style.color = isError ? '#c03535' : '';
}

async function fetchJSON(url, options) {
  const response = await fetch(url, options);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  return response.json();
}

function renderStatus(data) {
  if (!statusContent) return;
  const ingest = data.ingest || {};
  const rows = [
    { label: 'Version', value: data.version || '-' },
    { label: 'Config', value: data.config_path || '-' },
    { label: 'API', value: data.api?.addr || '-' },
    { label: 'REST', value: ingest.rest ? 'enabled' : 'disabled' },
    { label: 'Syslog', value: ingest.syslog ? 'enabled' : 'disabled' },
    { label: 'File Tail', value: ingest.file_tail ? 'enabled' : 'disabled' },
    { label: 'TCP Stream', value: ingest.tcp_stream ? 'enabled' : 'disabled' },
    { label: 'Kafka', value: ingest.kafka ? 'enabled' : 'disabled' },
    { label: 'Access Control', value: data.access_control?.enabled ? 'enabled' : 'disabled' },
    { label: 'Whitelist Only', value: data.access_control?.whitelist_only ? 'yes' : 'no' },
    { label: 'Windows', value: (data.detection?.windows || []).join(', ') || '-' },
    { label: 'Updated', value: fmtTime(data.time) },
  ];
  statusContent.innerHTML = rows.map((row) => `
    <div class="stat-row">
      <span>${row.label}</span>
      <span>${row.value}</span>
    </div>
  `).join('');
  if (statusPill) {
    statusPill.textContent = 'Live';
  }
}

function renderMetrics(data) {
  if (!metricsBody) return;
  const metrics = data.metrics || {};
  const rows = [];
  Object.keys(metrics).forEach((reader) => {
    const list = metrics[reader] || [];
    list.forEach((m) => rows.push({ reader, ...m }));
  });
  rows.sort((a, b) => {
    if (a.reader === b.reader) {
      return a.window_sec - b.window_sec;
    }
    return a.reader.localeCompare(b.reader);
  });
  metricsBody.innerHTML = rows.map((row) => `
    <tr>
      <td>${row.reader}</td>
      <td>${row.window_sec}s</td>
      <td>${row.aps.toFixed(2)}</td>
      <td>${row.fr.toFixed(2)}</td>
      <td>${row.uds.toFixed(2)}</td>
      <td>${row.tv.toFixed(4)}</td>
      <td>${row.attempts}</td>
      <td>${row.failures}</td>
    </tr>
  `).join('');
  if (metricsEmpty) {
    metricsEmpty.style.display = rows.length ? 'none' : 'block';
  }
}

function severityClass(severity) {
  if (!severity) return '';
  const lower = severity.toLowerCase();
  if (lower === 'critical') return 'severity-critical';
  if (lower === 'high') return 'severity-high';
  return '';
}

function renderAlerts(data) {
  if (!alertList) return;
  const alerts = (data.alerts || []).slice();
  alerts.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
  if (!alerts.length) {
    alertList.innerHTML = '<div class="muted">No alerts yet.</div>';
    return;
  }
  alertList.innerHTML = alerts.slice(0, 50).map((alert) => {
    const metrics = alert.metrics || {};
    return `
      <div class="alert-item ${severityClass(alert.severity)}">
        <strong>${alert.alert_type}</strong>
        <span>${alert.reader_id}</span>
        <div class="meta">${fmtTime(alert.timestamp)} · APS ${metrics.aps?.toFixed ? metrics.aps.toFixed(2) : '-'} · FR ${metrics.fr?.toFixed ? metrics.fr.toFixed(2) : '-'}</div>
      </div>
    `;
  }).join('');
}

async function refreshStatus() {
  try {
    const data = await fetchJSON('/status');
    renderStatus(data);
  } catch (err) {
    if (statusPill) {
      statusPill.textContent = 'Offline';
    }
  }
}

async function refreshMetrics() {
  try {
    const data = await fetchJSON('/metrics');
    renderMetrics(data);
  } catch (err) {
    if (metricsEmpty) {
      metricsEmpty.textContent = 'Metrics unavailable.';
      metricsEmpty.style.display = 'block';
    }
  }
}

async function refreshAlerts() {
  try {
    const data = await fetchJSON('/alerts?limit=200');
    renderAlerts(data);
  } catch (err) {
    if (alertList) {
      alertList.innerHTML = '<div class="muted">Alerts unavailable.</div>';
    }
  }
}

if (clearLogsBtn) {
  clearLogsBtn.addEventListener('click', async () => {
    setActionMessage('Clearing alerts...');
    try {
      await fetchJSON('/admin/clear', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: 'alerts' }),
      });
      setActionMessage('Alerts cleared.');
      refreshAlerts();
    } catch (err) {
      setActionMessage('Failed to clear alerts.', true);
    }
  });
}

if (clearMetricsBtn) {
  clearMetricsBtn.addEventListener('click', async () => {
    setActionMessage('Clearing metrics...');
    try {
      await fetchJSON('/admin/clear', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: 'metrics' }),
      });
      setActionMessage('Metrics cleared.');
      refreshMetrics();
    } catch (err) {
      setActionMessage('Failed to clear metrics.', true);
    }
  });
}

if (restartBtn) {
  restartBtn.addEventListener('click', async () => {
    setActionMessage('Restarting engine...');
    try {
      await fetchJSON('/admin/restart', { method: 'POST' });
      setActionMessage('Engine restarted.');
      refreshMetrics();
      refreshAlerts();
    } catch (err) {
      setActionMessage('Failed to restart engine.', true);
    }
  });
}

refreshStatus();
refreshMetrics();
refreshAlerts();

setInterval(refreshStatus, 5000);
setInterval(refreshMetrics, 2500);
setInterval(refreshAlerts, 2000);
