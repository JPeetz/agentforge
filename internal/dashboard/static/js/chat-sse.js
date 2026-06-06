// AgentForge SSE Chat — real-time streaming chat client with Markdown support
(function() {
  // State lives on window.agChat so it survives HTMX partial swaps
  if (!window.agChat) {
    window.agChat = { streaming: false, abortController: null, model: '' };
    // Restore model from config
    fetch('/api/config').then(function(r) { return r.json(); }).then(function(cfg) {
      window.agChat.model = (cfg && cfg.llm && cfg.llm.model) ? cfg.llm.model : '';
    }).catch(function() {});
  }

  // Expose reset for partial reload IIFE
  window.agChatReset = function() {
    window.agChat.streaming = false;
    window.agChat.abortController = null;
    var btn = document.getElementById('send-btn');
    if (btn) btn.disabled = false;
    var out = document.getElementById('stream-output');
    if (out) out.classList.remove('streaming');
    var typing = document.getElementById('typing-indicator');
    if (typing) typing.remove();
  };

  // Keep local aliases for readability; they read/write through window.agChat
  var get = function(k) { return window.agChat[k]; };
  var set = function(k, v) { window.agChat[k] = v; };

  window.sendChat = function() {
    if (get('streaming')) return;
    var inp = document.getElementById('chat-input');
    var msg = inp.value.trim();
    if (!msg) return;
    var div = document.getElementById('chat-messages');

    // Remove previous stream-output id to avoid duplicates
    var prev = document.getElementById('stream-output');
    if (prev) prev.removeAttribute('id');

    // Add user message
    var userMsg = document.createElement('div');
    userMsg.className = 'chat-msg user';
    userMsg.textContent = msg;
    div.appendChild(userMsg);
    inp.value = '';
    div.scrollTop = div.scrollHeight;

    set('streaming', true);
    document.getElementById('send-btn').disabled = true;

    // Create streaming output container
    var streamDiv = document.createElement('div');
    streamDiv.className = 'chat-msg agent streaming';
    streamDiv.id = 'stream-output';
    div.appendChild(streamDiv);
    div.scrollTop = div.scrollHeight;

    // Show typing indicator
    var typing = document.createElement('div');
    typing.className = 'typing-indicator';
    typing.id = 'typing-indicator';
    typing.innerHTML = '<span></span><span></span><span></span>';
    streamDiv.appendChild(typing);

    var abortCtrl = new AbortController();
    set('abortController', abortCtrl);

    fetch('/api/chat/stream', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({prompt: msg, agent: 'agentforge', model: get('model')}),
      signal: abortCtrl.signal
    }).then(function(resp) {
      if (!resp.ok) { endStream('HTTP error: ' + resp.status); return; }
      var reader = resp.body.getReader();
      var decoder = new TextDecoder();
      var buffer = '';

      function pump() {
        return reader.read().then(function(result) {
          if (result.done) {
            // Flush any remaining bytes in decoder
            var tail = decoder.decode();
            if (tail) {
              buffer += tail;
              var lines = buffer.split('\n');
              buffer = '';
              var eventType = '';
              for (var i = 0; i < lines.length; i++) {
                var line = lines[i];
                if (line.startsWith('event: ')) { eventType = line.slice(7).trim(); }
                else if (line.startsWith('data: ')) { handleSSE(eventType, line.slice(6)); eventType = ''; }
                if (line === '') { eventType = ''; }
              }
            }
            // Fallback: if done event never arrived, clean up
            if (get('streaming')) endStream(null);
            return;
          }
          buffer += decoder.decode(result.value, {stream: true});
          var lines = buffer.split('\n');
          buffer = lines.pop() || '';

          var eventType = '';
          for (var i = 0; i < lines.length; i++) {
            var line = lines[i];
            if (line.startsWith('event: ')) {
              eventType = line.slice(7).trim();
            } else if (line.startsWith('data: ')) {
              var data = line.slice(6);
              handleSSE(eventType, data);
              eventType = '';
            }
            if (line === '') {
              eventType = '';
            }
          }
          return pump();
        });
      }
      return pump();
    }).catch(function(e) {
      if (e.name !== 'AbortError') {
        endStream('Network error: ' + e.message);
      }
    });
  };

  function handleSSE(event, data) {
    try { var d = JSON.parse(data); } catch(e) { return; }

    // chunk/done/error events don't need #stream-output to exist — endStream is safe regardless
    var out = document.getElementById('stream-output');

    var typing = document.getElementById('typing-indicator');
    if (typing) typing.style.display = 'none';

    switch(event) {
      case 'chunk':
        if (d.content) {
          appendStreamChunk(out, d.content);
        }
        break;
      case 'tool_call':
        handleToolCall(d);
        break;
      case 'status':
        handleStatusEvent(d);
        break;
      case 'cost_update':
        handleCostUpdate(d);
        break;
      case 'done':
        endStream(null);
        break;
      case 'error':
        endStream(d.error || 'Unknown error');
        break;
      case 'ping':
        break;
    }
    var div = document.getElementById('chat-messages');
    div.scrollTop = div.scrollHeight;
  }

  function appendStreamChunk(container, content) {
    // Find or create content div
    var contentDiv = container.querySelector('.stream-content');
    if (!contentDiv) {
      contentDiv = document.createElement('div');
      contentDiv.className = 'stream-content';
      container.appendChild(contentDiv);
    }

    // Append raw content
    contentDiv.textContent += content;
  }

  function renderMarkdown(container, text) {
    var html = escapeHtml(text);

    // Code blocks (triple backticks)
    html = html.replace(/```([^\n]*)\n([\s\S]*?)```/g, function(match, lang, code) {
      var copyBtn = '<button class="code-block-copy" onclick="copyCode(this)" title="Copy code">Copy</button>';
      return '<pre style="position:relative;background:rgba(0,0,0,0.3);padding:10px 12px;border-radius:6px;overflow-x:auto;margin:6px 0;border:1px solid rgba(139,134,128,0.2)">' + copyBtn + '<code>' + code.trim() + '</code></pre>';
    });

    // Inline code
    html = html.replace(/`([^`]+)`/g, '<code style="background:rgba(0,0,0,0.3);padding:2px 6px;border-radius:3px;font-family:monospace;font-size:12px">$1</code>');

    // Bold
    html = html.replace(/\*\*([^\*]+)\*\*/g, '<strong>$1</strong>');

    // Italic
    html = html.replace(/\*([^\*]+)\*/g, '<em>$1</em>');

    // Line breaks
    html = html.replace(/\n/g, '<br>');

    container.innerHTML = html;
  }

  window.copyCode = function(btn) {
    var codeBlock = btn.nextElementSibling;
    var text = codeBlock.textContent;
    navigator.clipboard.writeText(text).then(function() {
      var oldText = btn.textContent;
      btn.textContent = '✓ Copied!';
      btn.style.background = 'rgba(34,197,94,0.4)';
      setTimeout(function() {
        btn.textContent = oldText;
        btn.style.background = '';
      }, 2000);
    }).catch(function(err) {
      console.error('Failed to copy:', err);
      btn.textContent = 'Copy failed';
    });
  };

  function handleToolCall(d) {
    if (!d.name) return;
    var div = document.getElementById('chat-messages');

    var toolId = 'tool-' + d.id;
    var existing = document.getElementById(toolId);
    if (d.done) {
      if (existing) {
        existing.className = 'tool-progress tool-done';
        existing.innerHTML = d.error
          ? '<span style="color:#EF4444">✗</span> ' + escapeHtml(d.name) + ': ' + escapeHtml(d.error)
          : '<span style="color:#22C55E">✓</span> ' + escapeHtml(d.name);
      }
    } else {
      if (!existing) {
        var tool = document.createElement('div');
        tool.className = 'tool-progress';
        tool.id = toolId;
        tool.innerHTML = '<span class="tool-spinner"></span> Calling ' + escapeHtml(d.name) + '...';
        div.insertBefore(tool, document.getElementById('stream-output'));
      }
    }
  }

  function handleStatusEvent(d) {
    if (!d.message) return;
    var existing = document.getElementById('status-message');
    if (existing) {
      existing.textContent = d.message;
    } else {
      var status = document.createElement('div');
      status.className = 'chat-status';
      status.id = 'status-message';
      status.textContent = d.message;
      document.getElementById('chat-messages').insertBefore(
        status, document.getElementById('stream-output')
      );
    }
  }

  function handleCostUpdate(d) {
    var costDisplay = document.getElementById('chat-cost-display');
    if (!costDisplay) return;

    costDisplay.style.display = 'block';

    if (d.total_cost !== undefined) {
      var costEl = document.getElementById('session-total-cost');
      if (costEl) {
        costEl.textContent = '$' + d.total_cost.toFixed(4);
      }
    }

    var totalTokens = (d.input_tokens || 0) + (d.output_tokens || 0) + (d.cached_tokens || 0);
    var tokenEl = document.getElementById('session-token-count');
    if (tokenEl) {
      tokenEl.textContent = totalTokens;
    }

    var inputEl = document.getElementById('session-input-tokens');
    if (inputEl && d.input_tokens !== undefined) {
      inputEl.textContent = d.input_tokens;
    }

    var outputEl = document.getElementById('session-output-tokens');
    if (outputEl && d.output_tokens !== undefined) {
      outputEl.textContent = d.output_tokens;
    }
  }

  function endStream(err) {
    // Critical state reset — must happen regardless of DOM state
    set('streaming', false);
    set('abortController', null);
    var btn = document.getElementById('send-btn');
    if (btn) btn.disabled = false;

    // DOM cleanup — safe to skip if partial was swapped
    var out = document.getElementById('stream-output');
    if (out) {
      out.classList.remove('streaming');
      if (err) {
        out.innerHTML = '<span class="chat-error">Error: ' + escapeHtml(err) + '</span>';
      } else {
        var contentDiv = out.querySelector('.stream-content');
        if (contentDiv) {
          renderMarkdown(contentDiv, contentDiv.textContent);
        }
      }
    }
    var spinners = document.querySelectorAll('.tool-progress .tool-spinner');
    spinners.forEach(function(s) {
      var tool = s.parentElement;
      if (tool) {
        tool.classList.add('tool-stale');
        s.innerHTML = '⏱';
      }
    });
    var statusMsg = document.getElementById('status-message');
    if (statusMsg) statusMsg.remove();
    var typing = document.getElementById('typing-indicator');
    if (typing) typing.remove();
  }

  function escapeHtml(s) {
    return s.replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
  }

  // Expose for history rendering: reads data-raw attribute, applies markdown
  window.agChatRenderHistory = function(el) {
    var raw = el.getAttribute('data-raw');
    if (!raw) return;
    var contentDiv = document.createElement('div');
    contentDiv.className = 'stream-content';
    el.appendChild(contentDiv);
    renderMarkdown(contentDiv, raw);
  };
})();
