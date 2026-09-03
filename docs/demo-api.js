/* Static GitHub Pages demo adapter. It replaces the Go API with deterministic,
   local-only data so every tab can be explored without Apple credentials. */
(function () {
  const originalFetch = window.fetch.bind(window);
  let checkJob = null;
  let downloadJob = null;
  const names = ['1Password', 'Infuse', 'Documents', 'Pixelmator Photo', 'GoodNotes', 'Procreate', 'Carrot Weather', 'Apollo', 'Overcast', 'Spark'];
  const icon = 'https://is1-ssl.mzstatic.com/image/thumb/Purple126/v4/00/00/00/00000000-0000-0000-0000-000000000000/AppIcon-0-0-1x_U007emarketing-0-7-0-0-85-220.png/512x512bb.jpg';
  const json = (data, status = 200) => Promise.resolve({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(data) });
  const body = req => { try { return JSON.parse(req.body || '{}'); } catch (_) { return {}; } };
  const app = (i, name = names[i % names.length]) => ({ appId: 100000000 + i, trackId: 100000000 + i, name, trackName: name, bundleId: `com.demo.${name.toLowerCase().replace(/[^a-z]+/g, '')}`, artistName: 'Demo Developer', version: `3.${i}.1`, price: 0, artworkUrl512: icon, fileSizeBytes: 78000000 + i * 1200000 });
  const responseFor = (url, req) => {
    if (url === '/api/status') return json({ authenticated: true, os: 'windows', account: { email: 'demo@example.com', name: 'Demo Apple ID', directoryServicesId: 'demo' } });
    if (url === '/api/icloud/status') return json({ installed: true, variant: 'demo' });
    if (url === '/api/downloads/active') return json({ success: true, jobs: [] });
    if (url.includes('/api/search/all')) return json({ success: true, official: { results: [app(1), app(2), app(3)] }, removed: { results: [{ appId: 568903335, name: '1Password 7' }, { appId: 497799835, name: 'Tweetbot' }] } });
    if (url.includes('/api/search?')) return json({ success: true, results: [app(1), app(2), app(3)] });
    if (url.includes('/api/removed-apps')) return json({ success: true, results: [] });
    if (url === '/api/auth/revoke') return json({ success: true });
    if (url.includes('/api/purchases')) return json({ success: true, apps: [app(1), app(2)], fetchedAt: new Date().toISOString() });
    if (url.includes('/api/install/devices')) return json({ success: true, devices: [], toolsAvailable: false, tools: [], driver: { available: false } });
    if (url.includes('/api/open-folder')) return json({ success: true });
    if (url.includes('/api/purchase')) return json({ success: true, message: 'Лицензия получена (демо)' });
    if (url.includes('/api/versions')) return json({ success: true, versions: [], items: [] });
    if (url.includes('/api/version-metadata')) return json({ success: true, items: [] });
    if (url === '/api/batch/check' && req.method === 'POST') {
      const b = body(req); const items = b.items || [];
      checkJob = { id: 'demo-check', status: 'running', total: items.length, started: Date.now(), items: items.map((x, i) => ({ appId: x.appId, name: x.name || `Demo App ${i + 1}`, status: 'checking' })) };
      return json({ success: true, jobId: checkJob.id, total: items.length });
    }
    if (url.includes('/api/batch/check/status')) {
      if (!checkJob) return json({});
      const done = Math.min(checkJob.total, Math.floor((Date.now() - checkJob.started) / 180));
      checkJob.items.forEach((x, i) => { if (i < done) { x.status = i % 9 === 4 ? 'error' : 'available'; x.version = `2.${i}.0`; if (x.status === 'error') x.error = 'Демонстрационная ошибка проверки'; } });
      checkJob.done = done; checkJob.progress = checkJob.total ? done / checkJob.total * 100 : 100; checkJob.status = done >= checkJob.total ? 'completed' : 'running';
      return json(checkJob);
    }
    if (url === '/api/batch/download' && req.method === 'POST') {
      const b = body(req); const items = b.items || [];
      downloadJob = { id: 'demo-download', status: 'running', total: items.length, started: Date.now(), items: items.map((x, i) => ({ appId: x.appId, name: x.name || `Demo App ${i + 1}`, status: 'queued', progress: 0, totalBytes: 90000000 + i * 1000000, bytesRead: 0 })) };
      return json({ success: true, batchId: downloadJob.id, total: items.length });
    }
    if (url.includes('/api/batch/download/status')) {
      if (!downloadJob) return json({});
      const elapsed = Math.floor((Date.now() - downloadJob.started) / 250);
      // The 25th item intentionally hangs: this demonstrates the real queue behavior.
      downloadJob.items.forEach((x, i) => { if (i < Math.min(elapsed, 24)) { x.status = 'completed'; x.progress = 100; x.outputPath = `demo/downloads/${x.name}.ipa`; } else if (i === 24 && downloadJob.items.length >= 25) { x.status = 'downloading'; x.progress = 0; } else if (i === Math.min(elapsed, 24)) { x.status = 'downloading'; x.progress = Math.min(99, ((Date.now() - downloadJob.started) % 9000) / 90); } });
      const done = downloadJob.items.filter(x => x.status === 'completed' || x.status === 'error').length;
      downloadJob.progress = downloadJob.items.reduce((n, x) => n + (x.status === 'completed' ? 100 : x.progress || 0), 0) / Math.max(1, downloadJob.total);
      downloadJob.completedCount = done; downloadJob.errors = 0;
      // With fewer than 25 items the demo completes normally.
      if (downloadJob.total < 25 && done === downloadJob.total) downloadJob.status = 'completed';
      return json(downloadJob);
    }
    return json({ success: true, devices: [], jobs: [], apps: [] });
  };
  window.fetch = function (input, init) { const url = typeof input === 'string' ? input : input.url; const req = { method: (init && init.method) || 'GET', body: init && init.body }; if (url.startsWith('/api/')) return responseFor(url, req); return originalFetch(input, init); };
})();
