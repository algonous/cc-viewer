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

var pollTimer = null;

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
}

function loadTranscript(idx) {
  if (idx < 0 || idx >= state.filteredSessions.length) return;
  stopPolling();
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
    startPolling(sid);
  });
}

function startPolling(sid) {
  stopPolling();
  pollTimer = setInterval(function() {
    if (!state.currentSession || state.currentSession.session_id !== sid) {
      stopPolling();
      return;
    }
    fetchJSON('/api/transcript/' + sid).then(function(data) {
      if (!state.transcript || !data || !data.rounds) return;
      var oldLen = state.transcript.rounds.length;
      var newLen = data.rounds.length;
      if (newLen < oldLen) return;

      var content = document.getElementById('viewer-content');
      var changed = false;

      // Check if the last existing round was updated (in-progress round growing).
      if (oldLen > 0 && newLen >= oldLen) {
        var lastIdx = oldLen - 1;
        var oldRound = state.transcript.rounds[lastIdx];
        var newRound = data.rounds[lastIdx];
        if (roundChanged(oldRound, newRound)) {
          state.transcript.rounds[lastIdx] = newRound;
          var oldEl = content.querySelector('.round[data-round-idx="' + lastIdx + '"]');
          if (oldEl) {
            var tmp = document.createElement('div');
            tmp.innerHTML = renderRound(newRound, lastIdx);
            var newEl = tmp.firstChild;
            content.replaceChild(newEl, oldEl);
            initBlocks(newEl);
          }
          changed = true;
        }
      }

      // Append new rounds.
      if (newLen > oldLen) {
        var html = '';
        for (var i = oldLen; i < newLen; i++) {
          state.transcript.rounds.push(data.rounds[i]);
          html += renderRound(data.rounds[i], i);
        }
        content.insertAdjacentHTML('beforeend', html);
        var newRoundEls = content.querySelectorAll('.round');
        for (var j = oldLen; j < newRoundEls.length; j++) {
          initBlocks(newRoundEls[j]);
        }
        changed = true;
      }

      if (!changed) return;

      // Update title and status bar.
      var s = state.currentSession;
      var title = document.getElementById('viewer-title');
      title.textContent = (s ? s.project_name + ' -- ' : '') + state.transcript.rounds.length + ' rounds';
      renderStatusBar();
    });
  }, 2000);
}

// Compare two round objects to detect changes (new blocks, updated content).
function roundChanged(oldR, newR) {
  var ob = oldR.blocks || [];
  var nb = newR.blocks || [];
  if (ob.length !== nb.length) return true;
  for (var i = 0; i < ob.length; i++) {
    if (ob[i].html !== nb[i].html || ob[i].name !== nb[i].name ||
        ob[i].input_summary !== nb[i].input_summary) return true;
  }
  return false;
}

// Initialize fold summaries and toggle handlers for blocks within a container element.
function initBlocks(container) {
  var blocks = container.querySelectorAll('.chat-block');
  for (var i = 0; i < blocks.length; i++) {
    var block = blocks[i];
    var summaryEl = block.querySelector('.fold-summary');
    var body = block.querySelector('.fold-body');
    if (summaryEl && body) {
      var text;
      if (block.classList.contains('chat-tool')) {
        var nameEl = body.querySelector('.tool-name');
        var inputEl = body.querySelector('.tool-input');
        text = (nameEl ? nameEl.textContent : '');
        if (inputEl && inputEl.textContent) text += ': ' + inputEl.textContent;
      } else {
        text = (body.textContent || '').trim().replace(/\s+/g, ' ');
        text = truncate(text, 80);
      }
      summaryEl.textContent = text;
    }
    var header = block.querySelector('.block-header');
    if (header) header.addEventListener('click', foldToggleHandler);
  }
}

function doExport() {
  if (!state.currentSession) return;
  var body = {
    session_id: state.currentSession.session_id,
    format: state.exportFormat,
  };

  // If blocks are selected, export only the selected blocks.
  var blocks = getSelectedBlocks();
  if (blocks) {
    body.blocks = blocks;
  }

  fetch('/api/export', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body),
  }).then(function(r) {
    var disposition = r.headers.get('Content-Disposition') || '';
    var match = disposition.match(/filename="?([^"]+)"?/);
    var filename = match ? match[1] : state.currentSession.session_id + '.' + state.exportFormat;
    return r.blob().then(function(blob) { return {blob: blob, filename: filename}; });
  }).then(function(result) {
    var url = URL.createObjectURL(result.blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = result.filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    setStatus('Downloaded: ' + result.filename);
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
        (s.project && s.project.toLowerCase().indexOf(text) >= 0) ||
        (s.all_messages && s.all_messages.toLowerCase().indexOf(text) >= 0);
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

  // Render blocks in file order.
  var blocks = round.blocks || [];
  for (var i = 0; i < blocks.length; i++) {
    var b = blocks[i];
    var contentHtml;
    if (b.role === 'tool') {
      contentHtml = '<div class="tool-list"><div class="tool-item">' +
        '<span class="tool-name">' + escapeHtml(b.name || '') + '</span>';
      if (b.input_summary) {
        contentHtml += '<span class="tool-input">' + escapeHtml(b.input_summary) + '</span>';
      }
      contentHtml += '</div></div>';
    } else {
      contentHtml = b.html || '';
    }
    html += renderBlock(idx, i, b.role, contentHtml);
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
      var nameEl = body.querySelector('.tool-name');
      var inputEl = body.querySelector('.tool-input');
      text = (nameEl ? nameEl.textContent : '');
      if (inputEl && inputEl.textContent) text += ': ' + inputEl.textContent;
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

// Build a list of [roundIdx, blockIdx] pairs from selected blocks.
// Returns null if no blocks are selected.
function getSelectedBlocks() {
  var keys = Object.keys(state.selectedBlocks);
  var result = [];
  for (var i = 0; i < keys.length; i++) {
    if (!state.selectedBlocks[keys[i]]) continue;
    var parts = keys[i].split('-');
    if (parts.length < 3) continue;
    var roundIdx = parseInt(parts[1], 10);
    var blockIdx = parseInt(parts[2], 10);
    if (isNaN(roundIdx) || isNaN(blockIdx)) continue;
    result.push([roundIdx, blockIdx]);
  }
  result.sort(function(a, b) { return a[0] !== b[0] ? a[0] - b[0] : a[1] - b[1]; });
  return result.length > 0 ? result : null;
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

  // If filter changed, reset viewer state.
  if (state.filteredSessions.length > 0) {
    loadTranscript(0);
  } else {
    stopPolling();
    state.currentSession = null;
    state.transcript = null;
    state.selectedBlocks = {};
    state.sidebarIdx = -1;
    renderViewer();
    updateSelectionUI();
    renderStatusBar();
  }
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
