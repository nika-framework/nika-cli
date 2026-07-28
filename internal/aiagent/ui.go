package aiagent

// chatPage is the whole browser UI: one self-contained HTML document with no
// external assets, so `nika agent start` works offline and behind a firewall.
const chatPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Nika Agent</title>
<style>
  :root {
    color-scheme: light dark;
    --bg: #0d0f14;
    --panel: #14171f;
    --panel-2: #1b1f29;
    --panel-3: #232833;
    --border: #262c38;
    --text: #e7eaf0;
    --muted: #8a93a6;
    --faint: #5d6577;
    --accent: #6ea8fe;
    --accent-soft: #172947;
    --ok: #4ade80;
    --warn: #fbbf24;
    --err: #f87171;
    --radius: 10px;
    --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
    --sidebar: 264px;
  }
  @media (prefers-color-scheme: light) {
    :root {
      --bg: #f7f8fa; --panel: #ffffff; --panel-2: #f1f3f7; --panel-3: #e8ebf1;
      --border: #dde1e9; --text: #12151c; --muted: #626b7d; --faint: #8b93a4;
      --accent: #2563eb; --accent-soft: #e4edff;
    }
  }
  * { box-sizing: border-box; }
  html, body { height: 100%; }
  body {
    margin: 0; background: var(--bg); color: var(--text);
    font: 14.5px/1.6 system-ui, -apple-system, "Segoe UI", Roboto, "Noto Sans", "Vazirmatn", sans-serif;
    display: flex; overflow: hidden;
  }
  button, input, textarea, select { font: inherit; color: inherit; }
  button { cursor: pointer; }

  /* ── Sidebar ─────────────────────────────────────────── */
  aside {
    width: var(--sidebar); flex: 0 0 var(--sidebar); height: 100vh;
    background: var(--panel); border-right: 1px solid var(--border);
    display: flex; flex-direction: column;
  }
  .brand {
    padding: 14px 16px; font-weight: 700; letter-spacing: .2px;
    border-bottom: 1px solid var(--border); display: flex; align-items: center; gap: 8px;
  }
  .brand span { color: var(--accent); }
  .brand .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--ok); margin-left: auto; }
  .brand .dot.busy { background: var(--warn); animation: pulse 1s infinite; }
  @keyframes pulse { 0%,100% { opacity:.3 } 50% { opacity:1 } }

  .newchat { margin: 12px; padding: 9px 12px; border-radius: 8px; border: 1px solid var(--border);
             background: var(--panel-2); display: flex; align-items: center; gap: 8px; }
  .newchat:hover { border-color: var(--accent); color: var(--accent); }

  .chats { flex: 1; overflow-y: auto; padding: 0 8px 12px; }
  .chats h4 { margin: 8px 8px 6px; font-size: 11px; text-transform: uppercase;
              letter-spacing: .8px; color: var(--faint); font-weight: 600; }
  .chat-item {
    display: flex; align-items: center; gap: 8px; padding: 8px 10px; border-radius: 8px;
    cursor: pointer; color: var(--muted); font-size: 13.5px;
  }
  .chat-item:hover { background: var(--panel-2); color: var(--text); }
  .chat-item.active { background: var(--accent-soft); color: var(--text); }
  .chat-item .label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .chat-item .del { opacity: 0; border: none; background: none; color: var(--faint); padding: 0 2px; font-size: 15px; line-height: 1; }
  .chat-item:hover .del { opacity: 1; }
  .chat-item .del:hover { color: var(--err); }

  .sidebar-foot { border-top: 1px solid var(--border); padding: 10px 14px; font-size: 11.5px; color: var(--faint); }
  .sidebar-foot div { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin: 2px 0; }
  .sidebar-foot code { font-family: var(--mono); font-size: 11px; }

  /* ── Main ────────────────────────────────────────────── */
  main { flex: 1; min-width: 0; height: 100vh; display: flex; flex-direction: column; }
  .tabs { display: flex; gap: 2px; padding: 10px 16px 0; border-bottom: 1px solid var(--border); background: var(--panel); }
  .tab {
    padding: 9px 16px; border: 1px solid transparent; border-bottom: none; background: none;
    color: var(--muted); border-radius: 8px 8px 0 0; font-size: 13.5px; display: flex; align-items: center; gap: 7px;
    position: relative; top: 1px;
  }
  .tab:hover { color: var(--text); }
  .tab.active { color: var(--text); background: var(--bg); border-color: var(--border); }
  .tab .count { background: var(--panel-3); border-radius: 999px; padding: 0 6px; font-size: 11px; color: var(--muted); }
  .tabs .spacer { flex: 1; }
  .tabs .badge { align-self: center; font-size: 11.5px; padding: 3px 9px; border-radius: 999px;
                 background: var(--panel-2); border: 1px solid var(--border); color: var(--muted); margin-bottom: 8px; }
  .tabs .badge.ro { border-color: var(--warn); color: var(--warn); }

  .pane { flex: 1; overflow-y: auto; display: none; }
  .pane.active { display: block; }

  /* ── Chat feed ───────────────────────────────────────── */
  .feed { max-width: 860px; margin: 0 auto; padding: 22px 20px 10px; display: flex; flex-direction: column; gap: 13px; }
  .msg { display: flex; gap: 11px; }
  .msg .who { flex: 0 0 28px; height: 28px; border-radius: 7px; display: grid; place-items: center;
              font-size: 13px; background: var(--panel-2); border: 1px solid var(--border); }
  .msg.user .who { background: var(--accent-soft); border-color: var(--accent); }
  .bubble { flex: 1; min-width: 0; background: var(--panel); border: 1px solid var(--border);
            border-radius: var(--radius); padding: 11px 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .msg.user .bubble { background: var(--accent-soft); border-color: transparent; }

  .steps { display: flex; flex-direction: column; gap: 5px; margin-left: 39px; }
  details.tool { background: var(--panel); border: 1px solid var(--border); border-radius: 9px; font-size: 13px; overflow: hidden; }
  details.tool > summary { cursor: pointer; padding: 7px 11px; display: flex; align-items: center; gap: 8px;
                           list-style: none; color: var(--muted); }
  details.tool > summary::-webkit-details-marker { display: none; }
  details.tool .name { font-family: var(--mono); font-size: 12.5px; color: var(--text);
                       overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  details.tool.failed { border-color: var(--err); }
  details.tool.failed .name { color: var(--err); }
  details.tool.running .dot { animation: pulse 1s infinite; }
  .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--ok); flex: 0 0 auto; }
  details.tool.failed .dot { background: var(--err); }
  details.tool pre, pre.out {
    margin: 0; padding: 9px 11px; background: var(--panel-2); border-top: 1px solid var(--border);
    font-family: var(--mono); font-size: 12px; white-space: pre-wrap; overflow-wrap: anywhere;
    max-height: 320px; overflow: auto;
  }
  .thinking { color: var(--muted); font-style: italic; margin-left: 39px; font-size: 13.5px; }
  .changed { margin-left: 39px; font-size: 13px; color: var(--muted);
             border-left: 2px solid var(--ok); padding-left: 10px; }
  .changed b { color: var(--ok); font-weight: 600; }
  .error { margin-left: 39px; color: var(--err); background: color-mix(in srgb, var(--err) 10%, transparent);
           border: 1px solid var(--err); border-radius: 9px; padding: 10px 12px; font-size: 13.5px; white-space: pre-wrap; }
  .empty { color: var(--muted); text-align: center; padding: 56px 20px; }
  .empty h3 { color: var(--text); font-weight: 600; margin: 0 0 8px; }

  /* ── Composer ────────────────────────────────────────── */
  footer { border-top: 1px solid var(--border); background: var(--panel); padding: 10px 20px 14px; }
  .suggest-row {
    max-width: 860px; margin: 0 auto 9px; display: flex; gap: 7px;
    overflow-x: auto; overflow-y: hidden; flex-wrap: nowrap;
    scrollbar-width: thin; padding-bottom: 3px;
  }
  .suggest-row::-webkit-scrollbar { height: 5px; }
  .suggest-row::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }
  .suggest-row button {
    flex: 0 0 auto; white-space: nowrap; font-size: 12.5px; padding: 5px 11px;
    border-radius: 999px; border: 1px solid var(--border); background: var(--panel-2); color: var(--muted);
  }
  .suggest-row button:hover { border-color: var(--accent); color: var(--accent); }
  .composer { max-width: 860px; margin: 0 auto; display: flex; gap: 9px; align-items: flex-end; }
  textarea {
    flex: 1; resize: none; background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 10px; padding: 10px 12px; min-height: 44px; max-height: 200px;
  }
  textarea:focus { outline: none; border-color: var(--accent); }
  .send { background: var(--accent); border: 1px solid var(--accent); color: #fff; border-radius: 10px; padding: 10px 18px; }
  .send:disabled { opacity: .45; cursor: not-allowed; }
  .hint { max-width: 860px; margin: 7px auto 0; color: var(--faint); font-size: 11.5px; }

  /* ── Commands ────────────────────────────────────────── */
  .cmd-wrap { max-width: 900px; margin: 0 auto; padding: 20px; }
  .cmd-group { margin-bottom: 26px; }
  .cmd-group h3 { margin: 0 0 10px; font-size: 11.5px; text-transform: uppercase;
                  letter-spacing: .9px; color: var(--faint); font-weight: 600; }
  .cmd-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(255px, 1fr)); gap: 10px; }
  .cmd-card {
    background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius);
    padding: 13px; cursor: pointer; text-align: left; display: flex; flex-direction: column; gap: 5px;
  }
  .cmd-card:hover { border-color: var(--accent); }
  .cmd-card .top { display: flex; align-items: center; gap: 8px; font-weight: 600; }
  .cmd-card p { margin: 0; font-size: 12.5px; color: var(--muted); }
  .cmd-card code { font-family: var(--mono); font-size: 11.5px; color: var(--faint);
                   overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block; }

  dialog {
    border: 1px solid var(--border); border-radius: 14px; background: var(--panel); color: var(--text);
    padding: 0; width: min(620px, 94vw); max-height: 88vh;
  }
  dialog::backdrop { background: rgba(0,0,0,.55); }
  .dlg-head { padding: 15px 18px; border-bottom: 1px solid var(--border); display: flex; align-items: center; gap: 9px; }
  .dlg-head b { font-size: 15px; }
  .dlg-head .close { margin-left: auto; border: none; background: none; color: var(--muted); font-size: 21px; line-height: 1; }
  .dlg-body { padding: 16px 18px; overflow-y: auto; max-height: 58vh; }
  .dlg-foot { padding: 12px 18px; border-top: 1px solid var(--border); display: flex; gap: 9px; justify-content: flex-end; }
  .dlg-foot .ghost { background: var(--panel-2); border: 1px solid var(--border); border-radius: 8px; padding: 8px 15px; }
  .field { margin-bottom: 13px; }
  .field label { display: block; font-size: 12.5px; color: var(--muted); margin-bottom: 5px; }
  .field input, .field select {
    width: 100%; background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 8px; padding: 8px 10px;
  }
  .field input:focus, .field select:focus { outline: none; border-color: var(--accent); }
  .field small { display: block; color: var(--faint); font-size: 11.5px; margin-top: 4px; }
  .frow { display: grid; grid-template-columns: 1.4fr 1fr auto auto; gap: 7px; margin-bottom: 7px; align-items: center; }
  .frow .req { display: flex; align-items: center; gap: 5px; font-size: 12px; color: var(--muted); white-space: nowrap; }
  .frow .rm { border: 1px solid var(--border); background: var(--panel-2); border-radius: 7px; padding: 6px 9px; color: var(--muted); }
  .frow .rm:hover { color: var(--err); border-color: var(--err); }
  .addf { border: 1px dashed var(--border); background: none; color: var(--muted);
          border-radius: 8px; padding: 7px 12px; width: 100%; font-size: 13px; }
  .addf:hover { border-color: var(--accent); color: var(--accent); }
  .result { margin-top: 14px; border: 1px solid var(--border); border-radius: 9px; overflow: hidden; }
  .result.err { border-color: var(--err); }
  .result .rh { padding: 7px 11px; font-size: 12px; color: var(--muted); background: var(--panel-2); border-bottom: 1px solid var(--border); }
  .result.err .rh { color: var(--err); }
</style>
</head>
<body>

<aside>
  <div class="brand">nika <span>agent</span><i class="dot" id="status"></i></div>
  <button class="newchat" id="newchat">＋ New chat</button>
  <div class="chats"><h4>Chats</h4><div id="chatlist"></div></div>
  <div class="sidebar-foot">
    <div id="sb-project"></div>
    <div id="sb-model"></div>
    <div id="sb-apps"></div>
  </div>
</aside>

<main>
  <div class="tabs">
    <button class="tab active" data-tab="chat">💬 Chat</button>
    <button class="tab" data-tab="commands">⚡ Commands <i class="count" id="cmd-count"></i></button>
    <span class="spacer"></span>
    <span class="badge" id="mode"></span>
  </div>

  <div class="pane active" id="pane-chat"><div class="feed" id="feed"></div></div>
  <div class="pane" id="pane-commands"><div class="cmd-wrap" id="cmdwrap"></div></div>

  <footer id="composer">
    <div class="suggest-row" id="suggest"></div>
    <div class="composer">
      <textarea id="input" rows="1" placeholder="Ask for anything in this project…" autofocus></textarea>
      <button class="send" id="send">Send</button>
    </div>
    <div class="hint">Enter to send · Shift+Enter for a new line · changes are written to <code id="hint-root"></code></div>
  </footer>
</main>

<dialog id="dlg">
  <form method="dialog"><div class="dlg-head">
    <span id="dlg-icon"></span><b id="dlg-title"></b>
    <button class="close" value="cancel">×</button>
  </div></form>
  <div class="dlg-body">
    <p id="dlg-desc" style="margin:0 0 14px;color:var(--muted);font-size:13px"></p>
    <div id="dlg-fields"></div>
    <div id="dlg-result"></div>
  </div>
  <div class="dlg-foot">
    <button class="ghost" id="dlg-cancel">Close</button>
    <button class="send" id="dlg-run">Run</button>
  </div>
</dialog>

<script>
const token = new URLSearchParams(location.search).get('token') || '';
const $ = (id) => document.getElementById(id);
let info = {apps: [], commands: []};
let chats = [];
let activeChat = null;
let busy = false;

const SUGGESTIONS = [
  'What modules does this project have?',
  'Add a price field (float64, required) to the product model and update the DTOs, response and mapper',
  'Generate a category module with name and slug for sqlite',
  'Run go build ./... and fix any errors',
  'Explain how routing works in this project',
  'Which app should a new orders module go in?',
  'Add an Accept-Language header to every user endpoint',
];

function api(path, options = {}) {
  return fetch(path, Object.assign({}, options, {
    headers: Object.assign({'Content-Type': 'application/json', 'X-Nika-Token': token}, options.headers || {}),
  }));
}
function el(tag, cls, text) {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  if (text !== undefined) node.textContent = text;
  return node;
}
function setBusy(value) {
  busy = value;
  $('send').disabled = value;
  $('status').className = value ? 'dot busy' : 'dot';
}
function scrollFeed() {
  const pane = $('pane-chat');
  pane.scrollTop = pane.scrollHeight;
}

/* ── Tabs ─────────────────────────────────────────────── */
for (const tab of document.querySelectorAll('.tab')) {
  tab.addEventListener('click', () => {
    for (const other of document.querySelectorAll('.tab')) other.classList.toggle('active', other === tab);
    const name = tab.dataset.tab;
    $('pane-chat').classList.toggle('active', name === 'chat');
    $('pane-commands').classList.toggle('active', name === 'commands');
    $('composer').style.display = name === 'chat' ? '' : 'none';
  });
}

/* ── Chat rendering ───────────────────────────────────── */
function feed() { return $('feed'); }

function showEmpty() {
  feed().innerHTML = '';
  const box = el('div', 'empty');
  box.appendChild(el('h3', null, 'Ask for anything in this project'));
  box.appendChild(el('div', null,
    'Add a field to a model, generate a module, rename a route, or just ask how something works. ' +
    'Every change is written straight to your project.'));
  feed().appendChild(box);
}

function addMessage(role, text) {
  const stale = feed().querySelector('.empty');
  if (stale) stale.remove();
  const row = el('div', 'msg ' + role);
  row.appendChild(el('div', 'who', role === 'user' ? '🧑' : '🤖'));
  row.appendChild(el('div', 'bubble', text));
  feed().appendChild(row);
  scrollFeed();
}

function toolLabel(name, args) {
  try {
    const parsed = JSON.parse(args || '{}');
    for (const key of ['path', 'command', 'pattern', 'module']) {
      if (parsed[key]) return name + '  ' + parsed[key];
    }
  } catch (_) {}
  return name;
}

// Renders one run's events into a steps container. Reused for both the live
// stream and replaying a stored transcript, so a chat looks identical either way.
function makeRunRenderer() {
  let steps = null;
  const pending = new Map();
  const ensure = () => {
    if (!steps) { steps = el('div', 'steps'); feed().appendChild(steps); }
    return steps;
  };
  return function handle(event) {
    if (event.kind === 'tool_call') {
      const box = el('details', 'tool running');
      const summary = el('summary');
      summary.appendChild(el('span', 'dot'));
      summary.appendChild(el('span', 'name', toolLabel(event.tool, event.args)));
      box.appendChild(summary);
      const pre = el('pre', null, event.args && event.args !== '{}' ? event.args : '');
      box.appendChild(pre);
      ensure().appendChild(box);
      pending.set(event.tool + ':' + (event.step || 0), {box, pre});
    } else if (event.kind === 'tool_result') {
      const entry = pending.get(event.tool + ':' + (event.step || 0));
      if (entry) {
        entry.box.classList.remove('running');
        if (event.failed) entry.box.classList.add('failed');
        entry.pre.textContent = (entry.pre.textContent ? entry.pre.textContent + '\n\n' : '') + (event.result || '');
      }
    } else if (event.kind === 'thinking' || event.kind === 'status') {
      ensure().appendChild(el('div', 'thinking', event.text));
    } else if (event.kind === 'message') {
      steps = null;
      addMessage('bot', event.text);
    } else if (event.kind === 'error') {
      steps = null;
      feed().appendChild(el('div', 'error', event.text));
    } else if (event.kind === 'done') {
      steps = null;
      if (event.changed && event.changed.length) {
        const box = el('div', 'changed');
        box.appendChild(el('b', null, 'Changed ' + event.changed.length + ' path(s): '));
        box.appendChild(document.createTextNode(event.changed.join(', ')));
        feed().appendChild(box);
      }
    }
    scrollFeed();
  };
}

/* ── Chat list ────────────────────────────────────────── */
async function loadChats(select) {
  chats = (await (await api('/api/chats')).json()).chats || [];
  if (select) activeChat = select;
  if (!activeChat || !chats.some((chat) => chat.id === activeChat)) {
    activeChat = chats.length ? chats[0].id : null;
  }
  renderChatList();
  await openChat(activeChat);
}

function renderChatList() {
  const list = $('chatlist');
  list.innerHTML = '';
  for (const chat of chats) {
    const row = el('div', 'chat-item' + (chat.id === activeChat ? ' active' : ''));
    row.appendChild(el('span', null, chat.messages ? '💬' : '○'));
    row.appendChild(el('span', 'label', chat.title));
    const del = el('button', 'del', '×');
    del.title = 'Delete chat';
    del.addEventListener('click', async (event) => {
      event.stopPropagation();
      if (busy) return;
      await api('/api/chats/' + chat.id, {method: 'DELETE'});
      if (activeChat === chat.id) activeChat = null;
      await loadChats();
    });
    row.appendChild(del);
    row.addEventListener('click', () => { if (!busy) { activeChat = chat.id; renderChatList(); openChat(chat.id); } });
    list.appendChild(row);
  }
}

async function openChat(id) {
  if (!id) { showEmpty(); return; }
  const data = await (await api('/api/chats/' + id)).json();
  feed().innerHTML = '';
  const records = data.records || [];
  if (!records.length) { showEmpty(); return; }
  let render = makeRunRenderer();
  for (const record of records) {
    if (record.kind === 'user') { render = makeRunRenderer(); addMessage('user', record.text); }
    else if (record.event) render(record.event);
  }
  scrollFeed();
}

$('newchat').addEventListener('click', async () => {
  if (busy) return;
  const chat = await (await api('/api/chats', {method: 'POST'})).json();
  await loadChats(chat.id);
  $('input').focus();
});

/* ── Sending ──────────────────────────────────────────── */
async function submit() {
  const text = $('input').value.trim();
  if (!text || busy) return;
  setBusy(true);
  $('input').value = '';
  $('input').style.height = 'auto';
  addMessage('user', text);

  const render = makeRunRenderer();
  try {
    const res = await api('/api/chat', {method: 'POST', body: JSON.stringify({chat: activeChat, message: text})});
    if (!res.ok) throw new Error((await res.text()) || ('HTTP ' + res.status));

    // Parse the SSE stream by hand: EventSource cannot POST.
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    for (;;) {
      const {done, value} = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, {stream: true});
      let split;
      while ((split = buffer.indexOf('\n\n')) >= 0) {
        const chunk = buffer.slice(0, split).trim();
        buffer = buffer.slice(split + 2);
        if (!chunk.startsWith('data:')) continue;
        try { render(JSON.parse(chunk.slice(5).trim())); } catch (_) {}
      }
    }
  } catch (err) {
    render({kind: 'error', text: String(err.message || err)});
  } finally {
    setBusy(false);
    $('input').focus();
    const previous = activeChat;
    chats = (await (await api('/api/chats')).json()).chats || [];
    activeChat = previous;
    renderChatList();
  }
}

$('send').addEventListener('click', submit);
$('input').addEventListener('keydown', (event) => {
  if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); submit(); }
});
$('input').addEventListener('input', () => {
  const box = $('input');
  box.style.height = 'auto';
  box.style.height = Math.min(box.scrollHeight, 200) + 'px';
});

/* ── Commands tab ─────────────────────────────────────── */
function renderCommands() {
  const wrap = $('cmdwrap');
  wrap.innerHTML = '';
  const groups = [];
  for (const command of info.commands) {
    let group = groups.find((entry) => entry.name === command.group);
    if (!group) { group = {name: command.group, items: []}; groups.push(group); }
    group.items.push(command);
  }
  for (const group of groups) {
    const section = el('div', 'cmd-group');
    section.appendChild(el('h3', null, group.name));
    const grid = el('div', 'cmd-grid');
    for (const command of group.items) {
      const card = el('button', 'cmd-card');
      const top = el('div', 'top');
      top.appendChild(el('span', null, command.icon || '•'));
      top.appendChild(el('span', null, command.title));
      card.appendChild(top);
      card.appendChild(el('p', null, command.description));
      card.appendChild(el('code', null, command.preview));
      card.addEventListener('click', () => openCommand(command));
      grid.appendChild(card);
    }
    section.appendChild(grid);
    wrap.appendChild(section);
  }
  $('cmd-count').textContent = info.commands.length;
}

const MONGO_TYPES = ['string','int','int64','float64','bool','time.Time','primitive.ObjectID','[]string','map[string]any'];
const SQL_TYPES = ['string','int','int64','float64','bool','time.Time'];

function openCommand(command) {
  $('dlg-icon').textContent = command.icon || '•';
  $('dlg-title').textContent = command.title;
  $('dlg-desc').textContent = command.description;
  $('dlg-result').innerHTML = '';
  const host = $('dlg-fields');
  host.innerHTML = '';

  const inputs = {};
  let fieldRows = null;

  for (const field of command.fields || []) {
    if (field.kind === 'fields') { fieldRows = buildFieldEditor(host, field, () => inputs.database?.value); continue; }
    const wrap = el('div', 'field');
    wrap.appendChild(el('label', null, field.label + (field.required ? ' *' : '')));

    let control;
    if (field.kind === 'select' || field.kind === 'app') {
      control = document.createElement('select');
      const options = field.kind === 'app' ? (info.apps || []) : (field.options || []);
      if (field.kind === 'app' && !options.length) continue; // single-app project
      if (field.kind === 'app' && !field.required) control.appendChild(new Option('(ask / default)', ''));
      for (const option of options) control.appendChild(new Option(option, option));
      if (field.default) control.value = field.default;
    } else {
      control = document.createElement('input');
      control.placeholder = field.placeholder || '';
      if (field.default) control.value = field.default;
    }
    control.dataset.name = field.name;
    inputs[field.name] = control;
    wrap.appendChild(control);
    if (field.help) wrap.appendChild(el('small', null, field.help));
    host.appendChild(wrap);
  }

  $('dlg-run').onclick = async () => {
    const values = {};
    for (const [name, control] of Object.entries(inputs)) values[name] = control.value;
    const payload = {id: command.id, values, fields: fieldRows ? fieldRows() : []};

    $('dlg-run').disabled = true;
    $('dlg-run').textContent = 'Running…';
    try {
      const result = await (await api('/api/commands/run', {method: 'POST', body: JSON.stringify(payload)})).json();
      showResult(result.ok, result.output);
    } catch (err) {
      showResult(false, String(err.message || err));
    } finally {
      $('dlg-run').disabled = false;
      $('dlg-run').textContent = 'Run';
    }
  };
  $('dlg').showModal();
}

function showResult(ok, output) {
  const host = $('dlg-result');
  host.innerHTML = '';
  const box = el('div', 'result' + (ok ? '' : ' err'));
  box.appendChild(el('div', 'rh', ok ? '✔ Done' : '✖ Failed'));
  box.appendChild(el('pre', 'out', output || '(no output)'));
  host.appendChild(box);
  host.scrollIntoView({behavior: 'smooth', block: 'nearest'});
}

// The repeating model-field editor used by "Generate resource".
function buildFieldEditor(host, spec, databaseOf) {
  const wrap = el('div', 'field');
  wrap.appendChild(el('label', null, spec.label));
  const rows = el('div');
  wrap.appendChild(rows);
  const add = el('button', 'addf', '＋ Add field');
  wrap.appendChild(add);
  if (spec.help) wrap.appendChild(el('small', null, spec.help));
  host.appendChild(wrap);

  function addRow(name = '', type = 'string', required = true) {
    const row = el('div', 'frow');
    const nameInput = document.createElement('input');
    nameInput.placeholder = 'snake_case name';
    nameInput.value = name;
    const typeSelect = document.createElement('select');
    const types = (databaseOf() === 'mongodb') ? MONGO_TYPES : SQL_TYPES;
    for (const option of types) typeSelect.appendChild(new Option(option, option));
    typeSelect.value = types.includes(type) ? type : types[0];
    const req = el('label', 'req');
    const check = document.createElement('input');
    check.type = 'checkbox';
    check.checked = required;
    req.appendChild(check);
    req.appendChild(document.createTextNode('required'));
    const remove = el('button', 'rm', '×');
    remove.addEventListener('click', () => row.remove());
    row.append(nameInput, typeSelect, req, remove);
    rows.appendChild(row);
  }

  add.addEventListener('click', () => addRow());
  addRow();

  return () => Array.from(rows.children).map((row) => ({
    name: row.children[0].value.trim(),
    type: row.children[1].value,
    required: row.children[2].querySelector('input').checked,
  })).filter((field) => field.name);
}

/* ── Boot ─────────────────────────────────────────────── */
$('dlg-cancel').addEventListener('click', () => $('dlg').close());

for (const suggestion of SUGGESTIONS) {
  const button = el('button', null, suggestion);
  button.addEventListener('click', () => { $('input').value = suggestion; $('input').focus(); });
  $('suggest').appendChild(button);
}

// Horizontal wheel scrolling for the suggestion strip: it is one line, so a
// vertical wheel gesture should move it sideways rather than do nothing.
$('suggest').addEventListener('wheel', (event) => {
  if (Math.abs(event.deltaY) > Math.abs(event.deltaX)) {
    event.preventDefault();
    $('suggest').scrollLeft += event.deltaY;
  }
}, {passive: false});

(async function boot() {
  info = await (await api('/api/session')).json();
  // A single-app project reports no apps at all; normalise the shape once here
  // so no renderer has to care whether it got a list, null, or nothing.
  info.apps = info.apps || [];
  info.commands = info.commands || [];
  $('sb-project').innerHTML = '📁 <code>' + info.project + '</code>';
  $('sb-model').innerHTML = '🧠 <code>' + info.model + '</code>';
  $('sb-apps').textContent = info.apps && info.apps.length ? '🧩 ' + info.apps.join(' · ') : '';
  $('hint-root').textContent = info.project;
  $('mode').textContent = info.read_only ? 'read-only' : 'can edit files';
  if (info.read_only) $('mode').classList.add('ro');
  renderCommands();
  await loadChats();
})();
</script>
</body>
</html>`
