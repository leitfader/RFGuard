const enabledEl = document.getElementById('ac-enabled');
const whitelistOnlyEl = document.getElementById('ac-whitelist-only');
const whitelistEl = document.getElementById('ac-whitelist');
const blacklistEl = document.getElementById('ac-blacklist');
const readerWhitelistEl = document.getElementById('ac-reader-whitelist');
const readerBlacklistEl = document.getElementById('ac-reader-blacklist');
const saveBtn = document.getElementById('save-access');
const saveStatus = document.getElementById('save-status');

let isLoading = false;

function setStatus(message, isError) {
  if (!saveStatus) return;
  saveStatus.textContent = message || '';
  saveStatus.style.color = isError ? '#c03535' : '';
}

async function fetchJSON(url, options) {
  const response = await fetch(url, options);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  return response.json();
}

function debounce(fn, delay) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), delay);
  };
}

function toLines(value) {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
}

function toReaderMap(value) {
  const map = {};
  value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .forEach((line) => {
      const [reader, list] = line.split(':');
      if (!reader || !list) return;
      const ids = list.split(',').map((v) => v.trim()).filter(Boolean);
      if (ids.length) {
        map[reader.trim()] = ids;
      }
    });
  return map;
}

function mapToLines(map) {
  if (!map) return '';
  return Object.keys(map)
    .map((reader) => `${reader}: ${map[reader].join(', ')}`)
    .join('\n');
}

async function loadAccessControl() {
  try {
    isLoading = true;
    const data = await fetchJSON('/config/access_control');
    const ac = data.access_control || {};
    if (enabledEl) enabledEl.checked = !!ac.enabled;
    if (whitelistOnlyEl) whitelistOnlyEl.checked = !!ac.whitelist_only;
    if (whitelistEl) whitelistEl.value = (ac.whitelist || []).join('\n');
    if (blacklistEl) blacklistEl.value = (ac.blacklist || []).join('\n');
    if (readerWhitelistEl) readerWhitelistEl.value = mapToLines(ac.reader_whitelists);
    if (readerBlacklistEl) readerBlacklistEl.value = mapToLines(ac.reader_blacklists);
    setStatus('');
  } catch (err) {
    setStatus('Failed to load access control.', true);
  } finally {
    isLoading = false;
  }
}

async function saveAccessControl() {
  const payload = {
    enabled: enabledEl?.checked || false,
    whitelist_only: whitelistOnlyEl?.checked || false,
    whitelist: whitelistEl ? toLines(whitelistEl.value) : [],
    blacklist: blacklistEl ? toLines(blacklistEl.value) : [],
    reader_whitelists: readerWhitelistEl ? toReaderMap(readerWhitelistEl.value) : {},
    reader_blacklists: readerBlacklistEl ? toReaderMap(readerBlacklistEl.value) : {},
  };
  setStatus('Saving...');
  try {
    await fetchJSON('/config/access_control', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    setStatus('Saved.');
  } catch (err) {
    setStatus('Failed to save access control.', true);
  }
}

if (saveBtn) {
  saveBtn.addEventListener('click', saveAccessControl);
}

const debouncedSave = debounce(() => {
  if (!isLoading) {
    saveAccessControl();
  }
}, 700);

if (enabledEl) {
  enabledEl.addEventListener('change', debouncedSave);
}

if (whitelistOnlyEl) {
  whitelistOnlyEl.addEventListener('change', debouncedSave);
}

if (whitelistEl) {
  whitelistEl.addEventListener('input', debouncedSave);
}

if (blacklistEl) {
  blacklistEl.addEventListener('input', debouncedSave);
}

if (readerWhitelistEl) {
  readerWhitelistEl.addEventListener('input', debouncedSave);
}

if (readerBlacklistEl) {
  readerBlacklistEl.addEventListener('input', debouncedSave);
}

loadAccessControl();
