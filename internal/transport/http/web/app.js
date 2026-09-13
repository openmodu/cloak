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
// Render untrusted output as text, never as HTML.
function renderResult(text, highlight) {
  const target = el('result-text');
  target.replaceChildren();
  if (!highlight) { target.textContent = text; return; }
  let offset = 0;
  for (const match of text.matchAll(/__PII_[A-Z_]+_\d+__/g)) {
    target.append(document.createTextNode(text.slice(offset, match.index)));
    const mark = document.createElement('mark');
    mark.textContent = match[0];
    target.append(mark);
    offset = match.index + match[0].length;
  }
  target.append(document.createTextNode(text.slice(offset)));
}
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
  el('heading').replaceChildren(
    masking ? 'Share the idea.' : 'Bring the details back.',
    document.createElement('br'),
    masking ? 'Keep the details private.' : 'Right where they belong.',
  );
  el('intro-copy').textContent = masking
    ? 'Mask what matters before the conversation. Bring it back when you are ready.'
    : 'Paste the restoration record from a previous masking result.';
  el('input-label').textContent = masking ? 'Text' : 'Restoration JSON';
  el('source').placeholder = masking ? 'Paste anything you would rather not send as-is…' : 'Paste {"text": "…", "maskMeta": "…"} here';
  el('source').value = input;
  el('sample').hidden = !masking;
  el('language-label').hidden = !masking;
  el('submit').textContent = masking ? 'Mask text' : 'Restore text';
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
    el('result-title').textContent = mode === 'mask' ? 'Masked text' : 'Restored text';
    renderResult(output.text, mode === 'mask');
    el('result-note').textContent = mode === 'mask' ? 'Review before sharing.' : 'Restored. Keep this private.';
    el('credential-details').hidden = mode !== 'mask';
    el('use-restore').hidden = mode !== 'mask';
    if (mode === 'mask') {
      record = { text: output.text, maskMeta: output.maskMeta };
      el('record-text').textContent = JSON.stringify(record, null, 2);
    }
    el('result').hidden = false;
    message(mode === 'mask' && output.text === body.text ? 'Nothing was masked. That does not mean the text is free of sensitive information.' : 'Done.');
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
el('sample').addEventListener('click', () => { clearResult(); message('Example loaded.'); el('source').value = 'Hi Alex,\n\nPlease send the project notes to alex.morgan@example.com. You can reach me at 18744325579 if anything comes up.\n\nThanks!'; count(); el('source').focus(); });
el('source').addEventListener('input', () => { clearResult(); message(''); count(); });
el('language').addEventListener('change', () => { clearResult(); message(''); });
el('source').addEventListener('keydown', event => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && !event.isComposing) { event.preventDefault(); el('workspace-form').requestSubmit(); } });
// No localStorage, cookies, analytics, external fonts, or third-party requests.
fetch('/api/health', { cache: 'no-store', credentials: 'omit' }).then(response => { if (!response.ok) throw new Error(); return response.json(); }).then(data => { if (data.status !== 'ok') throw new Error(); el('connection').textContent = 'Connected'; el('connection').className = 'status online'; }).catch(() => { el('connection').textContent = 'Unavailable'; el('connection').className = 'status offline'; });
