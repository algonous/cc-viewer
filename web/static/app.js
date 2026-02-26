'use strict';

var state = {
  sessions: [],
  filteredSessions: [],
  sidebarIdx: 0,
  transcript: null,
  currentSession: null,
  selectedBlocks: {},
  filterText: '',
  exportFormat: 'jsonl',
};

// Blocks that start folded by default.
var FOLD_CLOSED = {context: true, tool: true, thinking: true};

// --- API ---

function fetchJSON(url) {
  return fetch(url).then(function(r) { return r.json(); });
}

function loadTranscript(idx) {
  if (idx < 0 || idx >= state.filteredSessions.length) return;
  state.sidebarIdx = idx;
  state.currentSession = state.filteredSessions[idx];
  state.selectedBlocks = {};
  renderSidebar();

  var sid = state.currentSession.session_id;
  history.replaceState(null, '', '/' + sid);
  fetchJSON('/api/transcript/' + sid).then(function(data) {
    state.transcript = data;
    renderViewer();
    updateSelectionUI();
    renderStatusBar();
  });
}

function doExport() {
  if (!state.currentSession) return;
  var body = {
    session_id: state.currentSession.session_id,
    format: state.exportFormat,
    include_thinking: document.getElementById('export-thinking').checked,
  };
  fetch('/api/export', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body),
  }).then(function(r) { return r.json(); }).then(function(data) {
    if (data.path) {
      setStatus('Exported: ' + data.path);
    }
  });
  closeExportModal();
}

// --- Filter ---

function applyFilter() {
  var text = state.filterText.toLowerCase();
  if (!text) {
    state.filteredSessions = state.sessions.slice();
  } else {
    state.filteredSessions = state.sessions.filter(function(s) {
      return (s.project_name && s.project_name.toLowerCase().indexOf(text) >= 0) ||
        (s.first_message && s.first_message.toLowerCase().indexOf(text) >= 0) ||
        (s.project && s.project.toLowerCase().indexOf(text) >= 0);
    });
  }
}

// --- Render ---

function renderSidebar() {
  var list = document.getElementById('session-list');
  var html = '';
  for (var i = 0; i < state.filteredSessions.length; i++) {
    var s = state.filteredSessions[i];
    var active = i === state.sidebarIdx ? ' active' : '';
    var ts = formatTime(s.last_ts);
    var msg = escapeHtml(truncate(s.first_message || '', 60));
    html += '<div class="session-item' + active + '" data-idx="' + i + '">' +
      '<div><span class="session-project">' + escapeHtml(s.project_name || '?') + '</span>' +
      '<span class="session-time">' + ts + '</span></div>' +
      '<div class="session-message">"' + msg + '"</div></div>';
  }
  if (state.filteredSessions.length === 0) {
    html = '<div class="empty-state">No sessions</div>';
  }
  list.innerHTML = html;

  var activeEl = list.querySelector('.session-item.active');
  if (activeEl) activeEl.scrollIntoView({block: 'nearest'});
}

function renderViewer() {
  var title = document.getElementById('viewer-title');
  var content = document.getElementById('viewer-content');

  if (!state.transcript || !state.transcript.rounds) {
    title.textContent = '';
    content.innerHTML = '<div class="empty-state">Select a session</div>';
    return;
  }

  var rounds = state.transcript.rounds;
  var s = state.currentSession;
  title.textContent = (s ? s.project_name + ' -- ' : '') + rounds.length + ' rounds';

  var html = '';
  for (var i = 0; i < rounds.length; i++) {
    html += renderRound(rounds[i], i);
  }
  content.innerHTML = html;

  // Fill fold summaries from rendered content, then attach toggle handlers.
  fillFoldSummaries();
  var headers = content.querySelectorAll('.block-header');
  for (var j = 0; j < headers.length; j++) {
    headers[j].addEventListener('click', foldToggleHandler);
  }
}

function renderRound(round, idx) {
  var ts = formatShortTime(round.user_timestamp || '');
  var tokens = '';
  if (round.usage) {
    var u = round.usage;
    tokens = '<span class="round-tokens">' +
      'in:' + compactNum(u.input_tokens) +
      ' out:' + compactNum(u.output_tokens) +
      ' cr:' + compactNum(u.cache_read) +
      ' cw:' + compactNum(u.cache_creation) +
      '</span>';
  }

  var html = '<div class="round" data-round-idx="' + idx + '">' +
    '<div class="round-header">' +
    '<span class="round-index">#' + (round.index + 1) + '</span>' +
    '<span class="round-timestamp">' + escapeHtml(ts) + '</span>' +
    tokens +
    '</div>';

  // Render each block from structured data.
  var blockIdx = 0;

  // User / Context block.
  if (round.user_html) {
    var role = round.is_context ? 'context' : 'you';
    html += renderBlock(idx, blockIdx++, role, round.user_html);
  }

  // Tool calls block.
  if (round.tool_calls && round.tool_calls.length > 0) {
    var toolHtml = renderToolCalls(round.tool_calls);
    html += renderBlock(idx, blockIdx++, 'tool', toolHtml);
  }

  // Thinking block.
  if (round.thinking_html) {
    html += renderBlock(idx, blockIdx++, 'thinking', round.thinking_html);
  }

  // Assistant block.
  if (round.assistant_html) {
    html += renderBlock(idx, blockIdx++, 'claude', round.assistant_html);
  }

  html += '</div>';
  return html;
}

// Render a single chat block with fold toggle and checkbox.
function renderBlock(roundIdx, blockIdx, role, contentHtml) {
  var blockId = 'b-' + roundIdx + '-' + blockIdx;
  var roleLabel = role.toUpperCase();
  var startOpen = !FOLD_CLOSED[role];
  var checked = state.selectedBlocks[blockId] ? ' checked' : '';
  var checkedClass = state.selectedBlocks[blockId] ? ' block-checked' : '';

  // Summary: first ~80 chars of text content for the folded preview.
  var summary = '';
  if (role === 'tool') {
    // Already handled in the fold summary by the tool call count text.
    // We'll set it below.
  }
  // We'll extract summary from a temp element later -- for now use a placeholder.
  // Actually, let's just build the summary inline from data.

  var html = '<div class="chat-block chat-' + role + checkedClass + '" data-block-id="' + blockId + '">';

  // Header row: checkbox + fold arrow + role label + summary.
  html += '<div class="block-header">' +
    '<input type="checkbox" class="block-checkbox" data-block-id="' + blockId + '"' + checked + '>' +
    '<span class="fold-arrow' + (startOpen ? ' open' : '') + '">&#9654;</span>' +
    '<span class="chat-role">' + roleLabel + '</span>' +
    '<span class="fold-summary"></span>' +
    '</div>';

  // Fold body.
  html += '<div class="fold-body' + (startOpen ? ' open' : '') + '">' + contentHtml + '</div>';

  html += '</div>';
  return html;
}

// Render tool calls as structured HTML (not via markdown).
function renderToolCalls(toolCalls) {
  var html = '<div class="tool-list">';
  for (var i = 0; i < toolCalls.length; i++) {
    var tc = toolCalls[i];
    html += '<div class="tool-item">' +
      '<span class="tool-name">' + escapeHtml(tc.name) + '</span>';
    if (tc.input_summary) {
      html += '<span class="tool-input">' + escapeHtml(tc.input_summary) + '</span>';
    }
    html += '</div>';
  }
  html += '</div>';
  return html;
}

// After innerHTML is set, fill in fold summaries from rendered content.
function fillFoldSummaries() {
  var blocks = document.querySelectorAll('#viewer-content .chat-block');
  for (var i = 0; i < blocks.length; i++) {
    var block = blocks[i];
    var summaryEl = block.querySelector('.fold-summary');
    if (!summaryEl) continue;

    var body = block.querySelector('.fold-body');
    if (!body) continue;

    var text = '';
    if (block.classList.contains('chat-tool')) {
      var items = body.querySelectorAll('.tool-item');
      text = items.length + ' tool call' + (items.length !== 1 ? 's' : '');
    } else {
      text = (body.textContent || '').trim().replace(/\s+/g, ' ');
      text = truncate(text, 80);
    }
    summaryEl.textContent = text;
  }
}

function foldToggleHandler(e) {
  // Don't toggle fold when clicking the checkbox.
  if (e.target.classList.contains('block-checkbox')) return;
  var block = this.closest('.chat-block');
  if (!block) return;
  var body = block.querySelector('.fold-body');
  var arrow = block.querySelector('.fold-arrow');
  if (body) body.classList.toggle('open');
  if (arrow) arrow.classList.toggle('open');
}

function updateSelectionUI() {
  var count = selectedBlockCount();
  var actions = document.getElementById('selection-actions');
  if (count > 0) {
    actions.classList.remove('hidden');
  } else {
    actions.classList.add('hidden');
  }
  renderStatusBar();
}

function renderStatusBar() {
  var bar = document.getElementById('status-bar');
  var count = selectedBlockCount();
  var parts = [];
  parts.push(state.filteredSessions.length + ' sessions');
  if (state.transcript) {
    parts.push(state.transcript.rounds.length + ' rounds');
  }
  if (count > 0) {
    parts.push(count + ' block' + (count !== 1 ? 's' : '') + ' selected');
  }
  bar.textContent = parts.join('  |  ');
}

function selectedBlockCount() {
  var count = 0;
  var keys = Object.keys(state.selectedBlocks);
  for (var i = 0; i < keys.length; i++) {
    if (state.selectedBlocks[keys[i]]) count++;
  }
  return count;
}

// --- Selection actions ---

function dumpSelected() {
  var parts = [];
  var blocks = document.querySelectorAll('#viewer-content .chat-block[data-block-id]');
  for (var i = 0; i < blocks.length; i++) {
    var bid = blocks[i].getAttribute('data-block-id');
    if (state.selectedBlocks[bid]) {
      var roleEl = blocks[i].querySelector('.chat-role');
      var role = roleEl ? roleEl.textContent.trim() : '';
      var body = blocks[i].querySelector('.fold-body');
      var text = body ? body.textContent.trim() : '';
      if (role) {
        parts.push('[' + role + ']\n' + text);
      } else {
        parts.push(text);
      }
    }
  }
  var result = parts.join('\n\n---\n\n');
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(result).then(function() {
      setStatus('Copied ' + parts.length + ' block(s) to clipboard');
    });
  }
}

function clearAllSelections() {
  state.selectedBlocks = {};
  var checkboxes = document.querySelectorAll('#viewer-content .block-checkbox');
  for (var i = 0; i < checkboxes.length; i++) {
    checkboxes[i].checked = false;
    var block = checkboxes[i].closest('.chat-block');
    if (block) block.classList.remove('block-checked');
  }
  updateSelectionUI();
}

// --- Export modal ---

function openExportModal() {
  state.exportFormat = 'jsonl';
  var modal = document.getElementById('export-modal');
  modal.classList.remove('hidden');
  updateFormatButtons();
}

function closeExportModal() {
  document.getElementById('export-modal').classList.add('hidden');
}

function updateFormatButtons() {
  var btns = document.querySelectorAll('.modal-options .modal-btn');
  for (var i = 0; i < btns.length; i++) {
    btns[i].classList.toggle('selected', btns[i].getAttribute('data-format') === state.exportFormat);
  }
}

// --- Status message ---

var statusTimeout = null;
function setStatus(msg) {
  var bar = document.getElementById('status-bar');
  bar.textContent = msg;
  clearTimeout(statusTimeout);
  statusTimeout = setTimeout(function() { renderStatusBar(); }, 3000);
}

// --- Event handlers ---

// Filter input.
document.getElementById('filter-input').addEventListener('input', function() {
  state.filterText = this.value;
  applyFilter();
  renderSidebar();
});

// Session click.
document.getElementById('session-list').addEventListener('click', function(e) {
  var item = e.target.closest('.session-item');
  if (item) {
    var idx = parseInt(item.getAttribute('data-idx'), 10);
    loadTranscript(idx);
  }
});

// Block checkbox toggle (delegated).
document.getElementById('viewer-content').addEventListener('change', function(e) {
  if (e.target.classList.contains('block-checkbox')) {
    var blockId = e.target.getAttribute('data-block-id');
    state.selectedBlocks[blockId] = e.target.checked;
    var block = e.target.closest('.chat-block');
    if (block) block.classList.toggle('block-checked', e.target.checked);
    updateSelectionUI();
  }
});

// Export button.
document.getElementById('btn-export').addEventListener('click', function() {
  if (state.currentSession) openExportModal();
});

// Dump button.
document.getElementById('btn-dump').addEventListener('click', dumpSelected);

// Clear button.
document.getElementById('btn-clear').addEventListener('click', clearAllSelections);

// Export modal buttons.
var formatBtns = document.querySelectorAll('.modal-options .modal-btn');
for (var fi = 0; fi < formatBtns.length; fi++) {
  formatBtns[fi].addEventListener('click', function() {
    state.exportFormat = this.getAttribute('data-format');
    updateFormatButtons();
  });
}

document.getElementById('export-confirm').addEventListener('click', doExport);
document.getElementById('export-cancel').addEventListener('click', closeExportModal);

// Close modal on backdrop click.
document.getElementById('export-modal').addEventListener('click', function(e) {
  if (e.target === this) closeExportModal();
});

// --- Helpers ---

function compactNum(n) {
  if (n === undefined || n === null) return '0';
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
  return String(n);
}

function formatShortTime(isoStr) {
  if (!isoStr) return '';
  var d = new Date(isoStr);
  if (isNaN(d.getTime())) return isoStr;
  var yyyy = d.getFullYear();
  var mm = String(d.getMonth() + 1).padStart(2, '0');
  var dd = String(d.getDate()).padStart(2, '0');
  var hh = String(d.getHours()).padStart(2, '0');
  var mi = String(d.getMinutes()).padStart(2, '0');
  var ss = String(d.getSeconds()).padStart(2, '0');
  return yyyy + '-' + mm + '-' + dd + ' ' + hh + ':' + mi + ':' + ss;
}

function formatTime(unixMs) {
  if (!unixMs) return '';
  var d = new Date(unixMs);
  var mm = String(d.getMonth() + 1).padStart(2, '0');
  var dd = String(d.getDate()).padStart(2, '0');
  var hh = String(d.getHours()).padStart(2, '0');
  var mi = String(d.getMinutes()).padStart(2, '0');
  return mm + '/' + dd + ' ' + hh + ':' + mi;
}

function truncate(str, max) {
  if (str.length <= max) return str;
  return str.substring(0, max) + '...';
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// --- Init ---

function initFromURL() {
  var path = window.location.pathname;
  if (path.length > 1) {
    state._initialSessionID = path.substring(1);
  }
}

function loadSessions() {
  fetchJSON('/api/sessions').then(function(data) {
    state.sessions = data || [];
    applyFilter();
    renderSidebar();
    renderStatusBar();

    var targetIdx = 0;
    if (state._initialSessionID) {
      for (var i = 0; i < state.filteredSessions.length; i++) {
        if (state.filteredSessions[i].session_id === state._initialSessionID) {
          targetIdx = i;
          break;
        }
      }
      delete state._initialSessionID;
    }
    if (state.filteredSessions.length > 0) {
      loadTranscript(targetIdx);
    }
  });
}

initFromURL();
loadSessions();
