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
  el('llm-trace').hidden = true;
  el('masked-prompt').textContent = '';
  el('llm-reply').textContent = '';
  el('restore-timing').hidden = true;
  for (const id of ['mask-timing', 'llm-timing', 'restore-timing', 'total-timing']) el(id).textContent = '';
}
function formatDuration(ms) {
  if (typeof ms !== 'number' || !Number.isFinite(ms) || ms < 0) return 'Unavailable';
  if (ms < 0.01) return '<0.01 ms';
  return ms < 1000 ? `${ms.toFixed(2)} ms` : `${(ms / 1000).toFixed(2)} s`;
}
function count() { el('char-count').textContent = `${Array.from(el('source').value).length.toLocaleString()} characters`; }
// Render untrusted output as text, never as HTML.
function renderResult(text, highlight, target = el('result-text')) {
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
  const calling = mode === 'llm';
  el('mask-mode').classList.toggle('selected', masking);
  el('restore-mode').classList.toggle('selected', mode === 'restore');
  el('llm-mode').classList.toggle('selected', calling);
  el('mask-mode').setAttribute('aria-pressed', String(masking));
  el('restore-mode').setAttribute('aria-pressed', String(mode === 'restore'));
  el('llm-mode').setAttribute('aria-pressed', String(calling));
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
  el('sample').hidden = mode === 'restore';
  el('language-label').hidden = mode === 'restore';
  el('llm-settings').hidden = !calling;
  el('submit').textContent = masking ? 'Mask text' : 'Restore text';
  if (calling) {
    el('current-mode').textContent = 'LLM test';
    el('heading').replaceChildren('Test the whole flow.', document.createElement('br'), 'Keep the details private.');
    el('intro-copy').textContent = 'Mask → LLM → restore. Compare what the model sees with what you get back.';
    el('input-label').textContent = 'Prompt';
    el('source').placeholder = 'Write a task for the model, with fictional details to mask…';
    el('submit').textContent = 'Run LLM test';
  }
  count();
}
async function request(path, body, timeoutMs = 30000) {
  const headers = { 'Content-Type': 'application/json' };
  const token = el('access-key').value.trim();
  if (token) headers.Authorization = `Bearer ${token}`;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(path, { method: 'POST', headers, body: JSON.stringify(body), signal: controller.signal, cache: 'no-store', credentials: 'omit' });
    if (response.status === 401) {
      document.querySelector('.settings').open = true;
      throw new Error('Access denied. Enter your server access token below and try again.');
    }
    if (response.status === 503 && path === '/api/call') throw new Error('No LLM is configured on this server. Configure an LLM connection and restart Cloak.');
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
  if (mode === 'llm') body = { ...body, model: el('llm-model').value.trim(), trace: true };
  clearResult();
  if (mode === 'restore') {
    try {
      body = JSON.parse(el('source').value);
      if (!body || typeof body.text !== 'string' || typeof body.maskMeta !== 'string' || !body.maskMeta) throw new Error();
      body = { text: body.text, maskMeta: body.maskMeta };
    } catch { message('Enter a JSON object with text and maskMeta from a previous masking result.', true); return; }
  }
  busy = true;
  for (const id of ['submit', 'mask-mode', 'restore-mode', 'llm-mode', 'llm-model', 'new-session', 'sample', 'language']) el(id).disabled = true;
  el('source').readOnly = true;
  el('workspace-form').setAttribute('aria-busy', 'true');
  message(mode === 'llm' ? 'Running mask → LLM → restore. Waiting for the model; this may take up to two minutes…' : mode === 'mask' ? 'Masking sensitive details…' : 'Restoring original details…');
  try {
    const output = await request(mode === 'llm' ? '/api/call' : mode === 'mask' ? '/api/mask_text' : '/api/restore_text', body, mode === 'llm' ? 150000 : 30000);
    if (mode === 'llm') {
      if (typeof output.maskedText !== 'string' || typeof output.llmReply !== 'string') throw new Error('This server does not support LLM trace results. Update the server and try again.');
      renderResult(output.maskedText, true, el('masked-prompt'));
      renderResult(output.llmReply, true, el('llm-reply'));
      el('llm-trace').hidden = false;
      const timings = output.timings || {};
      el('mask-timing').textContent = formatDuration(timings.maskMs);
      el('llm-timing').textContent = formatDuration(timings.llmMs);
      el('restore-timing').textContent = formatDuration(timings.restoreMs);
      el('restore-timing').hidden = false;
      el('total-timing').textContent = `Server total: ${formatDuration(timings.totalMs)} · excludes browser transfer time`;
    }
    if (mode === 'mask' && typeof output.maskMeta !== 'string') throw new Error('The server did not return a restoration record.');
    el('result-title').textContent = mode === 'mask' ? 'Masked text' : 'Restored text';
    if (mode === 'llm') el('result-title').textContent = '3. Restored response';
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
    if (mode === 'llm') message(output.maskedText === body.text ? 'Completed, but nothing was masked. Review the prompt and detection settings.' : 'Completed. Compare all three stages below. Models may change or omit placeholders; review the restored response.');
  } catch (error) {
    message(error.name === 'AbortError' ? 'The request timed out. Please try again.' : error instanceof TypeError ? 'Could not reach the server. Check your connection and try again.' : error.message, true);
  } finally {
    busy = false;
    for (const id of ['submit', 'mask-mode', 'restore-mode', 'llm-mode', 'llm-model', 'new-session', 'sample', 'language']) el(id).disabled = false;
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
el('llm-mode').addEventListener('click', () => { if (mode !== 'llm') setMode('llm'); });
el('llm-model').addEventListener('input', () => { clearResult(); message(''); });
el('new-session').addEventListener('click', () => { setMode('mask'); el('access-key').value = ''; el('llm-model').value = ''; el('language').value = 'auto'; el('source').focus(); });
el('sample').addEventListener('click', () => { clearResult(); message('Example loaded.'); el('source').value = mode === 'llm' ? 'Write a short delivery confirmation using the following details. Preserve every placeholder token exactly once; do not change its spelling.\n\nContact: demo@example.com\nAddress: 北京市朝阳区建国路88号' : 'Hi Alex,\n\nPlease send the project notes to alex.morgan@example.com. You can reach me at 18744325579 if anything comes up.\n\nThanks!'; count(); el('source').focus(); });
el('source').addEventListener('input', () => { clearResult(); message(''); count(); });
el('language').addEventListener('change', () => { clearResult(); message(''); });
el('source').addEventListener('keydown', event => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && !event.isComposing) { event.preventDefault(); el('workspace-form').requestSubmit(); } });
// No localStorage, cookies, analytics, external fonts, or third-party requests.
fetch('/api/health', { cache: 'no-store', credentials: 'omit' }).then(response => { if (!response.ok) throw new Error(); return response.json(); }).then(data => { if (data.status !== 'ok') throw new Error(); el('connection').textContent = 'Connected'; el('connection').className = 'status online'; }).catch(() => { el('connection').textContent = 'Unavailable'; el('connection').className = 'status offline'; });
