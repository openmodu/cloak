const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

// Small DOM stand-in for state and request tests; not a visual browser test.
function setup(response) {
  function node() {
    return {
      value: '', hidden: false, textContent: '', children: [], listeners: {},
      classList: { toggle() {} }, setAttribute() {}, focus() {},
      addEventListener(name, fn) { this.listeners[name] = fn; },
      append(...nodes) { this.children.push(...nodes); },
      replaceChildren(...nodes) { this.children = nodes; this.textContent = ''; },
    };
  }
  const nodes = new Map();
  const get = id => { if (!nodes.has(id)) nodes.set(id, node()); return nodes.get(id); };
  get('language').value = 'auto';
  const requests = [];
  const context = vm.createContext({
    document: { getElementById: get, createElement: node, createTextNode: text => ({ textContent: text }), querySelector: () => get('settings') },
    AbortController, setTimeout, clearTimeout,
    navigator: { clipboard: { writeText: async () => {} } },
    fetch: async (url, options) => {
      if (url === '/api/health') return { ok: true, json: async () => ({ status: 'ok' }) };
      requests.push({ url, ...options });
      return response;
    },
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, '../internal/transport/http/web/app.js'), 'utf8'), context);
  return { get, requests };
}

test('LLM tab sends a traced call only after submission and clears results on switch', async () => {
  const { get, requests } = setup({ ok: true, status: 200, json: async () => ({ output: {
    text: 'Contact demo@example.com', maskedText: 'Contact __PII_EMAIL_ADDRESS_1__', llmReply: 'Contact __PII_EMAIL_ADDRESS_1__',
    timings: { maskMs: 12.345, llmMs: 1500, restoreMs: 0.002, totalMs: 1512.5 },
  } }) });
  get('llm-mode').listeners.click();
  assert.equal(requests.length, 0);
  assert.equal(get('llm-settings').hidden, false);
  assert.equal(get('submit').textContent, 'Run LLM test');
  get('source').value = 'Contact demo@example.com';
  get('llm-model').value = 'test-model';
  get('access-key').value = 'test-token';
  await get('workspace-form').listeners.submit({ preventDefault() {} });
  assert.equal(requests.length, 1);
  assert.equal(requests[0].url, '/api/call');
  assert.deepEqual(JSON.parse(requests[0].body), { text: 'Contact demo@example.com', language: 'auto', model: 'test-model', trace: true });
  assert.equal(requests[0].headers.Authorization, 'Bearer test-token');
  assert.equal(get('llm-trace').hidden, false);
  assert.equal(get('result-text').textContent, 'Contact demo@example.com');
  assert.equal(get('credential-details').hidden, true);
  assert.equal(get('submit').disabled, false);
  assert.equal(get('mask-timing').textContent, '12.35 ms');
  assert.equal(get('llm-timing').textContent, '1.50 s');
  assert.equal(get('restore-timing').textContent, '<0.01 ms');
  assert.equal(get('restore-timing').hidden, false);
  get('restore-mode').listeners.click();
  assert.equal(get('llm-trace').hidden, true);
  assert.equal(get('masked-prompt').textContent, '');
  assert.equal(get('llm-settings').hidden, true);
  assert.equal(get('restore-timing').hidden, true);
  assert.equal(get('total-timing').textContent, '');
});

test('Unconfigured LLM leaves no stale result and allows retry', async () => {
  const { get } = setup({ status: 503, ok: false });
  get('llm-mode').listeners.click();
  get('source').value = 'test';
  await get('workspace-form').listeners.submit({ preventDefault() {} });
  assert.match(get('message').textContent, /No LLM is configured/);
  assert.equal(get('result').hidden, true);
  assert.equal(get('llm-trace').hidden, true);
  assert.equal(get('submit').disabled, false);
});
