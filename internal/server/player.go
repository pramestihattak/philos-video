package server

// PlayerHTML is the embedded HTML player page.
const PlayerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>PramTube Player</title>
  <script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      background: #111;
      color: #eee;
      font-family: monospace;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      gap: 16px;
    }
    .player-wrap {
      position: relative;
      width: 100%;
      max-width: 900px;
    }
    video {
      width: 100%;
      display: block;
      background: #000;
    }
    .overlay {
      position: absolute;
      top: 8px;
      left: 8px;
      background: rgba(0,0,0,0.65);
      padding: 6px 10px;
      border-radius: 4px;
      font-size: 12px;
      line-height: 1.6;
      pointer-events: none;
    }
    .controls {
      display: flex;
      align-items: center;
      gap: 12px;
      font-size: 13px;
    }
    select {
      background: #222;
      color: #eee;
      border: 1px solid #444;
      padding: 4px 8px;
      border-radius: 4px;
      cursor: pointer;
    }
  </style>
</head>
<body>
  <div class="player-wrap">
    <video id="video" controls playsinline></video>
    <div class="overlay" id="overlay">Loading…</div>
  </div>
  <div class="controls">
    <label for="quality">Quality:</label>
    <select id="quality">
      <option value="-1">Auto</option>
    </select>
  </div>

  <script>
    const video = document.getElementById('video');
    const overlay = document.getElementById('overlay');
    const qualitySelect = document.getElementById('quality');

    const src = '/videos/master.m3u8';

    const stats = {
      level: 'Auto',
      bandwidth: 0,
      buffer: 0,
      segTime: 0,
    };

    function updateOverlay() {
      overlay.innerHTML =
        'Quality: ' + stats.level + '<br>' +
        'Bandwidth: ' + (stats.bandwidth / 1000).toFixed(0) + ' kbps<br>' +
        'Buffer: ' + stats.buffer.toFixed(1) + 's<br>' +
        'Last seg: ' + stats.segTime.toFixed(0) + ' ms';
    }

    if (Hls.isSupported()) {
      const hls = new Hls({ debug: false });
      hls.loadSource(src);
      hls.attachMedia(video);

      hls.on(Hls.Events.MANIFEST_PARSED, (event, data) => {
        data.levels.forEach((l, i) => {
          const opt = document.createElement('option');
          opt.value = i;
          opt.textContent = l.height + 'p (' + Math.round(l.bitrate / 1000) + ' kbps)';
          qualitySelect.appendChild(opt);
        });
      });

      hls.on(Hls.Events.LEVEL_SWITCHED, (event, data) => {
        const l = hls.levels[data.level];
        stats.level = l ? l.height + 'p' : 'Auto';
        updateOverlay();
      });

      hls.on(Hls.Events.FRAG_LOADED, (event, data) => {
        stats.bandwidth = hls.bandwidthEstimate || 0;
        stats.segTime = data.stats.loading.end - data.stats.loading.start;
        updateOverlay();
      });

      hls.on(Hls.Events.BUFFER_APPENDED, () => {
        const buf = video.buffered;
        if (buf.length > 0) {
          stats.buffer = buf.end(buf.length - 1) - video.currentTime;
        }
        updateOverlay();
      });

      qualitySelect.addEventListener('change', () => {
        hls.currentLevel = parseInt(qualitySelect.value, 10);
      });

    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = src;
      overlay.textContent = 'Native HLS (no stats overlay)';
    } else {
      overlay.textContent = 'HLS not supported in this browser.';
    }
  </script>
</body>
</html>`
