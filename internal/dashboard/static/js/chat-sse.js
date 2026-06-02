// AgentForge SSE Chat — real-time streaming chat client
(function() {
  var streaming = false;
  var currentAbortController = null;

  window.sendChat = function() {
    if (streaming) return;
    var inp = document.getElementById('chat-input');
    var msg = inp.value.trim();
    if (!msg) return;
    var div = document.getElementById('chat-messages');

    // Add user message
    div.innerHTML += '<div class="chat-msg user">' + escapeHtml(msg) + '</div>';
    inp.value = '';
    div.scrollTop = div.scrollHeight;

    streaming = true;
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

    currentAbortController = new AbortController();

    fetch('/api/chat/stream', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({prompt: msg, agent: 'agentforge', model: ''}),
      signal: currentAbortController.signal
    }).then(function(resp) {
      if (!resp.ok) { endStream('HTTP error: ' + resp.status); return; }
      var reader = resp.body.getReader();
      var decoder = new TextDecoder();
      var buffer = '';

      function pump() {
        return reader.read().then(function(result) {
          if (result.done) { return; }
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
              eventType = ''; // Reset after processing
            }
            // Empty lines are SSE event boundaries, reset eventType
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
    var out = document.getElementById('stream-output');
    if (!out) return;

    // Remove typing indicator on first real chunk
    var typing = document.getElementById('typing-indicator');
    if (typing) typing.style.display = 'none';

    switch(event) {
      case 'chunk':
        if (d.content) {
          out.textContent += d.content;
        }
        break;
      case 'tool_call':
        handleToolCall(d);
        break;
      case 'status':
        handleStatusEvent(d);
        break;
      case 'done':
        endStream(null);
        break;
      case 'error':
        endStream(d.error || 'Unknown error');
        break;
      case 'ping':
        // keep-alive, ignore
        break;
    }
    var div = document.getElementById('chat-messages');
    div.scrollTop = div.scrollHeight;
  }

  function handleToolCall(d) {
    if (!d.name) return;
    var div = document.getElementById('chat-messages');

    // Find or create tool progress element
    var toolId = 'tool-' + d.id;
    var existing = document.getElementById(toolId);
    if (d.done) {
      if (existing) {
        existing.className = 'tool-progress tool-done';
        existing.innerHTML = d.error
          ? '<span class="tool-error">✗</span> ' + escapeHtml(d.name) + ': ' + escapeHtml(d.error)
          : '<span class="tool-done">✓</span> ' + escapeHtml(d.name);
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

  function endStream(err) {
    streaming = false;
    document.getElementById('send-btn').disabled = false;
    var out = document.getElementById('stream-output');
    if (out) {
      out.classList.remove('streaming');
      if (err) {
        out.innerHTML = '<span class="chat-error">Error: ' + escapeHtml(err) + '</span>';
      }
    }
    // Clean up tool progress indicators
    var spinners = document.querySelectorAll('.tool-progress .tool-spinner');
    spinners.forEach(function(s) {
      var tool = s.parentElement;
      if (tool) {
        tool.classList.add('tool-stale');
        s.innerHTML = '⏱';
      }
    });
    // Remove status message
    var statusMsg = document.getElementById('status-message');
    if (statusMsg) statusMsg.remove();
    // Remove typing indicator
    var typing = document.getElementById('typing-indicator');
    if (typing) typing.remove();
  }

  function escapeHtml(s) {
    return s.replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
  }
})();