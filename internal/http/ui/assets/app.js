/* Links dashboard — vanilla JS, no build step, embedded in the Go binary. */
(() => {
    'use strict';

    // ======================================================================
    // DOM helpers & icons
    // ======================================================================
    const $ = (sel, root = document) => root.querySelector(sel);
    const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

    // h() builds elements without innerHTML so user content is never parsed as HTML.
    function h(tag, props, ...children) {
        const el = document.createElement(tag);
        if (props) {
            for (const [k, v] of Object.entries(props)) {
                if (v === null || v === undefined || v === false) continue;
                if (k === 'class') el.className = v;
                else if (k === 'style') el.style.cssText = v;
                else if (k === 'dataset') Object.assign(el.dataset, v);
                else if (k === 'svg') el.innerHTML = v; // trusted static icon markup only
                else if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
                else if (k === 'value' || k === 'checked' || k === 'disabled' || k === 'draggable' || k === 'hidden') el[k] = v;
                else if (v === true) el.setAttribute(k, '');
                else el.setAttribute(k, String(v));
            }
        }
        for (const c of children.flat()) {
            if (c === null || c === undefined || c === false) continue;
            el.append(c instanceof Node ? c : document.createTextNode(String(c)));
        }
        return el;
    }

    const svgWrap = (body) => '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' + body + '</svg>';
    const ICONS = {
        plus: '<path d="M12 5v14M5 12h14"/>',
        search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
        settings: '<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
        sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
        moon: '<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"/>',
        edit: '<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/>',
        copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
        trash: '<path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>',
        x: '<path d="M18 6 6 18M6 6l12 12"/>',
        chevron: '<path d="m6 9 6 6 6-6"/>',
        pause: '<rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/>',
        play: '<path d="M7 4v16l13-8Z"/>',
        download: '<path d="M12 3v12M7 10l5 5 5-5M5 21h14"/>',
        upload: '<path d="M12 21V9M7 14l5-5 5 5M5 3h14"/>',
        link: '<path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7"/><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7"/>',
    };
    const icon = (name, cls) => h('span', { class: 'ico' + (cls ? ' ' + cls : ''), 'aria-hidden': 'true', svg: svgWrap(ICONS[name] || '') });
    const setIcon = (el, name) => { if (el) el.innerHTML = svgWrap(ICONS[name] || ''); };

    const setText = (id, text) => {
        const el = typeof id === 'string' ? document.getElementById(id) : id;
        if (el && el.textContent !== String(text)) el.textContent = text;
    };
    const debounce = (fn, ms) => { let t; return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); }; };
    const last = (arr) => arr[arr.length - 1];
    const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

    // ======================================================================
    // State & preferences
    // ======================================================================
    let initial = {};
    try { initial = JSON.parse($('#initial-state').textContent || '{}') || {}; } catch (e) { console.error(e); }

    const prefs = {
        get(key, fallback) {
            try {
                const v = localStorage.getItem('links.' + key);
                return v === null ? fallback : JSON.parse(v);
            } catch { return fallback; }
        },
        set(key, value) {
            try { localStorage.setItem('links.' + key, JSON.stringify(value)); } catch { /* private mode */ }
        },
    };

    const state = {
        links: Array.isArray(initial.links) ? initial.links : [],
        settings: initial.settings || {},
        serverIp: initial.hostIp || '',
        detectedIp: '',
        hostIps: [],
        hostname: initial.hostname || '',
        statuses: {},
        search: '',
        collapsed: new Set(prefs.get('collapsed', [])),
        res: null,
    };

    // ======================================================================
    // API
    // ======================================================================
    async function api(method, path, body, raw) {
        const opts = { method, headers: {} };
        if (body !== undefined) {
            opts.headers['Content-Type'] = 'application/json';
            opts.body = raw ? body : JSON.stringify(body);
        }
        const res = await fetch(path, opts);
        const text = await res.text();
        let data = null;
        if (text) { try { data = JSON.parse(text); } catch { data = text; } }
        if (!res.ok) {
            const msg = (data && data.error) || (typeof data === 'string' && data.trim()) || res.statusText || 'Request failed';
            throw new Error(msg.replace(/^invalid input: /, ''));
        }
        return data;
    }

    const linkPayload = (l) => ({ title: l.title, url: l.url, group: l.group || '', description: l.description || '', icon: l.icon || '' });

    async function reloadLinks() {
        try { state.links = await api('GET', '/api/links'); } catch (e) { console.error(e); }
        renderLinks();
    }

    // ======================================================================
    // URL helpers — mirror internal/domain/url.go (SplitURL)
    // ======================================================================
    function splitUrl(raw) {
        const m = /^([a-z][a-z0-9+.-]*:\/\/)?([^/?#]*)(.*)$/is.exec(String(raw || ''));
        const p = { scheme: m[1] || '', userinfo: '', host: '', port: '', rest: m[3] || '' };
        let auth = m[2] || '';
        const at = auth.lastIndexOf('@');
        if (at >= 0) { p.userinfo = auth.slice(0, at + 1); auth = auth.slice(at + 1); }
        if (auth.startsWith('[') && auth.includes(']')) {
            const end = auth.indexOf(']');
            p.host = auth.slice(0, end + 1);
            const after = auth.slice(end + 1);
            if (after.startsWith(':')) p.port = after.slice(1);
            return p;
        }
        const c = auth.lastIndexOf(':');
        if (c >= 0) { p.host = auth.slice(0, c); p.port = auth.slice(c + 1); } else { p.host = auth; }
        return p;
    }
    const joinUrl = (p) => p.scheme + p.userinfo + p.host + (p.port ? ':' + p.port : '') + p.rest;

    const effectiveIp = () => state.serverIp || location.hostname;
    const resolveUrl = (raw) => String(raw || '').split('{ip}').join(effectiveIp()).split('{host}').join(location.hostname);
    const hasPlaceholder = (raw) => /\{(ip|host)\}/.test(raw || '');
    const safeHref = (u) => (/^\s*(javascript|data|vbscript):/i.test(u) ? '#' : u);

    function displayUrl(raw) {
        const p = splitUrl(resolveUrl(raw));
        if (!p.host && !p.scheme) return resolveUrl(raw);
        const rest = p.rest === '/' ? '' : p.rest;
        return p.host + (p.port ? ':' + p.port : '') + rest;
    }

    function faviconUrl(raw) {
        try {
            const u = new URL(resolveUrl(raw));
            if (u.protocol !== 'http:' && u.protocol !== 'https:') return null;
            return u.origin + '/favicon.ico';
        } catch { return null; }
    }

    const hueOf = (s) => { let x = 0; for (const ch of String(s)) x = (x * 31 + ch.codePointAt(0)) >>> 0; return x % 360; };

    // ======================================================================
    // Toasts, clipboard, ask dialog
    // ======================================================================
    function toast(message, opts = {}) {
        const { type = 'ok', action = null, duration = 3500 } = opts;
        const box = $('#toasts');
        const el = h('div', { class: 'toast ' + type, role: type === 'error' ? 'alert' : 'status' }, h('span', { class: 'toast-msg' }, message));
        let timer = null;
        const dismiss = () => {
            clearTimeout(timer);
            if (!el.isConnected) return;
            el.classList.add('out');
            setTimeout(() => el.remove(), 200);
        };
        if (action) {
            el.append(h('button', { type: 'button', class: 'btn btn-sm', onclick: () => { dismiss(); action.fn(); } }, action.label));
        }
        el.addEventListener('mouseenter', () => clearTimeout(timer));
        el.addEventListener('mouseleave', () => { timer = setTimeout(dismiss, 1500); });
        box.append(el);
        while (box.children.length > 3) box.firstElementChild.remove();
        timer = setTimeout(dismiss, duration);
    }
    const toastError = (err) => toast(err && err.message ? err.message : String(err), { type: 'error', duration: 6000 });

    // navigator.clipboard only works on https/localhost; dashboards are usually
    // opened over plain http on the LAN, so fall back to execCommand.
    async function copyText(text) {
        try {
            if (navigator.clipboard && window.isSecureContext) {
                await navigator.clipboard.writeText(text);
                return true;
            }
        } catch { /* fall through */ }
        const ta = h('textarea', { style: 'position:fixed;top:-1000px;left:0;opacity:0', readonly: true });
        ta.value = text;
        document.body.append(ta);
        ta.select();
        let ok = false;
        try { ok = document.execCommand('copy'); } catch { ok = false; }
        ta.remove();
        return ok;
    }

    const askDialog = $('#askDialog');
    let askResolve = null;
    function ask({ title, text = '', label = '', value = '', placeholder = '', okLabel = 'OK', danger = false, input = true }) {
        setText('askTitle', title);
        setText('askText', text);
        setText('askLabel', label);
        $('#askField').hidden = !input;
        const inp = $('#askInput');
        inp.value = value;
        inp.placeholder = placeholder;
        const ok = $('#askOk');
        ok.textContent = okLabel;
        ok.className = 'btn ' + (danger ? 'btn-danger' : 'btn-primary');
        return new Promise((resolve) => {
            askResolve = resolve;
            askDialog.showModal();
            if (input) { inp.focus(); inp.select(); } else { ok.focus(); }
        });
    }
    $('#askForm').addEventListener('submit', (e) => {
        e.preventDefault();
        const r = askResolve;
        askResolve = null;
        const value = $('#askField').hidden ? true : $('#askInput').value;
        askDialog.close();
        if (r) r(value);
    });
    askDialog.addEventListener('close', () => {
        if (askResolve) { const r = askResolve; askResolve = null; r(null); }
    });

    // Close any dialog via [data-close] or a click on its backdrop.
    for (const dlg of $$('dialog.modal')) {
        let downOnBackdrop = false;
        dlg.addEventListener('pointerdown', (e) => { downOnBackdrop = e.target === dlg; });
        dlg.addEventListener('click', (e) => {
            if (e.target === dlg && downOnBackdrop) dlg.close();
            const closer = e.target.closest('[data-close]');
            if (closer && closer.closest('dialog') === dlg) dlg.close();
        });
    }

    // ======================================================================
    // Theme
    // ======================================================================
    const lightQuery = window.matchMedia ? window.matchMedia('(prefers-color-scheme: light)') : null;
    let themeColors = null;

    function applyTheme() {
        const pref = prefs.get('theme', 'auto');
        const theme = pref === 'auto' ? (lightQuery && lightQuery.matches ? 'light' : 'dark') : pref;
        document.documentElement.dataset.theme = theme;
        themeColors = null;
        const btn = $('#themeBtn');
        btn.replaceChildren(icon(theme === 'dark' ? 'sun' : 'moon'));
        btn.title = theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme';
        drawAllCharts();
    }
    if (lightQuery && lightQuery.addEventListener) lightQuery.addEventListener('change', applyTheme);
    $('#themeBtn').addEventListener('click', () => {
        prefs.set('theme', document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark');
        applyTheme();
    });

    function colors() {
        if (themeColors) return themeColors;
        const cs = getComputedStyle(document.documentElement);
        const v = (name) => cs.getPropertyValue(name).trim();
        themeColors = {
            grid: v('--chart-grid'), text: v('--chart-text'), border: v('--border-strong'), surface: v('--surface-pop'),
            cpu: v('--accent'), mem: v('--accent-2'), ok: v('--ok'), warn: v('--warn'), crit: v('--crit'),
        };
        return themeColors;
    }

    function withAlpha(color, a) {
        const m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(color || '');
        if (!m) return color;
        let hex = m[1];
        if (hex.length === 3) hex = hex.split('').map((c) => c + c).join('');
        const n = parseInt(hex, 16);
        return 'rgba(' + (n >> 16 & 255) + ',' + (n >> 8 & 255) + ',' + (n & 255) + ',' + a + ')';
    }

    // ======================================================================
    // Header
    // ======================================================================
    function renderHeader() {
        const title = state.settings.title || 'Links';
        setText('pageTitle', title);
        if (document.title !== title) document.title = title;
        setText('hostName', state.hostname);
        const ipEl = $('#hostIp');
        ipEl.replaceChildren(state.serverIp || '–');
        if (state.res && state.res.hostIpOverride) ipEl.append(h('span', { class: 'ip-override', title: 'Set manually in Settings' }, 'manual'));
    }

    $('#ipCopyBtn').addEventListener('click', async () => {
        if (!state.serverIp) return;
        const ok = await copyText(state.serverIp);
        toast(ok ? 'Copied ' + state.serverIp : 'Copy failed', { type: ok ? 'ok' : 'error' });
    });
    $('#ipEditBtn').addEventListener('click', () => openSettings('ip'));

    // ======================================================================
    // Links
    // ======================================================================
    const UNGROUPED = '';
    const groupLabel = (g) => g || 'Ungrouped';

    function linkMatches(l, terms) {
        if (!terms.length) return true;
        const hay = [l.title, resolveUrl(l.url), l.url, l.group, l.description].join(' ').toLowerCase();
        return terms.every((t) => hay.includes(t));
    }

    function highlight(text, terms) {
        const s = String(text || '');
        if (!terms.length || !s) return s;
        const lower = s.toLowerCase();
        const marks = [];
        for (const t of terms) {
            let i = lower.indexOf(t);
            while (t && i !== -1) { marks.push([i, i + t.length]); i = lower.indexOf(t, i + t.length); }
        }
        if (!marks.length) return s;
        marks.sort((a, b) => a[0] - b[0]);
        const frag = document.createDocumentFragment();
        let pos = 0;
        for (const [a, b] of marks) {
            if (a < pos) continue;
            frag.append(s.slice(pos, a), h('mark', null, s.slice(a, b)));
            pos = b;
        }
        frag.append(s.slice(pos));
        return frag;
    }

    function groupsOf(links) {
        const order = [];
        const map = new Map();
        for (const l of links) {
            const g = l.group || UNGROUPED;
            if (!map.has(g)) { map.set(g, []); order.push(g); }
            map.get(g).push(l);
        }
        return order.map((name) => ({ name, links: map.get(name) }));
    }

    function statusInfo(st) {
        if (!st) return { s: '', text: prefs.get('statusChecks', true) ? 'Checking…' : 'Status checks are off' };
        if (!st.up) return { s: 'down', text: 'Offline · ' + (st.error || 'no response') };
        const ms = st.ms + ' ms';
        if (st.code >= 500) return { s: 'warn', text: 'HTTP ' + st.code + ' · ' + ms };
        return { s: 'up', text: (st.code ? 'Online · HTTP ' + st.code : 'Online') + ' · ' + ms };
    }

    function tileIcon(l) {
        const custom = (l.icon || '').trim();
        const box = h('span', { class: 'tile-icon', style: '--h:' + hueOf(l.title) });
        const addImg = (src, fallbackLetter) => {
            box.classList.add('has-img');
            const img = h('img', { src, alt: '', loading: 'lazy', decoding: 'async', referrerpolicy: 'no-referrer', draggable: false });
            img.addEventListener('load', () => { if (img.naturalWidth > 1) img.classList.add('loaded'); else img.remove(); });
            img.addEventListener('error', () => { img.remove(); box.classList.remove('has-img'); });
            box.append(fallbackLetter, img);
        };
        const letter = (Array.from(l.title || '?')[0] || '?');
        if (/^(https?:\/\/|\/)/i.test(custom) || hasPlaceholder(custom)) {
            addImg(resolveUrl(custom), letter);
        } else if (custom) {
            box.classList.add('emoji');
            box.textContent = Array.from(custom).slice(0, 2).join('');
        } else {
            const fav = faviconUrl(l.url);
            if (fav) addImg(fav, letter); else box.textContent = letter;
        }
        return box;
    }

    let dragId = null;
    function clearDropMarks() {
        for (const el of $$('.drop-before, .drop-after, .drop-into')) el.classList.remove('drop-before', 'drop-after', 'drop-into');
    }

    function renderTile(l, terms, i) {
        const resolved = resolveUrl(l.url);
        const st = statusInfo(state.statuses[l.id]);
        const newTab = prefs.get('newTab', true);

        const urlLine = h('span', { class: 'tile-url' },
            h('span', { class: 'status-dot', dataset: { s: st.s }, title: st.text }),
            h('span', { class: 'mono' }, highlight(displayUrl(l.url), terms)),
            hasPlaceholder(l.url) ? h('span', { class: 'ph-badge', title: 'Follows the server IP: ' + l.url }, l.url.includes('{ip}') ? 'ip' : 'host') : null,
        );

        const a = h('a', {
            class: 'tile-link',
            href: safeHref(resolved),
            target: newTab ? '_blank' : null,
            rel: 'noopener noreferrer',
            title: l.title + '\n' + resolved,
            draggable: false,
        },
        tileIcon(l),
        h('span', { class: 'tile-body' },
            h('span', { class: 'tile-title' }, highlight(l.title, terms)),
            urlLine,
            l.description ? h('span', { class: 'tile-desc' }, highlight(l.description, terms)) : null,
        ));

        const actions = h('div', { class: 'tile-actions' },
            h('button', { type: 'button', class: 'icon-btn edit', title: 'Edit', 'aria-label': 'Edit ' + l.title, onclick: () => openLinkDialog(l) }, icon('edit')),
            h('button', {
                type: 'button', class: 'icon-btn', title: 'Copy URL', 'aria-label': 'Copy URL of ' + l.title,
                onclick: async () => { const ok = await copyText(resolved); toast(ok ? 'Copied URL' : 'Copy failed', { type: ok ? 'ok' : 'error' }); },
            }, icon('copy')),
            h('button', { type: 'button', class: 'icon-btn danger', title: 'Delete', 'aria-label': 'Delete ' + l.title, onclick: () => deleteLink(l) }, icon('trash')),
        );

        const li = h('li', { class: 'tile', dataset: { id: l.id }, style: '--i:' + Math.min(i, 30) }, a, actions);

        if (!terms.length) {
            li.draggable = true;
            li.addEventListener('dragstart', (e) => {
                dragId = l.id;
                li.classList.add('dragging');
                e.dataTransfer.effectAllowed = 'move';
                try { e.dataTransfer.setData('text/plain', l.title); } catch { /* ignore */ }
            });
            li.addEventListener('dragend', () => { dragId = null; li.classList.remove('dragging'); clearDropMarks(); });
            li.addEventListener('dragover', (e) => {
                if (!dragId || dragId === l.id) return;
                e.preventDefault();
                e.stopPropagation();
                e.dataTransfer.dropEffect = 'move';
                const r = li.getBoundingClientRect();
                const after = e.clientX - r.left > r.width / 2;
                if (!li.classList.contains(after ? 'drop-after' : 'drop-before')) {
                    clearDropMarks();
                    li.classList.add(after ? 'drop-after' : 'drop-before');
                }
            });
            li.addEventListener('drop', (e) => {
                if (!dragId) return;
                e.preventDefault();
                e.stopPropagation();
                const after = li.classList.contains('drop-after');
                const id = dragId;
                clearDropMarks();
                if (id !== l.id) moveLink(id, after ? { afterId: l.id } : { beforeId: l.id });
            });
        }
        return li;
    }

    function renderLinks() {
        const root = $('#links');
        const terms = state.search.trim().toLowerCase().split(/\s+/).filter(Boolean);
        root.replaceChildren();

        if (state.links.length === 0) {
            root.append(h('div', { class: 'empty' },
                h('div', { class: 'empty-art' }, icon('link')),
                h('h2', null, 'No links yet'),
                h('p', null, 'Add the services you run — Jellyfin, Home Assistant, your router. Use ', h('code', null, '{ip}'), ' in a URL and it will follow this server\'s IP.'),
                h('button', { type: 'button', class: 'btn btn-primary', onclick: () => openLinkDialog() }, icon('plus'), 'Add your first link'),
            ));
            return;
        }

        const visible = state.links.filter((l) => linkMatches(l, terms));
        if (visible.length === 0) {
            root.append(h('div', { class: 'empty' },
                h('h2', null, 'Nothing matches “' + state.search.trim() + '”'),
                h('p', null, 'Press Esc to clear the search.'),
            ));
            return;
        }

        const groups = groupsOf(visible);
        const showHeads = groups.length > 1 || groups[0].name !== UNGROUPED;
        let i = 0;
        for (const g of groups) {
            const collapsed = !terms.length && state.collapsed.has(g.name);
            const section = h('section', { class: 'group' + (collapsed ? ' collapsed' : ''), dataset: { group: g.name } });
            if (showHeads) {
                section.append(h('div', { class: 'group-head' },
                    h('button', {
                        type: 'button', class: 'group-toggle', 'aria-expanded': String(!collapsed),
                        onclick: () => toggleGroup(g.name),
                    }, icon('chevron'), groupLabel(g.name), h('span', { class: 'group-count' }, g.links.length)),
                    h('span', { class: 'group-line' }),
                    g.name ? h('button', { type: 'button', class: 'icon-btn group-rename', title: 'Rename group', 'aria-label': 'Rename group ' + g.name, onclick: () => renameGroup(g.name) }, icon('edit')) : null,
                ));
            }
            const ul = h('ul', { class: 'tiles' });
            for (const l of g.links) ul.append(renderTile(l, terms, i++));
            section.append(ul);

            if (!terms.length) {
                section.addEventListener('dragover', (e) => {
                    if (!dragId) return;
                    e.preventDefault();
                    if (!section.classList.contains('drop-into')) { clearDropMarks(); section.classList.add('drop-into'); }
                });
                section.addEventListener('dragleave', (e) => {
                    if (!section.contains(e.relatedTarget)) section.classList.remove('drop-into');
                });
                section.addEventListener('drop', (e) => {
                    if (!dragId) return;
                    e.preventDefault();
                    const id = dragId;
                    clearDropMarks();
                    moveLink(id, { group: g.name });
                });
            }
            root.append(section);
        }

        const options = $('#groupOptions');
        options.replaceChildren(...[...new Set(state.links.map((l) => l.group).filter(Boolean))].map((g) => h('option', { value: g })));
    }

    function updateStatusDots() {
        for (const li of $$('#links .tile')) {
            const st = statusInfo(state.statuses[li.dataset.id]);
            const dot = $('.status-dot', li);
            if (dot) { dot.dataset.s = st.s; dot.title = st.text; }
        }
    }

    function toggleGroup(name) {
        if (state.collapsed.has(name)) state.collapsed.delete(name); else state.collapsed.add(name);
        prefs.set('collapsed', [...state.collapsed]);
        renderLinks();
    }

    async function renameGroup(name) {
        const to = await ask({ title: 'Rename group', label: 'Group name', value: name, okLabel: 'Rename', placeholder: 'Leave empty to ungroup' });
        if (to === null || to.trim() === name) return;
        try {
            const res = await api('POST', '/api/groups/rename', { from: name, to: to.trim() });
            state.links = res.links;
            if (state.collapsed.delete(name) && to.trim()) state.collapsed.add(to.trim());
            prefs.set('collapsed', [...state.collapsed]);
            renderLinks();
            toast(to.trim() ? 'Renamed to “' + to.trim() + '”' : 'Ungrouped ' + res.updated + ' links');
        } catch (err) { toastError(err); }
    }

    async function moveLink(id, target) {
        const before = state.links.map((l) => l.id).join(',');
        const from = state.links.findIndex((l) => l.id === id);
        if (from < 0) return;
        const [link] = state.links.splice(from, 1);
        const prevGroup = link.group || UNGROUPED;
        let to;
        let group;
        const refId = target.beforeId || target.afterId;
        if (refId) {
            const ref = state.links.findIndex((l) => l.id === refId);
            if (ref < 0) { state.links.splice(from, 0, link); return; }
            to = ref + (target.afterId ? 1 : 0);
            group = state.links[ref].group || UNGROUPED;
        } else {
            group = target.group || UNGROUPED;
            let lastIdx = -1;
            state.links.forEach((l, i) => { if ((l.group || UNGROUPED) === group) lastIdx = i; });
            to = lastIdx >= 0 ? lastIdx + 1 : state.links.length;
        }
        const moved = { ...link, group };
        state.links.splice(to, 0, moved);
        if (state.links.map((l) => l.id).join(',') === before && prevGroup === group) return;
        renderLinks();
        try {
            if (prevGroup !== group) await api('PUT', '/api/links/' + encodeURIComponent(id), linkPayload(moved));
            state.links = await api('PUT', '/api/links/order', { ids: state.links.map((l) => l.id) });
            if (prevGroup !== group) toast('Moved to ' + groupLabel(group));
        } catch (err) {
            toastError(err);
            await reloadLinks();
        }
    }

    async function deleteLink(link) {
        const idx = state.links.findIndex((l) => l.id === link.id);
        try {
            await api('DELETE', '/api/links/' + encodeURIComponent(link.id));
        } catch (err) { toastError(err); return; }
        state.links = state.links.filter((l) => l.id !== link.id);
        delete state.statuses[link.id];
        renderLinks();
        toast('Deleted “' + link.title + '”', {
            duration: 7000,
            action: {
                label: 'Undo',
                fn: async () => {
                    try {
                        const created = await api('POST', '/api/links?at=' + Math.max(0, idx), linkPayload(link));
                        state.links.splice(clamp(idx, 0, state.links.length), 0, created);
                        renderLinks();
                        refreshStatusesSoon();
                    } catch (err) { toastError(err); }
                },
            },
        });
    }

    // ---------- search ----------
    const searchInput = $('#search');
    searchInput.addEventListener('input', () => { state.search = searchInput.value; renderLinks(); });
    searchInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            const first = $('#links .tile-link');
            if (first) { e.preventDefault(); first.click(); }
        } else if (e.key === 'Escape') {
            if (searchInput.value) { searchInput.value = ''; state.search = ''; renderLinks(); } else { searchInput.blur(); }
            e.stopPropagation();
        }
    });

    // ======================================================================
    // Link dialog (add / edit, with host & port editing)
    // ======================================================================
    const linkDialog = $('#linkDialog');
    const linkForm = $('#linkForm');
    const f = (name) => linkForm.elements.namedItem(name);
    let editingId = null;

    function openLinkDialog(link) {
        editingId = link ? link.id : null;
        setText('linkDialogTitle', link ? 'Edit link' : 'Add link');
        $('#linkSaveBtn').textContent = link ? 'Save changes' : 'Add link';
        $('#linkDeleteBtn').hidden = !link;
        $('#linkFormError').hidden = true;
        for (const name of ['title', 'url', 'group', 'description', 'icon']) {
            f(name).value = link ? (link[name] || '') : '';
            f(name).removeAttribute('aria-invalid');
        }
        if (!link) {
            const p = splitUrl('');
            p.scheme = 'http://';
            p.host = state.serverIp ? '{ip}' : '';
            f('url').value = p.host ? joinUrl(p) : '';
        }
        syncPartsFromUrl();
        linkDialog.showModal();
        f('title').focus();
    }

    function syncPartsFromUrl() {
        const p = splitUrl(f('url').value.trim());
        f('host').value = p.host;
        f('port').value = p.port;
        renderHostHelpers();
    }

    function syncUrlFromParts() {
        const p = splitUrl(f('url').value.trim());
        if (!p.scheme && (f('host').value || f('port').value)) p.scheme = 'http://';
        p.host = f('host').value.trim();
        p.port = f('port').value.replace(/[^0-9]/g, '');
        if (f('port').value !== p.port) f('port').value = p.port;
        f('url').value = joinUrl(p);
        renderHostHelpers();
    }

    function renderHostHelpers() {
        const host = f('host').value.trim();
        const chips = [];
        const chip = (value, label, detail) => h('button', {
            type: 'button',
            class: 'chip-btn' + (host.toLowerCase() === value.toLowerCase() ? ' active' : ''),
            title: 'Set host to ' + value,
            onclick: () => { f('host').value = value; syncUrlFromParts(); f('host').focus(); },
        }, label, detail ? h('span', { class: 'mono' }, detail) : null);

        chips.push(chip('{ip}', 'Server IP', '{ip} → ' + effectiveIp()));
        if (location.hostname && location.hostname !== effectiveIp()) chips.push(chip('{host}', 'This hostname', '{host} → ' + location.hostname));
        for (const c of state.hostIps.slice(0, 4)) {
            if (c.ip !== host) chips.push(chip(c.ip, c.ip, c.interface));
        }
        $('#hostChips').replaceChildren(...chips);

        const url = f('url').value.trim();
        const preview = $('#urlPreview');
        preview.replaceChildren();
        if (!url) return;
        if (hasPlaceholder(url)) {
            const full = /^[a-z][a-z0-9+.-]*:\/\//i.test(url) ? url : 'http://' + url;
            preview.append('Opens ', h('code', null, resolveUrl(full)), ' — updates automatically when the server IP changes.');
        } else if (host && (host === state.serverIp || state.hostIps.some((c) => c.ip === host))) {
            preview.append('Tip: use ', h('code', null, '{ip}'), ' instead of ', h('code', null, host), ' so this link keeps working if the server gets a new IP.');
        } else if (!/^[a-z][a-z0-9+.-]*:/i.test(url)) {
            preview.append('Will be saved as ', h('code', null, 'http://' + url));
        }
    }

    f('url').addEventListener('input', syncPartsFromUrl);
    f('host').addEventListener('input', syncUrlFromParts);
    f('port').addEventListener('input', syncUrlFromParts);
    for (const name of ['title', 'url']) f(name).addEventListener('input', () => f(name).removeAttribute('aria-invalid'));

    linkForm.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); linkForm.requestSubmit(); }
    });

    linkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const errBox = $('#linkFormError');
        errBox.hidden = true;
        const payload = {
            title: f('title').value.trim(),
            url: f('url').value.trim(),
            group: f('group').value.trim(),
            description: f('description').value.trim(),
            icon: f('icon').value.trim(),
        };
        const missing = ['title', 'url'].filter((k) => !payload[k]);
        if (missing.length) {
            for (const k of missing) f(k).setAttribute('aria-invalid', 'true');
            f(missing[0]).focus();
            errBox.textContent = missing.map((k) => k[0].toUpperCase() + k.slice(1)).join(' and ') + (missing.length > 1 ? ' are' : ' is') + ' required.';
            errBox.hidden = false;
            return;
        }
        const btn = $('#linkSaveBtn');
        btn.disabled = true;
        try {
            if (editingId) {
                const updated = await api('PUT', '/api/links/' + encodeURIComponent(editingId), payload);
                state.links = state.links.map((l) => (l.id === editingId ? updated : l));
                delete state.statuses[editingId];
                toast('Saved “' + updated.title + '”');
            } else {
                const created = await api('POST', '/api/links', payload);
                state.links.push(created);
                toast('Added “' + created.title + '”');
            }
            linkDialog.close();
            renderLinks();
            refreshStatusesSoon();
        } catch (err) {
            errBox.textContent = err.message;
            errBox.hidden = false;
        } finally {
            btn.disabled = false;
        }
    });

    $('#linkDeleteBtn').addEventListener('click', () => {
        const link = state.links.find((l) => l.id === editingId);
        linkDialog.close();
        if (link) deleteLink(link);
    });
    $('#addBtn').addEventListener('click', () => openLinkDialog());

    // ======================================================================
    // Settings dialog
    // ======================================================================
    const settingsDialog = $('#settingsDialog');

    function openSettings(focus) {
        renderSettings();
        settingsDialog.showModal();
        const target = focus === 'ip' ? $('#ipSelect') : null;
        if (target) target.focus();
    }
    $('#settingsBtn').addEventListener('click', () => openSettings());

    async function saveSettings(patch) {
        const saved = await api('PUT', '/api/settings', { ...state.settings, ...patch });
        state.settings = saved;
        return saved;
    }

    function hostUsage() {
        const map = new Map();
        for (const l of state.links) {
            const host = splitUrl(l.url).host;
            if (!host) continue;
            const key = host.toLowerCase();
            const e = map.get(key) || { host, count: 0 };
            e.count++;
            map.set(key, e);
        }
        return [...map.values()].sort((a, b) => b.count - a.count || a.host.localeCompare(b.host));
    }

    async function replaceHost(from, to) {
        const res = await api('POST', '/api/links/replace-host', { from, to });
        state.links = res.links;
        state.statuses = {};
        renderLinks();
        refreshStatusesSoon();
        toast('Updated ' + res.updated + (res.updated === 1 ? ' link' : ' links') + ': ' + from + ' → ' + to);
        return res;
    }

    function renderSettings() {
        const body = $('#settingsBody');
        const detected = state.detectedIp;
        const cands = state.hostIps;
        const current = state.settings.hostIp || '';

        // --- server IP ---
        const ipSelect = h('select', { id: 'ipSelect', class: 'input' },
            h('option', { value: '' }, 'Auto-detect' + (detected ? ' · ' + detected : '')),
            cands.map((c) => h('option', { value: c.ip }, c.ip + ' · ' + c.interface)),
            h('option', { value: '__custom' }, 'Custom IP or hostname…'),
        );
        const knownCurrent = current === '' || cands.some((c) => c.ip === current);
        ipSelect.value = knownCurrent ? current : '__custom';
        const ipCustom = h('input', { class: 'input mono', placeholder: '192.168.1.20 or pi.local', value: knownCurrent ? '' : current, spellcheck: 'false' });
        const customField = h('label', { class: 'field', hidden: ipSelect.value !== '__custom' }, h('span', { class: 'field-label' }, 'Custom value'), ipCustom);
        ipSelect.addEventListener('change', () => { customField.hidden = ipSelect.value !== '__custom'; if (!customField.hidden) ipCustom.focus(); });

        const saveIp = async () => {
            const value = ipSelect.value === '__custom' ? ipCustom.value.trim() : ipSelect.value;
            const oldIp = state.serverIp;
            try {
                const saved = await saveSettings({ hostIp: value });
                state.serverIp = saved.hostIp || state.detectedIp || state.serverIp;
                if (state.res) state.res.hostIpOverride = !!saved.hostIp;
                renderHeader();
                renderLinks();
                renderSettings();
                refreshStatusesSoon();
                const literal = state.links.filter((l) => splitUrl(l.url).host.toLowerCase() === (oldIp || '').toLowerCase()).length;
                if (oldIp && oldIp !== state.serverIp && literal > 0) {
                    toast('Server IP is now ' + state.serverIp + '. ' + literal + (literal === 1 ? ' link still points' : ' links still point') + ' to ' + oldIp + '.', {
                        duration: 10000,
                        action: { label: 'Update them', fn: () => replaceHost(oldIp, state.serverIp).then(renderSettings).catch(toastError) },
                    });
                } else {
                    toast('Server IP: ' + state.serverIp + (saved.hostIp ? ' (manual)' : ' (auto)'));
                }
            } catch (err) { toastError(err); }
        };

        const ipSection = h('section', { class: 'set-section' },
            h('h4', null, 'Server IP'),
            h('p', null, 'Shown in the header and used wherever a link contains ', h('code', null, '{ip}'), '. Auto-detect picks the most likely LAN address; choose another if it guessed wrong.'),
            h('div', { class: 'set-row' },
                h('label', { class: 'field' }, h('span', { class: 'field-label' }, 'Address'), ipSelect),
                h('button', { type: 'button', class: 'btn btn-primary', onclick: saveIp }, 'Save'),
            ),
            customField,
        );
        ipCustom.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); saveIp(); } });

        // --- hosts used in links ---
        const usage = hostUsage();
        const hostRows = usage.map((u) => {
            const input = h('input', { class: 'input input-sm mono', placeholder: 'New IP or host', spellcheck: 'false', 'aria-label': 'Replace ' + u.host + ' with' });
            const apply = async () => {
                const to = input.value.trim();
                if (!to) { input.focus(); return; }
                try { await replaceHost(u.host, to); renderSettings(); } catch (err) { toastError(err); }
            };
            input.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); apply(); } });
            const isServer = u.host !== '{ip}' && (u.host === state.serverIp || cands.some((c) => c.ip === u.host));
            return h('div', { class: 'host-row' },
                h('span', { class: 'host-name', title: u.host }, u.host, u.host === '{ip}' ? h('span', { class: 'muted' }, ' → ' + effectiveIp()) : null),
                h('span', { class: 'host-count' }, u.count + (u.count === 1 ? ' link' : ' links')),
                input,
                h('div', { class: 'host-actions' },
                    h('button', { type: 'button', class: 'btn btn-sm', onclick: apply }, 'Replace'),
                    isServer ? h('button', {
                        type: 'button', class: 'btn btn-sm btn-ghost', title: 'Replace ' + u.host + ' with {ip} so these links follow the server IP',
                        onclick: () => replaceHost(u.host, '{ip}').then(renderSettings).catch(toastError),
                    }, 'Use {ip}') : null,
                ),
            );
        });
        const hostsSection = h('section', { class: 'set-section' },
            h('h4', null, 'Change IP in links'),
            h('p', null, 'Every host and IP your links point to. Replace one to update all its links at once — scheme, port and path are kept.'),
            usage.length ? h('div', { class: 'host-table' }, hostRows) : h('p', { class: 'muted' }, 'No links yet.'),
        );

        // --- dashboard ---
        const titleInput = h('input', { class: 'input', value: state.settings.title || '', placeholder: 'Links', maxlength: '80' });
        const saveTitle = async () => {
            try { await saveSettings({ title: titleInput.value.trim() }); renderHeader(); toast('Title saved'); } catch (err) { toastError(err); }
        };
        titleInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); saveTitle(); } });
        const dashSection = h('section', { class: 'set-section' },
            h('h4', null, 'Dashboard'),
            h('div', { class: 'set-row' },
                h('label', { class: 'field' }, h('span', { class: 'field-label' }, 'Title'), titleInput),
                h('button', { type: 'button', class: 'btn', onclick: saveTitle }, 'Save'),
            ),
        );

        // --- preferences (this browser) ---
        const themePref = prefs.get('theme', 'auto');
        const themeSeg = h('div', { class: 'segmented', role: 'group', 'aria-label': 'Theme' },
            ['auto', 'dark', 'light'].map((t) => h('button', {
                type: 'button', class: t === themePref ? 'active' : '',
                onclick: () => { prefs.set('theme', t); applyTheme(); renderSettings(); },
            }, t[0].toUpperCase() + t.slice(1))),
        );
        const toggle = (key, fallback, onChange) => {
            const input = h('input', { type: 'checkbox', checked: prefs.get(key, fallback) });
            input.addEventListener('change', () => { prefs.set(key, input.checked); onChange(input.checked); });
            return h('label', { class: 'switch' }, input, h('span'));
        };
        const pref = (title, sub, control) => h('div', { class: 'pref' }, h('div', { class: 'pref-text' }, h('b', null, title), h('span', null, sub)), control);
        const prefSection = h('section', { class: 'set-section' },
            h('h4', null, 'Preferences'),
            h('p', null, 'Saved in this browser only.'),
            pref('Theme', 'Auto follows your system.', themeSeg),
            pref('Status checks', 'The server pings each link every 30 s and shows a dot.', toggle('statusChecks', true, (on) => {
                if (on) refreshStatuses(); else { state.statuses = {}; updateStatusDots(); }
            })),
            pref('Open links in a new tab', '', toggle('newTab', true, () => renderLinks())),
        );

        // --- backup ---
        const replaceBox = h('input', { type: 'checkbox' });
        const fileInput = h('input', { type: 'file', accept: 'application/json,.json' });
        fileInput.addEventListener('change', async () => {
            const file = fileInput.files && fileInput.files[0];
            fileInput.value = '';
            if (!file) return;
            const replace = replaceBox.checked;
            if (replace) {
                const ok = await ask({ title: 'Replace all links?', text: 'Your ' + state.links.length + ' current links will be removed and replaced by the ones in “' + file.name + '”.', input: false, okLabel: 'Replace', danger: true });
                if (!ok) return;
            }
            try {
                const text = await file.text();
                const res = await api('POST', '/api/import' + (replace ? '?replace=1' : ''), text, true);
                state.links = res.links;
                state.statuses = {};
                renderLinks();
                renderSettings();
                refreshStatusesSoon();
                toast('Imported ' + res.added + (res.added === 1 ? ' link' : ' links') + (res.skipped ? ' · ' + res.skipped + ' skipped (duplicates or invalid)' : ''));
            } catch (err) { toastError(err); }
        });
        const backupSection = h('section', { class: 'set-section' },
            h('h4', null, 'Backup'),
            h('p', null, 'Export links and settings to JSON, or import a backup (also accepts an old links.db.json).'),
            h('div', { class: 'set-row' },
                h('a', { class: 'btn', href: '/api/export', download: '' }, icon('download'), 'Export'),
                h('span', { class: 'btn file-btn' }, icon('upload'), 'Import…', fileInput),
                h('label', { class: 'check' }, replaceBox, 'Replace existing links'),
            ),
        );

        body.replaceChildren(ipSection, hostsSection, dashSection, prefSection, backupSection);
    }

    // ======================================================================
    // Link status checks
    // ======================================================================
    let statusBusy = false;
    async function refreshStatuses() {
        if (!prefs.get('statusChecks', true) || state.links.length === 0 || statusBusy) return;
        statusBusy = true;
        try {
            state.statuses = await api('GET', '/api/links/status');
            updateStatusDots();
        } catch (err) {
            console.warn('status check failed', err);
        } finally {
            statusBusy = false;
        }
    }
    const refreshStatusesSoon = debounce(refreshStatuses, 800);
    setInterval(() => { if (!document.hidden) refreshStatuses(); }, 30000);

    // ======================================================================
    // Resources: formatting
    // ======================================================================
    const isNum = (v) => v !== null && v !== undefined && v !== '' && Number.isFinite(Number(v));
    const formatPercent = (p) => (isNum(p) ? Number(p).toFixed(1) : '–');

    function formatBytes(bytes, digits) {
        if (!isNum(bytes)) return '–';
        let n = Number(bytes);
        const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        let u = 0;
        while (n >= 1024 && u < units.length - 1) { n /= 1024; u++; }
        const d = digits !== undefined ? digits : (u >= 3 ? 1 : (u === 0 ? 0 : (n < 10 ? 1 : 0)));
        return n.toFixed(d) + ' ' + units[u];
    }
    const formatRate = (bps) => (isNum(bps) ? formatBytes(bps) + '/s' : '–');

    const formatClock = (ms) => (isNum(ms) && ms > 0 ? new Date(Number(ms)).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '--:--:--');
    const formatDateTime = (ms) => (isNum(ms) && ms > 0 ? new Date(Number(ms)).toLocaleString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '–');

    function formatUptime(sec) {
        if (!isNum(sec) || sec <= 0) return '–';
        const d = Math.floor(sec / 86400);
        const hh = Math.floor((sec % 86400) / 3600);
        const m = Math.floor((sec % 3600) / 60);
        if (d > 0) return d + 'd ' + hh + 'h ' + m + 'm';
        if (hh > 0) return hh + 'h ' + m + 'm';
        return m + 'm';
    }

    const formatGHz = (mhz) => (isNum(mhz) && mhz > 0 ? (mhz / 1000).toFixed(2) + ' GHz' : '–');
    const formatTempC = (c) => (isNum(c) && c > 0 ? Number(c).toFixed(0) + ' °C' : '–');
    const cleanMeta = (v) => { const s = String(v || '').trim(); return s === '' || s.toLowerCase() === 'unknown' ? '' : s; };
    const joinParts = (parts) => parts.filter((p) => p && String(p).trim() !== '').join(' · ');

    function levelFor(v, warn, crit) {
        if (!isNum(v)) return '';
        const n = Number(v);
        if (n >= crit) return 'level-crit';
        if (n >= warn) return 'level-warn';
        return 'level-ok';
    }
    function setLevel(el, cls) {
        if (!el) return;
        el.classList.remove('level-ok', 'level-warn', 'level-crit');
        if (cls) el.classList.add(cls);
    }

    function buildCpuMeta(cpu) {
        if (!cpu) return '–';
        const parts = [];
        if (cpu.model) parts.push(cpu.model);
        if (cpu.physicalCores > 0 && cpu.logicalCores > 0) parts.push(cpu.physicalCores + 'C/' + cpu.logicalCores + 'T');
        if (cpu.currentMHz > 0 && cpu.maxMHz > 0) {
            const pct = cpu.currentPercentOfMax > 0 ? ' (' + cpu.currentPercentOfMax.toFixed(0) + '%)' : '';
            parts.push(formatGHz(cpu.currentMHz) + ' / ' + formatGHz(cpu.maxMHz) + pct);
        } else if (cpu.currentMHz > 0) {
            parts.push(formatGHz(cpu.currentMHz));
        }
        if (cpu.performanceCores > 0 && cpu.efficiencyCores > 0) {
            const pl = 'P:' + cpu.performanceCores + 'C' + (cpu.performanceThreads > 0 ? '/' + cpu.performanceThreads + 'T' : '');
            const el = 'E:' + cpu.efficiencyCores + 'C' + (cpu.efficiencyThreads > 0 ? '/' + cpu.efficiencyThreads + 'T' : '');
            parts.push(pl + ' ' + el);
        }
        return joinParts(parts) || '–';
    }

    function buildMemMeta(memory) {
        if (!memory || !Array.isArray(memory.modules)) return '';
        const sized = memory.modules.filter((m) => m && Number(m.sizeBytes) > 0);
        if (!sized.length) return '';
        const first = Number(sized[0].sizeBytes);
        const vendor = sized.map((m) => cleanMeta(m.vendor)).find(Boolean);
        if (sized.every((m) => Number(m.sizeBytes) === first)) {
            return sized.length + ' × ' + formatBytes(first) + (vendor ? ' · ' + vendor : '');
        }
        return sized.length + ' modules' + (vendor ? ' · ' + vendor : '');
    }

    function buildSwapDevices(memory) {
        if (!memory || !Array.isArray(memory.swapDevices) || !memory.swapDevices.length) return '';
        const labels = memory.swapDevices.slice(0, 3).map((d) => joinParts([cleanMeta(d && d.name), cleanMeta(d && d.type)])).filter(Boolean);
        return labels.join(', ') + (memory.swapDevices.length > 3 ? ', …' : '');
    }

    function processLine(proc, kind) {
        if (!proc || !proc.name) return '–';
        const frag = document.createDocumentFragment();
        frag.append(h('span', null, proc.name));
        if (proc.pid > 0) frag.append(h('span', { class: 'muted' }, ' #' + proc.pid));
        let usage = '';
        if (kind === 'cpu' && isNum(proc.cpuPercent)) usage = formatPercent(proc.cpuPercent) + '%';
        if (kind === 'mem') usage = isNum(proc.memoryBytes) && proc.memoryBytes > 0 ? formatBytes(proc.memoryBytes) : (proc.memoryPercent > 0 ? formatPercent(proc.memoryPercent) + '%' : '');
        if (usage) frag.append(h('span', { class: 'muted' }, ' · '), h('span', { class: 'mono' }, usage));
        return frag;
    }

    const RING_C = 2 * Math.PI * 34;
    function setRing(idOrEl, pct, level) {
        const ring = typeof idOrEl === 'string' ? document.getElementById(idOrEl) : idOrEl;
        if (!ring) return;
        const p = isNum(pct) ? clamp(Number(pct), 0, 100) : 0;
        $('.ring-bar', ring).style.strokeDashoffset = String(RING_C * (1 - p / 100));
        setLevel(ring, level);
    }
    function setBar(el, pct, level) {
        if (!el) return;
        el.firstElementChild.style.width = (isNum(pct) ? clamp(Number(pct), 0, 100) : 0) + '%';
        setLevel(el, level);
    }

    // Keyed list rendering so cards are updated in place instead of rebuilt every second.
    function syncList(container, items, keyOf, create, update) {
        const existing = new Map();
        for (const el of Array.from(container.children)) existing.set(el.dataset.key, el);
        const keep = new Set();
        items.forEach((item, i) => {
            const key = String(keyOf(item, i));
            let el = existing.get(key);
            if (!el) { el = create(item); el.dataset.key = key; }
            keep.add(key);
            update(el, item);
            if (container.children[i] !== el) container.insertBefore(el, container.children[i] || null);
        });
        for (const [key, el] of existing) if (!keep.has(key)) el.remove();
    }

    // ======================================================================
    // Resources: history
    // ======================================================================
    const HISTORY_MAX_AGE = 30 * 60 * 1000;
    const HISTORY_MAX_POINTS = 2000;
    const GAP_MS = 15000;
    const DISK_PALETTE = ['#f59e0b', '#10b981', '#f472b6', '#60a5fa', '#a3e635', '#fb7185', '#2dd4bf', '#c084fc'];
    const hist = { time: [], cpu: [], mem: [], rx: [], tx: [], disks: {} };
    const diskColors = {};

    function diskColor(mount) {
        if (!diskColors[mount]) {
            const used = new Set(Object.values(diskColors));
            diskColors[mount] = DISK_PALETTE.find((c) => !used.has(c)) || '#94a3b8';
        }
        return diskColors[mount];
    }

    function pushPoint(t, cpu, mem, rx, tx, disks) {
        const n = hist.time.length;
        hist.time.push(t);
        hist.cpu.push(isNum(cpu) ? clamp(Number(cpu), 0, 100) : null);
        hist.mem.push(isNum(mem) ? clamp(Number(mem), 0, 100) : null);
        hist.rx.push(isNum(rx) ? Number(rx) : null);
        hist.tx.push(isNum(tx) ? Number(tx) : null);
        for (const m of Object.keys(disks)) if (!hist.disks[m]) hist.disks[m] = new Array(n).fill(null);
        for (const m of Object.keys(hist.disks)) hist.disks[m].push(m in disks && isNum(disks[m]) ? clamp(Number(disks[m]), 0, 100) : null);
    }

    function trimHistory() {
        const n = hist.time.length;
        if (!n) return;
        const cutoff = last(hist.time) - HISTORY_MAX_AGE;
        let drop = 0;
        while (drop < n - 1 && hist.time[drop] < cutoff) drop++;
        drop = Math.max(drop, n - HISTORY_MAX_POINTS);
        if (drop <= 0) return;
        for (const arr of [hist.time, hist.cpu, hist.mem, hist.rx, hist.tx]) arr.splice(0, drop);
        for (const m of Object.keys(hist.disks)) {
            hist.disks[m].splice(0, drop);
            if (hist.disks[m].every((v) => v === null)) { delete hist.disks[m]; delete diskColors[m]; }
        }
    }

    function seedHistory(points) {
        hist.time = []; hist.cpu = []; hist.mem = []; hist.rx = []; hist.tx = []; hist.disks = {};
        for (const p of points) {
            if (!p || !isNum(p.time)) continue;
            pushPoint(Number(p.time), p.cpu, p.mem, p.rx, p.tx, p.disks && typeof p.disks === 'object' ? p.disks : {});
        }
        trimHistory();
    }

    function appendSnapshot(d) {
        const t = isNum(d.updatedAt) ? Number(d.updatedAt) : Date.now();
        if (hist.time.length && t <= last(hist.time)) return;
        const disks = {};
        for (const disk of d.disks || []) if (disk && disk.mountpoint) disks[disk.mountpoint] = disk.usedPercent;
        pushPoint(t, d.cpu && d.cpu.percent, d.memory && d.memory.usedPercent, d.network && d.network.rxBytesPerSec, d.network && d.network.txBytesPerSec, disks);
        trimHistory();
    }

    function lowerBound(arr, x) {
        let lo = 0, hi = arr.length;
        while (lo < hi) { const mid = (lo + hi) >> 1; if (arr[mid] < x) lo = mid + 1; else hi = mid; }
        return lo;
    }
    function nearestIndex(t) {
        const n = hist.time.length;
        if (!n) return -1;
        const i = lowerBound(hist.time, t);
        if (i <= 0) return 0;
        if (i >= n) return n - 1;
        return t - hist.time[i - 1] <= hist.time[i] - t ? i - 1 : i;
    }

    // ======================================================================
    // Resources: canvas drawing
    // ======================================================================
    const MONO = getComputedStyle(document.documentElement).getPropertyValue('--mono') || 'monospace';

    function prepCanvas(canvas) {
        if (!canvas) return null;
        const rect = canvas.getBoundingClientRect();
        const w = Math.floor(rect.width);
        const hgt = Math.floor(rect.height);
        if (w < 2 || hgt < 2) return null;
        const dpr = window.devicePixelRatio || 1;
        if (canvas.width !== Math.floor(w * dpr) || canvas.height !== Math.floor(hgt * dpr)) {
            canvas.width = Math.floor(w * dpr);
            canvas.height = Math.floor(hgt * dpr);
        }
        const ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, w, hgt);
        return { ctx, w, h: hgt };
    }

    function strokeSegments(ctx, segs, color, width, dash, alpha, fillBase) {
        ctx.save();
        ctx.globalAlpha = alpha;
        if (fillBase !== null && fillBase !== undefined) {
            const top = Math.min(...segs.flat().map((p) => p[1]));
            const g = ctx.createLinearGradient(0, top, 0, fillBase);
            g.addColorStop(0, withAlpha(color, 0.26));
            g.addColorStop(1, withAlpha(color, 0));
            ctx.fillStyle = g;
            for (const seg of segs) {
                if (seg.length < 2) continue;
                ctx.beginPath();
                ctx.moveTo(seg[0][0], fillBase);
                for (const p of seg) ctx.lineTo(p[0], p[1]);
                ctx.lineTo(last(seg)[0], fillBase);
                ctx.closePath();
                ctx.fill();
            }
        }
        ctx.strokeStyle = color;
        ctx.lineWidth = width;
        ctx.lineJoin = 'round';
        ctx.lineCap = 'round';
        ctx.setLineDash(dash || []);
        for (const seg of segs) {
            ctx.beginPath();
            seg.forEach((p, i) => (i ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1])));
            if (seg.length === 1) ctx.lineTo(seg[0][0] + 0.1, seg[0][1]);
            ctx.stroke();
        }
        ctx.restore();
    }

    function segmentsOf(values, start, end, xOf, yOf) {
        const segs = [];
        let cur = null;
        let prevT = null;
        for (let i = start; i < end; i++) {
            const v = values[i];
            const t = hist.time[i];
            if (v === null || v === undefined || (prevT !== null && t - prevT > GAP_MS)) {
                if (cur) segs.push(cur);
                cur = null;
            }
            if (v !== null && v !== undefined) (cur || (cur = [])).push([xOf(t), yOf(v)]);
            prevT = t;
        }
        if (cur) segs.push(cur);
        return segs;
    }

    function drawSpark(canvas, series, maxValue) {
        const r = prepCanvas(canvas);
        if (!r) return;
        const { ctx, w, h: hgt } = r;
        const n = hist.time.length;
        if (n < 2) return;
        const windowMs = 120000;
        const t1 = last(hist.time);
        const t0 = t1 - windowMs;
        let start = lowerBound(hist.time, t0);
        if (start > 0) start--;
        let top = maxValue;
        if (top === 'auto') {
            top = 1024;
            for (const s of series) for (let i = start; i < n; i++) if (s.values[i] > top) top = s.values[i];
            top *= 1.15;
        }
        const xOf = (t) => ((t - t0) / windowMs) * w;
        const yOf = (v) => hgt - 2 - (clamp(v, 0, top) / top) * (hgt - 6);
        for (const s of series) {
            const segs = segmentsOf(s.values, start, n, xOf, yOf);
            if (segs.length) strokeSegments(ctx, segs, s.color, 1.6, null, 1, hgt);
        }
    }

    // ---------- main history chart ----------
    const chartCanvas = $('#historyChart');
    const chartTip = $('#chartTip');
    const chart = {
        rangeMs: prefs.get('range', 300000),
        hidden: new Set(prefs.get('hiddenSeries', [])),
        selected: null, // { key, time }
        hoverX: null,
        geom: null,
        legendKey: '',
    };
    const PAD = { l: 44, r: 12, t: 12, b: 24 };

    function chartSeries() {
        const c = colors();
        const list = [
            { key: 'cpu', label: 'CPU', values: hist.cpu, color: c.cpu, fill: true },
            { key: 'mem', label: 'RAM', values: hist.mem, color: c.mem, fill: true },
        ];
        for (const m of Object.keys(hist.disks).sort()) list.push({ key: 'disk:' + m, label: m, values: hist.disks[m], color: diskColor(m), dash: [5, 4] });
        return list;
    }

    function drawChart() {
        const r = prepCanvas(chartCanvas);
        if (!r) return;
        const { ctx, w, h: hgt } = r;
        const col = colors();
        const gw = w - PAD.l - PAD.r;
        const gh = hgt - PAD.t - PAD.b;
        if (gw < 10 || gh < 10) return;
        const n = hist.time.length;
        const t1 = n ? last(hist.time) : Date.now();
        const t0 = t1 - chart.rangeMs;
        const xOf = (t) => PAD.l + ((t - t0) / chart.rangeMs) * gw;
        const yOf = (v) => PAD.t + (1 - clamp(v, 0, 100) / 100) * gh;
        chart.geom = { gw, gh, t0, t1, xOf, yOf };

        ctx.font = '11px ' + MONO;
        ctx.lineWidth = 1;
        ctx.strokeStyle = col.grid;
        ctx.fillStyle = col.text;
        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';
        for (let i = 0; i <= 4; i++) {
            const y = Math.round(PAD.t + (i / 4) * gh) + 0.5;
            ctx.beginPath();
            ctx.moveTo(PAD.l, y);
            ctx.lineTo(w - PAD.r, y);
            ctx.stroke();
            ctx.fillText((100 - i * 25) + '%', PAD.l - 8, y);
        }
        ctx.textBaseline = 'alphabetic';
        ctx.textAlign = 'left'; ctx.fillText(formatClock(t0), PAD.l, hgt - 5);
        ctx.textAlign = 'center'; ctx.fillText(formatClock((t0 + t1) / 2), PAD.l + gw / 2, hgt - 5);
        ctx.textAlign = 'right'; ctx.fillText(formatClock(t1), w - PAD.r, hgt - 5);

        if (n < 2) {
            ctx.textAlign = 'center';
            ctx.fillText('Collecting data…', PAD.l + gw / 2, PAD.t + gh / 2);
            return;
        }

        let start = lowerBound(hist.time, t0);
        if (start > 0) start--;
        const list = chartSeries().filter((s) => !chart.hidden.has(s.key));
        const selKey = chart.selected ? chart.selected.key : null;
        const sel = list.find((s) => s.key === selKey) || null;

        ctx.save();
        ctx.beginPath();
        ctx.rect(PAD.l, 0, gw, hgt);
        ctx.clip();
        for (const s of list) {
            if (s === sel) continue;
            const segs = segmentsOf(s.values, start, n, xOf, yOf);
            if (segs.length) strokeSegments(ctx, segs, s.color, 1.8, s.dash, sel ? 0.22 : 1, s.fill && !sel ? PAD.t + gh : null);
        }
        if (sel) {
            const segs = segmentsOf(sel.values, start, n, xOf, yOf);
            if (segs.length) strokeSegments(ctx, segs, sel.color, 2.8, sel.dash, 1, sel.fill ? PAD.t + gh : null);
        }
        ctx.restore();

        const dot = (x, y, color, radius) => {
            ctx.beginPath();
            ctx.arc(x, y, radius, 0, Math.PI * 2);
            ctx.fillStyle = color;
            ctx.fill();
            ctx.lineWidth = 2;
            ctx.strokeStyle = col.surface;
            ctx.stroke();
        };

        // pinned selection
        if (sel && chart.selected.time >= t0) {
            const i = lowerBound(hist.time, chart.selected.time);
            const v = hist.time[i] === chart.selected.time ? sel.values[i] : null;
            if (v !== null && v !== undefined) {
                const x = xOf(hist.time[i]);
                ctx.strokeStyle = withAlpha(sel.color, 0.5);
                ctx.setLineDash([3, 3]);
                ctx.beginPath(); ctx.moveTo(x + 0.5, PAD.t); ctx.lineTo(x + 0.5, PAD.t + gh); ctx.stroke();
                ctx.setLineDash([]);
                dot(x, yOf(v), sel.color, 5);
            }
        }

        // hover crosshair + tooltip
        if (chart.hoverX !== null) {
            const i = nearestIndex(t0 + ((chart.hoverX - PAD.l) / gw) * chart.rangeMs);
            if (i >= 0 && hist.time[i] >= t0 - 1000) {
                const x = xOf(hist.time[i]);
                ctx.strokeStyle = col.border;
                ctx.beginPath(); ctx.moveTo(x + 0.5, PAD.t); ctx.lineTo(x + 0.5, PAD.t + gh); ctx.stroke();
                for (const s of list) {
                    const v = s.values[i];
                    if (v !== null && v !== undefined) dot(x, yOf(v), s.color, s === sel ? 4.5 : 3.5);
                }
                showTip(i, x, list, sel);
            } else {
                chartTip.hidden = true;
            }
        } else {
            chartTip.hidden = true;
        }
    }

    function showTip(i, x, list, sel) {
        const rows = [h('div', { class: 'tip-time' }, formatDateTime(hist.time[i]))];
        for (const s of list) {
            const v = s.values[i];
            rows.push(h('div', { class: 'tip-row' + (s === sel ? ' sel' : '') },
                h('span', { class: 'swatch' + (s.dash ? ' dashed' : ''), style: '--c:' + s.color }),
                h('span', null, s.label),
                h('b', null, v === null || v === undefined ? '–' : formatPercent(v) + '%')));
        }
        if (isNum(hist.rx[i]) || isNum(hist.tx[i])) {
            rows.push(h('div', { class: 'tip-row' }, h('span', { class: 'muted' }, '↓ ' + formatRate(hist.rx[i]) + '  ↑ ' + formatRate(hist.tx[i]))));
        }
        chartTip.replaceChildren(...rows);
        chartTip.hidden = false;
        const wrapW = chartCanvas.clientWidth;
        const tipW = chartTip.offsetWidth;
        let left = x + 16;
        if (left + tipW > wrapW - 4) left = x - tipW - 16;
        chartTip.style.left = Math.max(4, left) + 'px';
    }

    function pickSeries(px, py) {
        const g = chart.geom;
        if (!g || hist.time.length < 2) return null;
        if (px < PAD.l || px > PAD.l + g.gw || py < PAD.t - 6 || py > PAD.t + g.gh + 6) return null;
        const i = nearestIndex(g.t0 + ((px - PAD.l) / g.gw) * chart.rangeMs);
        if (i < 0) return null;
        let best = null;
        let bestDist = Infinity;
        for (const s of chartSeries()) {
            if (chart.hidden.has(s.key)) continue;
            const v = s.values[i];
            if (v === null || v === undefined) continue;
            const dist = Math.abs(py - g.yOf(v));
            if (dist < bestDist) { bestDist = dist; best = { key: s.key, time: hist.time[i] }; }
        }
        return best && bestDist <= 14 ? best : null;
    }

    function renderChartStatus() {
        const el = $('#chartStatus');
        const sel = chart.selected;
        if (!sel) { el.replaceChildren('Hover to inspect · click a line to pin it'); return; }
        const s = chartSeries().find((x) => x.key === sel.key);
        const i = lowerBound(hist.time, sel.time);
        if (!s || hist.time[i] !== sel.time) { chart.selected = null; renderChartStatus(); return; }
        el.replaceChildren(
            h('span', { class: 'swatch' + (s.dash ? ' dashed' : ''), style: '--c:' + s.color + ';display:inline-block;margin-right:6px' }),
            s.label + ' @ ' + formatDateTime(sel.time) + ' = ', h('b', null, formatPercent(s.values[i]) + '%'), ' · Esc to clear',
        );
    }

    function renderLegend() {
        const list = chartSeries();
        const key = list.map((s) => s.key + s.color).join('|') + '|' + [...chart.hidden].join(',') + '|' + (chart.selected ? chart.selected.key : '') + document.documentElement.dataset.theme;
        if (key === chart.legendKey) return;
        chart.legendKey = key;
        $('#chartLegend').replaceChildren(...list.map((s) => h('button', {
            type: 'button',
            class: (chart.hidden.has(s.key) ? 'off' : '') + (chart.selected && chart.selected.key === s.key ? ' selected' : ''),
            style: '--c:' + s.color,
            title: chart.hidden.has(s.key) ? 'Show ' + s.label : 'Hide ' + s.label,
            'aria-pressed': String(!chart.hidden.has(s.key)),
            onclick: () => {
                if (chart.hidden.has(s.key)) chart.hidden.delete(s.key);
                else {
                    chart.hidden.add(s.key);
                    if (chart.selected && chart.selected.key === s.key) chart.selected = null;
                }
                prefs.set('hiddenSeries', [...chart.hidden]);
                drawAllCharts();
            },
        }, h('span', { class: 'swatch' + (s.dash ? ' dashed' : '') }), s.label)));
    }

    let rafPending = false;
    function drawAllCharts() {
        if (rafPending) return;
        rafPending = true;
        requestAnimationFrame(() => {
            rafPending = false;
            const c = colors();
            drawSpark($('#cpuSpark'), [{ values: hist.cpu, color: c.cpu }], 100);
            drawSpark($('#memSpark'), [{ values: hist.mem, color: c.mem }], 100);
            drawSpark($('#netSpark'), [{ values: hist.rx, color: c.mem }, { values: hist.tx, color: c.cpu }], 'auto');
            drawChart();
            renderLegend();
            renderChartStatus();
        });
    }

    // chart interaction
    const canvasXY = (e) => { const r = chartCanvas.getBoundingClientRect(); return { x: e.clientX - r.left, y: e.clientY - r.top }; };
    let pointerDown = null;
    chartCanvas.addEventListener('pointermove', (e) => { chart.hoverX = canvasXY(e).x; drawAllCharts(); });
    chartCanvas.addEventListener('pointerleave', () => { chart.hoverX = null; drawAllCharts(); });
    chartCanvas.addEventListener('pointerdown', (e) => { pointerDown = canvasXY(e); });
    chartCanvas.addEventListener('pointerup', (e) => {
        if (!pointerDown) return;
        const pt = canvasXY(e);
        const moved = Math.hypot(pt.x - pointerDown.x, pt.y - pointerDown.y);
        pointerDown = null;
        if (moved > 6) return;
        const best = pickSeries(pt.x, pt.y);
        if (!best || (chart.selected && chart.selected.key === best.key && chart.selected.time === best.time)) chart.selected = null;
        else chart.selected = best;
        drawAllCharts();
    });

    for (const btn of $$('#rangeSelect button')) {
        btn.classList.toggle('active', Number(btn.dataset.range) === chart.rangeMs);
        btn.addEventListener('click', () => {
            chart.rangeMs = Number(btn.dataset.range);
            prefs.set('range', chart.rangeMs);
            for (const b of $$('#rangeSelect button')) b.classList.toggle('active', b === btn);
            drawAllCharts();
        });
    }

    function exportCSV() {
        const n = hist.time.length;
        if (!n) { toast('No data to export yet', { type: 'error' }); return; }
        const series = chartSeries();
        const t0 = last(hist.time) - chart.rangeMs;
        const header = ['timestamp'].concat(series.map((s) => s.label + ' %'), ['rx bytes/s', 'tx bytes/s']);
        const esc = (v) => (/[",\n]/.test(v) ? '"' + v.replace(/"/g, '""') + '"' : v);
        const lines = [header.map(esc).join(',')];
        for (let i = lowerBound(hist.time, t0); i < n; i++) {
            const row = [new Date(hist.time[i]).toISOString()];
            for (const s of series) { const v = s.values[i]; row.push(v === null || v === undefined ? '' : Number(v).toFixed(1)); }
            row.push(isNum(hist.rx[i]) ? hist.rx[i].toFixed(0) : '', isNum(hist.tx[i]) ? hist.tx[i].toFixed(0) : '');
            lines.push(row.join(','));
        }
        const label = { 60000: '1m', 300000: '5m', 900000: '15m', 1800000: '30m' }[chart.rangeMs] || 'range';
        const url = URL.createObjectURL(new Blob([lines.join('\n')], { type: 'text/csv' }));
        const a = h('a', { href: url, download: 'resources_last_' + label + '.csv' });
        document.body.append(a);
        a.click();
        a.remove();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
    }
    $('#exportCsvBtn').addEventListener('click', exportCSV);

    // ======================================================================
    // Resources: rendering
    // ======================================================================
    function setLive(s) {
        const pill = $('#livePill');
        pill.dataset.state = s;
        setText('liveText', s);
        $('#ipDot').classList.toggle('on', s === 'live');
    }

    function renderResources(d) {
        const cpu = d.cpu || {};
        const mem = d.memory || {};
        const host = d.host || {};

        // header + host IP
        const prevIp = state.serverIp;
        state.serverIp = d.hostIp || state.serverIp;
        state.detectedIp = d.detectedHostIp || '';
        state.hostIps = Array.isArray(d.hostIps) ? d.hostIps : [];
        state.hostname = host.hostname || state.hostname;
        renderHeader();
        if (prevIp !== state.serverIp && state.links.some((l) => hasPlaceholder(l.url) || hasPlaceholder(l.icon))) renderLinks();

        setText('hostLine', joinParts([host.hostname, host.platform, host.kernel, host.arch]) || 'This machine');
        setText('updatedAt', formatClock(d.updatedAt));

        // CPU
        setRing('cpuRing', cpu.percent, levelFor(cpu.percent, 60, 90));
        setText('cpuPercent', isNum(cpu.percent) ? Math.round(cpu.percent) : '–');
        $('#cpuRing').title = 'CPU ' + formatPercent(cpu.percent) + '%';
        setText('cpuMeta', buildCpuMeta(cpu));
        const temp = $('#cpuTemp');
        if (isNum(cpu.temperatureC) && cpu.temperatureC > 0) {
            temp.hidden = false;
            setText(temp, formatTempC(cpu.temperatureC));
            temp.className = 'chip ' + levelFor(cpu.temperatureC, 80, 90);
        } else {
            temp.hidden = true;
        }
        setText('processCount', isNum(d.processes) ? String(d.processes) : '–');
        setText('loadAvg', d.load ? [d.load.load1, d.load.load5, d.load.load15].map((v) => Number(v).toFixed(2)).join('  ') : '–');
        $('#cpuTopProc').replaceChildren(processLine(d.topCpu, 'cpu'));

        // memory
        setRing('memRing', mem.usedPercent, levelFor(mem.usedPercent, 60, 90));
        setText('memPercent', isNum(mem.usedPercent) ? Math.round(mem.usedPercent) : '–');
        $('#memRing').title = 'RAM ' + formatPercent(mem.usedPercent) + '%';
        setText('memUsed', formatBytes(mem.usedBytes));
        setText('memTotal', formatBytes(mem.totalBytes));
        setText('memMeta', buildMemMeta(mem));
        if (mem.swapTotalBytes > 0) {
            setText('swapText', formatBytes(mem.swapUsedBytes) + ' / ' + formatBytes(mem.swapTotalBytes) + ' · ' + formatPercent(mem.swapUsedPercent) + '%');
            setBar($('#swapBar').parentElement, mem.swapUsedPercent, levelFor(mem.swapUsedPercent, 60, 90));
        } else {
            setText('swapText', 'none');
            setBar($('#swapBar').parentElement, 0, '');
        }
        setText('swapDevices', buildSwapDevices(mem));
        $('#memTopProc').replaceChildren(processLine(d.topMemory, 'mem'));

        // network
        const net = d.network || {};
        setText('netRx', formatRate(net.rxBytesPerSec));
        setText('netTx', formatRate(net.txBytesPerSec));
        setText('netRxTotal', formatBytes(net.rxTotalBytes));
        setText('netTxTotal', formatBytes(net.txTotalBytes));
        setText('uptime', formatUptime(host.uptimeSec));

        renderGPUs(d.gpus);
        renderDisks(d.disks);
    }

    function renderGPUs(gpus) {
        const list = $('#gpuList');
        const items = Array.isArray(gpus) ? gpus : [];
        list.hidden = items.length === 0;
        syncList(list, items, (g, i) => (isNum(g.index) ? g.index : i) + ':' + (g.name || ''), () => {
            const r = {
                ring: h('div', { class: 'ring' },
                    h('span', { svg: '<svg viewBox="0 0 80 80"><circle class="ring-track" cx="40" cy="40" r="34"/><circle class="ring-bar" cx="40" cy="40" r="34"/></svg>', style: 'display:contents' }),
                    h('div', { class: 'ring-label' }, h('b', null, '–'), h('small', null, '%'))),
                title: h('span', null, 'GPU'),
                temp: h('span', { class: 'chip' }),
                name: h('div', { class: 'gpu-name' }),
                meta: h('div', { class: 'metric-meta' }),
                vramText: h('span', { class: 'mono' }, '–'),
                vramBar: h('div', { class: 'bar' }, h('span')),
            };
            const el = h('article', { class: 'card metric gpu' },
                h('div', { class: 'metric-top' }, r.ring,
                    h('div', { class: 'metric-info' }, h('div', { class: 'metric-title' }, r.title, r.temp), r.name, r.meta)),
                h('div', { class: 'bar-row' }, h('div', { class: 'bar-label' }, h('span', null, 'VRAM'), r.vramText), r.vramBar));
            el._refs = r;
            return el;
        }, (el, g) => {
            const r = el._refs;
            setText(r.title, items.length > 1 && isNum(g.index) ? 'GPU ' + g.index : 'GPU');
            setText(r.name, g.name || '–');
            setText(r.meta, joinParts([g.vendor, g.driver]));
            const util = isNum(g.utilizationPercent) ? Number(g.utilizationPercent) : null;
            setRing(r.ring, util, levelFor(util, 80, 95));
            setText($('b', r.ring), util === null ? '–' : Math.round(util));
            r.ring.title = util === null ? 'Utilization unavailable' : 'Utilization ' + formatPercent(util) + '%';
            const hasVram = isNum(g.memoryUsedBytes) && isNum(g.memoryTotalBytes) && g.memoryTotalBytes > 0;
            const vramPct = hasVram ? (g.memoryUsedBytes / g.memoryTotalBytes) * 100 : null;
            setText(r.vramText, hasVram ? formatBytes(g.memoryUsedBytes) + ' / ' + formatBytes(g.memoryTotalBytes) + ' · ' + formatPercent(vramPct) + '%' : '–');
            setBar(r.vramBar, vramPct, levelFor(vramPct, 80, 95));
            r.temp.hidden = !(isNum(g.temperatureC) && g.temperatureC > 0);
            setText(r.temp, formatTempC(g.temperatureC));
            r.temp.className = 'chip ' + levelFor(g.temperatureC, 80, 90);
        });
    }

    function renderDisks(disks) {
        const list = $('#diskList');
        const items = Array.isArray(disks) ? disks.filter((d) => d && d.mountpoint) : [];
        setText('diskSummary', items.length ? items.length + (items.length === 1 ? ' volume' : ' volumes') : '');
        if (!items.length) {
            if (!$('p', list)) list.replaceChildren(h('p', { class: 'muted' }, 'No disk data'));
            return;
        }
        syncList(list, items, (d) => d.mountpoint, () => {
            const refs = {
                mount: h('span', { class: 'disk-mount' }),
                pct: h('span', { class: 'disk-pct' }),
                bar: h('div', { class: 'bar' }, h('span')),
                meta: h('span'),
                size: h('span', { class: 'mono' }),
            };
            const el = h('div', { class: 'disk' },
                h('div', { class: 'disk-top' }, refs.mount, refs.pct),
                refs.bar,
                h('div', { class: 'disk-bottom' }, refs.meta, refs.size));
            el._refs = refs;
            return el;
        }, (el, d) => {
            const r = el._refs;
            const level = levelFor(d.usedPercent, 80, 90);
            setText(r.mount, d.mountpoint);
            r.mount.title = d.mountpoint;
            setText(r.pct, formatPercent(d.usedPercent) + '%');
            setLevel(r.pct, level);
            setBar(r.bar, d.usedPercent, level);
            const meta = joinParts([cleanMeta(d.driveType), cleanMeta(d.filesystem), cleanMeta(d.device), cleanMeta(d.model)]);
            setText(r.meta, meta);
            r.meta.title = meta;
            setText(r.size, formatBytes(d.usedBytes) + ' / ' + formatBytes(d.totalBytes));
        });
    }

    // ======================================================================
    // Polling
    // ======================================================================
    let paused = false;
    let needHistory = true;
    let polling = false;
    let pollTimer = null;

    async function updateResources() {
        if (polling) return;
        polling = true;
        try {
            const res = await fetch(needHistory ? '/api/resources?history=1' : '/api/resources', { cache: 'no-store' });
            if (!res.ok) throw new Error((await res.text()) || res.statusText);
            const d = await res.json();
            state.res = d;
            if (needHistory) {
                if (Array.isArray(d.history) && d.history.length) seedHistory(d.history);
                needHistory = false;
            }
            appendSnapshot(d);
            renderResources(d);
            drawAllCharts();
            if (!paused) setLive('live');
        } catch (err) {
            console.error(err);
            setLive('error');
        } finally {
            polling = false;
        }
    }

    function schedulePoll() {
        clearTimeout(pollTimer);
        pollTimer = setTimeout(async () => {
            if (!paused) await updateResources();
            schedulePoll();
        }, document.hidden ? 5000 : 1000);
    }

    const pauseBtn = $('#pauseBtn');
    function renderPause() {
        pauseBtn.replaceChildren(icon(paused ? 'play' : 'pause'), h('span', null, paused ? 'Resume' : 'Pause'));
    }
    pauseBtn.addEventListener('click', () => {
        paused = !paused;
        renderPause();
        if (paused) {
            setLive('paused');
        } else {
            needHistory = true; // fill the gap left while paused
            updateResources();
        }
    });

    document.addEventListener('visibilitychange', () => {
        if (document.hidden) return;
        needHistory = true; // background polling is slower; refill from the server
        if (!paused) updateResources();
        refreshStatuses();
        schedulePoll();
    });

    // ======================================================================
    // Keyboard shortcuts
    // ======================================================================
    document.addEventListener('keydown', (e) => {
        const typing = e.target.closest && e.target.closest('input, textarea, select, [contenteditable="true"]');
        const dialogOpen = !!$('dialog[open]');
        if (!dialogOpen && ((e.key === 'k' && (e.ctrlKey || e.metaKey)) || (e.key === '/' && !typing))) {
            e.preventDefault();
            searchInput.focus();
            searchInput.select();
            return;
        }
        if (typing || dialogOpen || e.ctrlKey || e.metaKey || e.altKey) return;
        if (e.key === 'n' || e.key === 'N') { e.preventDefault(); openLinkDialog(); }
        if (e.key === 'Escape' && chart.selected) { chart.selected = null; drawAllCharts(); }
    });

    window.addEventListener('resize', debounce(drawAllCharts, 60));

    // ======================================================================
    // Init
    // ======================================================================
    for (const el of $$('[data-icon]')) setIcon(el, el.dataset.icon);
    setIcon($('.search-icon'), 'search');
    setIcon($('#ipEditBtn'), 'edit');
    applyTheme();
    renderHeader();
    const linksRoot = $('#links');
    linksRoot.classList.add('animate');
    renderLinks();
    setTimeout(() => linksRoot.classList.remove('animate'), 900);
    renderPause();
    updateResources().then(() => { schedulePoll(); refreshStatuses(); });
})();
