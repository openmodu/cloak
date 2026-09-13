'use strict';
const el = id => document.getElementById(id);
let mode = 'mask';
let record = null;
let busy = false;

function message(text, error = false) {
  el('message').textContent = text;
  el('message').classList.toggle('error', error);
}
function clearResult() {
  record = null;
  el('result').hidden = true;
  el('result-text').textContent = '';
  el('record-text').textContent = '';
  el('credential-details').open = false;
}
function count() { el('char-count').textContent = `${Array.from(el('source').value).length.toLocaleString()} characters`; }
function setMode(next, input = '') {
  if (busy) return;
  mode = next;
  clearResult();
  message('');
  const masking = mode === 'mask';
  el('mask-mode').classList.toggle('selected', masking);
  el('restore-mode').classList.toggle('selected', !masking);
  el('mask-mode').setAttribute('aria-pressed', String(masking));
  el('restore-mode').setAttribute('aria-pressed', String(!masking));
  el('current-mode').textContent = masking ? 'Mask text' : 'Restore text';
  el('heading').replaceChildren();
  el('heading').append(masking ? 'Share the idea.' : 'Bring the details back.', document.createElement('br'), masking ? 'Keep the details private.' : 'Right where they belong.');
  el('intro-copy').textContent = masking ? 'Mask sensitive details before your next conversation. Bring them back when you’re ready.' : 'Paste a restoration record to recover the original details.';
  el('input-label').textContent = masking ? 'What would you like to protect?' : 'Your restoration JSON';
  el('source').placeholder = masking ? 'Paste an email, a note, or anything you’d rather keep private…' : 'Paste {"text": "…", "maskMeta": "…"} here';
  el('source').value = input;
  el('sample').hidden = !masking;
  el('language-label').hidden = !masking;
  el('submit').textContent = masking ? 'Mask text ↑' : 'Restore text ↶';
  count();
}
async function request(path, body) {
  const headers = { 'Content-Type': 'application/json' };
  const token = el('access-key').value.trim();
  if (token) headers.Authorization = `Bearer ${token}`;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 30000);
  try {
    const response = await fetch(path, { method: 'POST', headers, body: JSON.stringify(body), signal: controller.signal, cache: 'no-store', credentials: 'omit' });
    if (response.status === 401) {
      document.querySelector('.settings').open = true;
      throw new Error('Access denied. Enter your server access token below and try again.');
    }
    const data = await response.json();
    if (!response.ok || data.error) throw new Error(data.error?.message || `Request failed (${response.status}).`);
    if (!data.output || typeof data.output.text !== 'string') throw new Error('The server returned an unexpected response.');
    return data.output;
  } finally { clearTimeout(timeout); }
}
el('workspace-form').addEventListener('submit', async event => {
  event.preventDefault();
  if (busy || !el('source').value.trim()) return;
  let body = { text: el('source').value, language: el('language').value };
  clearResult();
  if (mode === 'restore') {
    try {
      body = JSON.parse(el('source').value);
      if (!body || typeof body.text !== 'string' || typeof body.maskMeta !== 'string' || !body.maskMeta) throw new Error();
      body = { text: body.text, maskMeta: body.maskMeta };
    } catch { message('Enter a JSON object with text and maskMeta from a previous masking result.', true); return; }
  }
  busy = true;
  for (const id of ['submit', 'mask-mode', 'restore-mode', 'new-session', 'sample', 'language']) el(id).disabled = true;
  el('source').readOnly = true;
  el('workspace-form').setAttribute('aria-busy', 'true');
  message(mode === 'mask' ? 'Masking sensitive details…' : 'Restoring original details…');
  try {
    const output = await request(mode === 'mask' ? '/api/mask_text' : '/api/restore_text', body);
    if (mode === 'mask' && typeof output.maskMeta !== 'string') throw new Error('The server did not return a restoration record.');
    el('result-title').textContent = mode === 'mask' ? 'Your masked text' : 'Your restored text';
    el('result-text').textContent = output.text;
    el('result-note').textContent = mode === 'mask' ? 'Review the result before sharing.' : 'Original details restored. Keep this text private.';
    el('credential-details').hidden = mode !== 'mask';
    el('use-restore').hidden = mode !== 'mask';
    if (mode === 'mask') {
      record = { text: output.text, maskMeta: output.maskMeta };
      el('record-text').textContent = JSON.stringify(record, null, 2);
    }
    el('result').hidden = false;
    message(mode === 'mask' && output.text === body.text ? 'No changes detected. This does not guarantee the text is free of sensitive information.' : 'Done. Your result is ready below.');
  } catch (error) {
    message(error.name === 'AbortError' ? 'The request timed out. Please try again.' : error instanceof TypeError ? 'Could not reach the server. Check your connection and try again.' : error.message, true);
  } finally {
    busy = false;
    for (const id of ['submit', 'mask-mode', 'restore-mode', 'new-session', 'sample', 'language']) el(id).disabled = false;
    el('source').readOnly = false;
    el('workspace-form').setAttribute('aria-busy', 'false');
  }
});
async function copy(text) {
  try { await navigator.clipboard.writeText(text); message('Copied to clipboard.'); }
  catch { message('Clipboard is unavailable. Select the result and copy it manually.', true); }
}
el('copy-text').addEventListener('click', () => copy(el('result-text').textContent));
el('copy-record').addEventListener('click', () => { if (record) copy(JSON.stringify(record, null, 2)); });
el('use-restore').addEventListener('click', () => { if (record) { setMode('restore', JSON.stringify(record, null, 2)); el('source').focus(); } });
el('mask-mode').addEventListener('click', () => { if (mode !== 'mask') setMode('mask'); });
el('restore-mode').addEventListener('click', () => { if (mode !== 'restore') setMode('restore'); });
el('new-session').addEventListener('click', () => { setMode('mask'); el('access-key').value = ''; el('language').value = 'auto'; el('source').focus(); });
el('sample').addEventListener('click', () => { clearResult(); message('Example loaded. Choose Mask text to try it.'); el('source').value = 'Hi Alex,\n\nPlease send the project notes to alex.morgan@example.com. You can reach me at 18744325579 if anything comes up.\n\nThanks!'; count(); el('source').focus(); });
el('source').addEventListener('input', () => { clearResult(); message(''); count(); });
el('language').addEventListener('change', () => { clearResult(); message(''); });
el('source').addEventListener('keydown', event => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && !event.isComposing) { event.preventDefault(); el('workspace-form').requestSubmit(); } });
// No localStorage, cookies, analytics, external fonts, or third-party requests.
fetch('/api/health', { cache: 'no-store', credentials: 'omit' }).then(response => { if (!response.ok) throw new Error(); return response.json(); }).then(data => { if (data.status !== 'ok') throw new Error(); el('connection').textContent = 'Service connected'; el('connection').className = 'connection online'; }).catch(() => { el('connection').textContent = 'Service unavailable'; el('connection').className = 'connection offline'; });
