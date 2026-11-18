/* CLUSTERX design prototype behavior (shared). Vanilla JS, no network. Every control in the prototypes is wired here or omitted. */
(function () {
  'use strict';
  var root = document.documentElement;
  root.classList.add('js');
  var mq = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : { matches: false, addEventListener: function () {} };
  var reduced = function () { return mq.matches || root.getAttribute('data-rm') === '1'; };
  root.setAttribute('data-rm', mq.matches ? '1' : '0');
  if (mq.addEventListener) mq.addEventListener('change', function () { root.setAttribute('data-rm', mq.matches ? '1' : '0'); });

  var $ = function (s, c) { return (c || document).querySelector(s); };
  var $$ = function (s, c) { return Array.prototype.slice.call((c || document).querySelectorAll(s)); };
  var CLX = (window.CLX = window.CLX || {});

  /* polite status region (one per page) */
  var status = $('#status-live');
  function say(msg) { if (status) { status.textContent = ''; setTimeout(function () { status.textContent = msg; }, 30); } }
  CLX.say = say;
  var toastTimer;
  function toast(msg) {
    var t = $('#toast'); if (!t) { t = document.createElement('div'); t.id = 'toast'; t.className = 'toast-live'; t.setAttribute('role', 'status'); document.body.appendChild(t); }
    t.textContent = msg; t.hidden = false; clearTimeout(toastTimer); toastTimer = setTimeout(function () { t.hidden = true; }, 3800);
  }
  CLX.toast = toast;

  /* ---- hash state: #state=empty&role=viewer ---- */
  function params() { var o = {}; (location.hash || '').replace(/^#/, '').split('&').forEach(function (p) { var kv = p.split('='); if (kv[0]) o[kv[0]] = decodeURIComponent(kv[1] || ''); }); return o; }
  function setParams(o) { var cur = params(); for (var k in o) cur[k] = o[k]; location.hash = Object.keys(cur).map(function (k) { return k + '=' + encodeURIComponent(cur[k]); }).join('&'); }
  CLX.params = params; CLX.setParams = setParams;
  var LIVE = {
    live: ['g-online', 'Live'], reconnecting: ['i-refresh', 'Reconnecting'], polling: ['i-refresh', 'Polling every 15 s'],
    stale: ['g-stale', 'Stale'], offline: ['i-signal-off', 'Offline'], paused: ['i-pausebar', 'Paused (tab hidden)'], demo: ['i-dot', 'Demo data']
  };
  function setLive(kind) {
    $$('.live').forEach(function (el) {
      var d = LIVE[kind] || LIVE.live; el.setAttribute('data-live', kind);
      var use = $('use', el); if (use) use.setAttribute('href', '#' + d[0]);
      var t = $('.live-t', el); if (t) t.textContent = d[1];
    });
  }
  CLX.setLive = setLive;
  function applyState() {
    var p = params();
    var st = p.state || document.body.getAttribute('data-default-state') || 'loaded';
    var role = p.role || 'operator';
    root.setAttribute('data-state', st); root.setAttribute('data-role', role);
    $$('[data-when]').forEach(function (el) { el.hidden = el.getAttribute('data-when').split(' ').indexOf(st) < 0; });
    $$('[data-when-not]').forEach(function (el) { el.hidden = el.getAttribute('data-when-not').split(' ').indexOf(st) >= 0; });
    $$('[data-role-only]').forEach(function (el) { el.hidden = el.getAttribute('data-role-only') !== role; });
    $$('[data-hide-role]').forEach(function (el) { el.hidden = el.getAttribute('data-hide-role') === role; });
    $$('[data-state-set]').forEach(function (b) { b.setAttribute('aria-pressed', b.getAttribute('data-state-set') === st ? 'true' : 'false'); });
    $$('[data-role-set]').forEach(function (b) { b.setAttribute('aria-pressed', b.getAttribute('data-role-set') === role ? 'true' : 'false'); });
    var map = (document.body.getAttribute('data-live-map') || '').split(',').reduce(function (a, s) { var kv = s.split(':'); if (kv[0]) a[kv[0]] = kv[1]; return a; }, {});
    if ($('.live')) setLive(map[st] || 'live');
    $$('.is-stale-target').forEach(function (el) { el.classList.toggle('is-stale', st === 'stale' || st === 'offline'); });
    var h1 = $('h1[data-state-title]'); if (h1) { var tt = h1.getAttribute('data-state-title'); }
    if (window.CLX.onState) window.CLX.onState(st, role);
  }
  window.addEventListener('hashchange', applyState);
  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-state-set]'); if (b) { setParams({ state: b.getAttribute('data-state-set') }); say('Prototype state: ' + b.textContent.trim()); return; }
    var r = e.target.closest('[data-role-set]'); if (r) { setParams({ role: r.getAttribute('data-role-set') }); say('Prototype role: ' + r.textContent.trim()); }
  });

  /* ---- dialogs ---- */
  document.addEventListener('click', function (e) {
    var o = e.target.closest('[data-open]');
    if (o) { var d = $(o.getAttribute('data-open')); if (d && d.showModal) { d.showModal(); var f = $('[autofocus]', d); if (f) f.focus(); } return; }
    var c = e.target.closest('[data-close]');
    if (c) { var dd = c.closest('dialog'); if (dd) dd.close(c.getAttribute('data-close')); return; }
    var dlg = e.target; if (dlg.tagName === 'DIALOG' && dlg.hasAttribute('open')) { var r = dlg.getBoundingClientRect(); if (e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom) dlg.close('dismiss'); }
  });
  $$('dialog.drawer a').forEach(function (a) { a.addEventListener('click', function () { var d = a.closest('dialog'); if (d) d.close(); }); });

  /* ---- copy buttons ---- */
  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-copy]'); if (!b) return;
    var sel = b.getAttribute('data-copy'); var src = sel.charAt(0) === '#' ? $(sel) : null;
    var text = src ? (src.getAttribute('data-copy-text') || src.textContent) : sel;
    var done = function () { var l = b.querySelector('.cl'); var old = l ? l.textContent : ''; if (l) l.textContent = 'Copied'; say('Copied to clipboard'); setTimeout(function () { if (l) l.textContent = old; }, 1800); };
    var fallback = function () { var ta = document.createElement('textarea'); ta.value = text; ta.setAttribute('readonly', ''); ta.style.position = 'fixed'; ta.style.opacity = '0'; document.body.appendChild(ta); ta.select(); try { document.execCommand('copy'); done(); } catch (x) { say('Copy blocked. Select the text and copy manually.'); } document.body.removeChild(ta); };
    if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(text).then(done, fallback); else fallback();
  });

  /* ---- links to routes not in this prototype set ---- */
  document.addEventListener('click', function (e) {
    var a = e.target.closest('a[data-nolink]'); if (!a) return;
    e.preventDefault(); toast('Not part of this prototype set. Route contract: docs/design/route-inventory.md (' + (a.getAttribute('data-nolink') || a.textContent.trim()) + ').');
  });

  /* ---- timestamps: local + explicit zone, UTC in title (route-inventory 1.6) ---- */
  $$('time[data-fmt]').forEach(function (t) {
    var d = new Date(t.getAttribute('datetime')); if (isNaN(d)) return;
    try {
      var loc = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false, timeZoneName: 'short' }).format(d);
      t.textContent = loc; t.title = d.toISOString().replace('.000', '') + ' (UTC)';
    } catch (x) { /* keep static UTC text */ }
  });

  /* ---- M-SECTION reveal ---- */
  var rv = $$('.reveal');
  if ('IntersectionObserver' in window && rv.length && !reduced()) {
    var io = new IntersectionObserver(function (es) { es.forEach(function (en) { if (en.isIntersecting) { en.target.classList.add('in'); io.unobserve(en.target); } }); }, { threshold: 0.2 });
    rv.forEach(function (el, i) { el.style.transitionDelay = Math.min(i % 4, 3) * 40 + 'ms'; io.observe(el); });
  } else { rv.forEach(function (el) { el.classList.add('in'); }); }

  /* ---- generic table filter: <select data-filter="status" data-table="#t"> rows carry data-status ---- */
  $$('[data-filter]').forEach(function (sel) {
    sel.addEventListener('change', function () {
      var tb = $(sel.getAttribute('data-table')); if (!tb) return;
      var rows = $$('tbody tr', tb), n = 0;
      var active = $$('[data-filter][data-table="' + sel.getAttribute('data-table') + '"]').filter(function (s) { return s.value; });
      rows.forEach(function (r) { var ok = active.every(function (s) { var v = r.getAttribute('data-' + s.getAttribute('data-filter')) || ''; return v.split(' ').indexOf(s.value) > -1; }); r.hidden = !ok; if (ok) n++; });
      var cap = $('caption .n', tb); if (cap) cap.textContent = n;
      var none = $(sel.getAttribute('data-empty') || '#filter-empty'); if (none) none.hidden = n > 0;
      say(n + (n === 1 ? ' result' : ' results'));
    });
  });

  /* ---- command search (Ctrl/Cmd+K) ---- */
  var cmd = $('#cmd');
  if (cmd) {
    var inp = $('input', cmd), list = $('#cmd-list', cmd), empty = $('#cmd-empty', cmd);
    var items = $$('li', list);
    var refresh = function () {
      var q = inp.value.trim().toLowerCase(); var n = 0;
      items.forEach(function (li) { if (li.hasAttribute('data-id-result')) return; var ok = !q || li.textContent.toLowerCase().indexOf(q) > -1; li.hidden = !ok; if (ok) n++; });
      var idr = $('[data-id-result]', list), isId = /^[0-9a-f]{8}$/.test(q);
      if (idr) { idr.hidden = !isId; if (isId) { $('a', idr).textContent = 'Open job ' + q; n++; } }
      var worker = /^wk-\d+$/.test(q);
      empty.hidden = n > 0 || !q;
      empty.textContent = (q && !isId && !worker) ? 'No job or worker with that ID. Enter an exact ID.' : '';
    };
    inp.addEventListener('input', refresh);
    document.addEventListener('keydown', function (e) { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); if (!cmd.open) cmd.showModal(); inp.focus(); } });
    cmd.addEventListener('close', function () { inp.value = ''; refresh(); });
    refresh();
  }

  /* ---- fixtures (all illustrative) ---- */
  function rng(seed) { var s = seed >>> 0; return function () { s = (s * 1664525 + 1013904223) >>> 0; return s / 4294967296; }; }
  CLX.rng = rng;
  var HANDLERS = [['thumbnail', 2], ['ocr', 1], ['export', 3], ['digest', 1], ['transcode', 1]];
  CLX.workers = function (n) {
    var r = rng(7), out = [];
    for (var i = 0; i < n; i++) {
      var cap = [2, 4, 4, 8][i % 4], h = HANDLERS[i % 5];
      var st = 'online', live = true, rev = false;
      if (n > 1 && i % 17 === 5) { st = 'stale'; live = false; }
      if (n > 1 && i % 29 === 7) { st = 'draining'; }
      if (n > 1 && i % 41 === 11) { st = 'revoked'; live = false; rev = true; }
      var active = (st === 'online' && i % 3 === 0) ? Math.min(cap, 1 + (i % 3)) : (st === 'draining' ? 1 : 0);
      var hb = live ? 2 + Math.floor(r() * 8) : 38 + Math.floor(r() * 90);
      out.push({ id: 'wk-' + (i + 1), name: 'worker-' + String(i + 1).padStart(3, '0'), handler: h[0], ver: h[1], cap: cap, active: active, st: st, live: live, rev: rev, hb: hb, inc: 'inc-' + (1000 + i * 7).toString(16) });
    }
    return out;
  };
  var STATE_LABEL = { online: 'Online', stale: 'Stale', draining: 'Draining', revoked: 'Credential revoked', off: 'Deregistered' };
  var STATE_GLYPH = { online: 'g-online', stale: 'g-stale', draining: 'g-draining', revoked: 'g-revoked', off: 'g-off' };
  CLX.STATE_LABEL = STATE_LABEL; CLX.STATE_GLYPH = STATE_GLYPH;

  /* ---- topology (console, data-backed in product; illustrative here). SVG graph with synchronized list. ---- */
  CLX.topology = function (host, workers, opts) {
    opts = opts || {};
    var NS = 'http://www.w3.org/2000/svg', n = workers.length, cluster = n > 12;
    var W, H, nodes = [];
    function el(name, attrs, parent) { var e = document.createElementNS(NS, name); for (var k in attrs) e.setAttribute(k, attrs[k]); if (parent) parent.appendChild(e); return e; }
    host.textContent = '';
    var items;
    if (cluster) {
      var groups = {}; workers.forEach(function (w) { (groups[w.handler] = groups[w.handler] || []).push(w); });
      items = Object.keys(groups).map(function (k) { var g = groups[k]; return { key: k, workers: g, label: k + '@' + g[0].ver, live: g.filter(function (w) { return w.st === 'online'; }).length, stale: g.filter(function (w) { return w.st === 'stale'; }).length, drain: g.filter(function (w) { return w.st === 'draining'; }).length, rev: g.filter(function (w) { return w.rev; }).length, act: g.reduce(function (a, w) { return a + w.active; }, 0) }; });
    } else {
      items = workers.map(function (w) { return { key: w.id, workers: [w], label: w.name, w: w }; });
    }
    var cols = Math.min(items.length, 3), rows = Math.ceil(items.length / cols);
    var nw = cluster ? 250 : 150, nh = cluster ? 78 : 54, gx = 24, gy = 22, x0 = 190;
    W = x0 + cols * (nw + gx) + 6; H = Math.max(180, 40 + rows * (nh + gy));
    var svg = el('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img', 'aria-label': opts.label || ('Worker topology: ' + n + ' workers. A table with the same entries follows.') }, host);
    var vb = { x: 0, y: 0, w: W, h: H };
    var g0 = el('g', {}, svg);
    /* coordinator hub */
    var hubY = H / 2;
    var hub = el('g', {}, g0);
    el('rect', { x: 16, y: hubY - 34, width: 120, height: 68, rx: 6, fill: 'var(--color-ink)' }, hub);
    var t1 = el('text', { x: 76, y: hubY - 4, 'text-anchor': 'middle', fill: 'var(--color-white)', 'font-size': 12, 'font-weight': 600 }, hub); t1.textContent = 'coordinator';
    var t2 = el('text', { x: 76, y: hubY + 14, 'text-anchor': 'middle', fill: 'var(--color-code-mute)', 'font-size': 10, 'font-family': 'var(--font-mono)' }, hub); t2.textContent = 'claims via gRPC';
    items.forEach(function (it, i) {
      var c = i % cols, r = Math.floor(i / cols), x = x0 + c * (nw + gx), y = 24 + r * (nh + gy);
      it.x = x; it.y = y;
      el('path', { d: 'M136 ' + hubY + ' C 170 ' + hubY + ', 160 ' + (y + nh / 2) + ', ' + x + ' ' + (y + nh / 2), fill: 'none', stroke: 'var(--color-line-strong)', 'stroke-width': 1 }, g0);
    });
    items.forEach(function (it, i) {
      var w = it.w, st = cluster ? (it.stale + it.rev === it.workers.length ? 'stale' : 'online') : w.st;
      var g = el('g', { class: 'node', tabindex: 0, role: 'button', 'data-key': it.key, 'data-live': (st === 'stale' || st === 'revoked') ? 'stale' : (st === 'off' ? 'off' : 'on'), 'aria-label': cluster ? (it.label + ' cluster: ' + it.workers.length + ' workers, ' + it.live + ' online, ' + it.stale + ' stale, ' + it.drain + ' draining, ' + it.rev + ' revoked') : (w.name + ', ' + STATE_LABEL[w.st] + ', handler ' + w.handler + ', ' + w.active + ' of ' + w.cap + ' slots active') }, g0);
      el('rect', { class: 'body', x: it.x, y: it.y, width: nw, height: nh, rx: 6 }, g);
      if (cluster) {
        var a = el('text', { x: it.x + 12, y: it.y + 22, 'font-weight': 600, 'font-size': 13 }, g); a.textContent = it.label + ' cluster';
        var b = el('text', { x: it.x + 12, y: it.y + 42, class: 'sub' }, g); b.textContent = it.workers.length + ' workers · ' + it.act + ' active attempts';
        var c2 = el('text', { x: it.x + 12, y: it.y + 60, class: 'sub' }, g); c2.textContent = it.live + ' on · ' + it.stale + ' stale · ' + it.drain + ' drain · ' + it.rev + ' revoked';
      } else {
        var u = el('use', { href: '#' + STATE_GLYPH[w.st], x: it.x + 10, y: it.y + 10, width: 16, height: 16, style: 'color:' + (w.st === 'online' ? 'var(--color-success)' : w.st === 'revoked' ? 'var(--color-danger)' : 'var(--color-warning)') }, g);
        var a2 = el('text', { x: it.x + 32, y: it.y + 22, 'font-weight': 600 }, g); a2.textContent = w.name;
        var b2 = el('text', { x: it.x + 10, y: it.y + 40, class: 'sub' }, g); b2.textContent = STATE_LABEL[w.st] + ' · ' + w.active + '/' + w.cap;
        if (w.active > 0 && w.live && n < 100) { var p = el('rect', { class: 'pulse', x: it.x - 2, y: it.y - 2, width: nw + 4, height: nh + 4, rx: 8, style: 'transform-box:fill-box;transform-origin:center' }, g); void p; }
      }
      nodes.push(g);
      g.addEventListener('click', function () { select(it.key, true); });
      g.addEventListener('keydown', function (e) {
        var idx = nodes.indexOf(g), nx = null;
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); select(it.key, true); return; }
        if (e.key === 'ArrowRight') nx = idx + 1; else if (e.key === 'ArrowLeft') nx = idx - 1; else if (e.key === 'ArrowDown') nx = idx + cols; else if (e.key === 'ArrowUp') nx = idx - cols;
        if (nx !== null && nodes[nx]) { e.preventDefault(); nodes[nx].focus(); }
      });
    });
    function apply() { svg.setAttribute('viewBox', vb.x + ' ' + vb.y + ' ' + vb.w + ' ' + vb.h); }
    CLX.topoCtl = {
      zoom: function (f) { var cx = vb.x + vb.w / 2, cy = vb.y + vb.h / 2; vb.w = Math.min(W * 2, Math.max(W / 4, vb.w * f)); vb.h = vb.w * H / W; vb.x = cx - vb.w / 2; vb.y = cy - vb.h / 2; apply(); say('Zoom ' + Math.round(W / vb.w * 100) + ' percent'); },
      pan: function (dx, dy) { vb.x += dx * vb.w; vb.y += dy * vb.h; apply(); },
      reset: function () { vb = { x: 0, y: 0, w: W, h: H }; apply(); say('View reset'); }
    };
    function select(key, fromGraph) {
      nodes.forEach(function (g) { g.setAttribute('data-sel', g.getAttribute('data-key') === key ? '1' : '0'); });
      if (opts.onSelect) opts.onSelect(key, fromGraph, items);
    }
    CLX.topoSelect = select;
    return { items: items, select: select, svg: svg };
  };
  document.addEventListener('click', function (e) {
    var z = e.target.closest('[data-topo]'); if (!z || !CLX.topoCtl) return;
    var a = z.getAttribute('data-topo');
    if (a === 'in') CLX.topoCtl.zoom(0.8); else if (a === 'out') CLX.topoCtl.zoom(1.25); else if (a === 'reset') CLX.topoCtl.reset();
    else if (a === 'left') CLX.topoCtl.pan(-0.15, 0); else if (a === 'right') CLX.topoCtl.pan(0.15, 0); else if (a === 'up') CLX.topoCtl.pan(0, -0.15); else if (a === 'down') CLX.topoCtl.pan(0, 0.15);
  });

  /* ---- tabs/segment toggles: [data-toggle-group] buttons show panels by id ---- */
  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-panel]'); if (!b) return;
    var grp = b.closest('[data-panel-group]'); if (!grp) return;
    $$('[data-panel]', grp).forEach(function (x) { x.setAttribute('aria-pressed', x === b ? 'true' : 'false'); var p = $(x.getAttribute('data-panel')); if (p) p.hidden = x !== b; });
    say(b.textContent.trim() + ' view');
  });

  applyState();
})();
