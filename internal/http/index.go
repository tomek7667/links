package http

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/tomek7667/links/internal/domain"
)

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Links</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #1e1e1e;
            color: #e0e0e0;
            padding: 24px;
        }
        .container { max-width: 900px; margin: 0 auto; }
        .add-form {
            display: flex;
            gap: 10px;
            margin-bottom: 24px;
            flex-wrap: wrap;
        }
        .add-form input {
            padding: 14px 16px;
            font-size: 16px;
            border: 1px solid #444;
            border-radius: 4px;
            flex: 1;
            min-width: 160px;
            background: #2d2d2d;
            color: #e0e0e0;
        }
        .add-form input:focus { outline: none; border-color: #888; }
        .add-form button {
            padding: 14px 24px;
            font-size: 16px;
            background: transparent;
            color: #e0e0e0;
            border: 1px solid #444;
            border-radius: 4px;
            cursor: pointer;
        }
        .add-form button:hover { border-color: #888; }
        .links-list { list-style: none; }
        .link-item {
            display: flex;
            align-items: center;
            background: #2d2d2d;
            margin-bottom: 8px;
            border-radius: 4px;
            border: 1px solid #3a3a3a;
        }
        .link-item:hover { background: #353535; }
        .link-item a {
            flex: 1;
            padding: 18px 20px;
            font-size: 17px;
            color: #e0e0e0;
            text-decoration: none;
        }
        .link-url { color: #888; font-size: 14px; margin-left: 10px; }
        .delete-btn {
            padding: 18px 20px;
            font-size: 14px;
            background: transparent;
            color: #888;
            border: none;
            border-left: 1px solid #3a3a3a;
            cursor: pointer;
        }
        .delete-btn:hover { background: #4a2a2a; color: #e57373; }
        .empty { color: #888; padding: 24px; text-align: center; }

        .resources {
            margin-top: 28px;
            padding: 18px;
            background: #2d2d2d;
            border: 1px solid #3a3a3a;
            border-radius: 4px;
        }
        .resources-header {
            display: flex;
            justify-content: space-between;
            align-items: baseline;
            gap: 12px;
            flex-wrap: wrap;
            margin-bottom: 14px;
        }
        .resources-title { font-size: 20px; font-weight: 600; }
        .muted { color: #888; font-size: 13px; }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 12px;
            margin-bottom: 14px;
        }
        .stat {
            padding: 12px;
            border: 1px solid #3a3a3a;
            border-radius: 4px;
            background: #252525;
        }
        .stat-label { color: #888; font-size: 13px; margin-bottom: 6px; }
        .stat-value { font-size: 20px; line-height: 1.2; }
        .stat-sub { color: #aaa; font-size: 13px; margin-top: 4px; }
        .pill-btn {
            padding: 4px 8px;
            font-size: 12px;
            border: 1px solid #444;
            border-radius: 4px;
            background: #2d2d2d;
            color: #bbb;
            cursor: pointer;
            margin-left: 8px;
        }
        .pill-btn:hover { border-color: #888; color: #e0e0e0; }
        .disk-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 8px;
            font-size: 13px;
        }
        .disk-table th, .disk-table td {
            padding: 8px 10px;
            border-bottom: 1px solid #3a3a3a;
            text-align: left;
        }
        .disk-table th { color: #888; font-weight: 600; }
        .disk-table td[colspan] { text-align: center; }
        .disk-meta { font-size: 12px; margin-top: 4px; }
        .graph-wrap { margin-top: 14px; }
        #resourcesGraph {
            width: 100%;
            height: 180px;
            background: #1f1f1f;
            border: 1px solid #3a3a3a;
            border-radius: 4px;
            display: block;
        }
        .legend {
            display: flex;
            gap: 12px;
            flex-wrap: wrap;
            margin-top: 8px;
            color: #888;
            font-size: 12px;
        }
        .legend-item { display: flex; align-items: center; gap: 6px; }
        .legend-swatch { width: 10px; height: 10px; border-radius: 2px; }
        .graph-selection { color: #e0e0e0; font-size: 13px; margin-top: 4px; }
        .graph-actions {
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 10px;
            flex-wrap: wrap;
            margin-top: 4px;
        }
        .graph-btn {
            padding: 6px 10px;
            font-size: 12px;
            border: 1px solid #444;
            border-radius: 4px;
            background: #2d2d2d;
            color: #e0e0e0;
            cursor: pointer;
        }
        .graph-btn:hover { border-color: #888; }

        .level-ok { color: #81c784; }
        .level-warn { color: #ffb74d; }
        .level-crit { color: #e57373; }
    </style>
</head>
<body>
    <div class="container">
        <form class="add-form" id="addForm">
            <input type="text" id="title" placeholder="Title" required>
            <input type="url" id="url" placeholder="https://example.com" required>
            <button type="submit">Add</button>
        </form>
        <ul class="links-list" id="linksList">
            {{range .}}
            <li class="link-item">
                <a href="{{.Url}}" target="_blank">{{.Title}}<span class="link-url">({{.Url}})</span></a>
                <button class="delete-btn" onclick="deleteLink('{{.Url}}')">Delete</button>
            </li>
            {{else}}
            <li class="empty">No links yet</li>
            {{end}}
        </ul>

        <div class="resources" id="resources">
            <div class="resources-header">
                <div class="resources-title"><span id="hostIp">-</span></div>
                <div class="muted">Resources</div>
            </div>

            <div class="stats-grid">
                <div class="stat">
                <div class="stat-label">CPU (0-100%)</div>
                    <div class="stat-value" id="cpuPercentWrap"><span id="cpuPercent">-</span>%</div>
                    <div class="stat-sub" id="cpuMeta">-</div>
                    <div class="stat-sub">Temp: <span id="cpuTemp">-</span></div>
                    <div class="stat-sub">Processes: <span id="processCount">-</span></div>
                </div>
                <div class="stat">
                    <div class="stat-label">RAM</div>
                    <div class="stat-value"><span id="memUsed">-</span> / <span id="memTotal">-</span></div>
                    <div class="stat-sub"><span id="memPercent">-</span>% used</div>
                    <div class="stat-sub" id="memMeta">-</div>
                    <div class="stat-sub" id="swapMeta">Swap/pagefile: -</div>
                </div>
                <div class="stat">
                    <div class="stat-label">Last update <button type="button" class="pill-btn" id="pauseBtn">Pause</button></div>
                    <div class="stat-value"><span id="updatedAt">-</span></div>
                    <div class="stat-sub" id="resourcesStatus">-</div>
                </div>
            </div>

            <div class="stat" id="gpuSection" style="display:none">
                <div class="stat-label">GPU</div>
                <table class="disk-table">
                    <thead>
                        <tr>
                            <th>GPU</th>
                            <th>Util</th>
                            <th>VRAM</th>
                            <th>Temp</th>
                        </tr>
                    </thead>
                    <tbody id="gpuTableBody">
                        <tr><td colspan="4" class="muted">No GPU data</td></tr>
                    </tbody>
                </table>
            </div>

            <div class="stat">
                <div class="stat-label">Disks</div>
                <table class="disk-table">
                    <thead>
                        <tr>
                            <th>Mount</th>
                            <th>Used</th>
                            <th>Total</th>
                            <th>%</th>
                        </tr>
                    </thead>
                    <tbody id="diskTableBody">
                        <tr><td colspan="4" class="muted">Loading...</td></tr>
                    </tbody>
                </table>
            </div>

            <div class="graph-wrap">
                <div class="stat-label">History</div>
                <div class="graph-actions">
                    <div class="muted" id="graphMeta">-</div>
                    <button type="button" class="graph-btn" id="exportCsvBtn">Export CSV</button>
                </div>
                <div class="graph-selection" id="graphSelection">Click a line to select</div>
                <canvas id="resourcesGraph"></canvas>
                <div class="legend" id="graphLegend"></div>
            </div>
        </div>
    </div>
    <script>
        document.getElementById('addForm').onsubmit = async (e) => {
            e.preventDefault();
            const title = document.getElementById('title').value;
            const url = document.getElementById('url').value;
            await fetch('/api/links', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({title, url})
            });
            location.reload();
        };
        window.deleteLink = async (url) => {
            await fetch('/api/links', {
                method: 'DELETE',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({url})
            });
            location.reload();
        };

        const pollIntervalMs = 1000;
        const resourcesState = {
            intervalMs: pollIntervalMs,
            maxPoints: 2000,
            maxAgeMs: 30 * 60 * 1000,
            tick: 0,
            seeded: false,
            seriesLastSeen: {},
            selected: { label: null, index: null },
            palette: ['#4fc3f7', '#81c784', '#ffb74d', '#ba68c8', '#e57373', '#64b5f6', '#aed581'],
            colors: {},
            history: {
                time: [],
                cpu: [],
                mem: [],
                disks: {},
            },
        };
        let needHistory = true;
        let paused = false;

        const escapeHtml = (str) => {
            return String(str)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#039;');
        };

        const setText = (id, text) => {
            const el = document.getElementById(id);
            if (el) el.textContent = text;
        };

        const clampPercent = (v) => {
            if (v === null || v === undefined) return 0;
            const n = Number(v);
            if (Number.isNaN(n)) return 0;
            return Math.max(0, Math.min(100, n));
        };

        const formatGB = (bytes) => {
            if (bytes === null || bytes === undefined) return '-';
            const gb = Number(bytes) / 1024 / 1024 / 1024;
            if (!Number.isFinite(gb)) return '-';
            return gb.toFixed(1) + ' GB';
        };

        const formatPercent = (p) => {
            const n = Number(p);
            if (!Number.isFinite(n)) return '-';
            return n.toFixed(1);
        };

		const formatTime = (ms) => {
			const n = Number(ms);
			if (!Number.isFinite(n) || n <= 0) return '-';
			return new Date(n).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
		};

		const formatDateTime = (ms) => {
			const n = Number(ms);
			if (!Number.isFinite(n) || n <= 0) return '-';
			return new Date(n).toLocaleString(undefined, {
				year: 'numeric',
				month: '2-digit',
				day: '2-digit',
				hour: '2-digit',
				minute: '2-digit',
				second: '2-digit',
			});
		};

        const formatDuration = (ms) => {
            const totalSec = Math.max(0, Math.round(Number(ms) / 1000));
            const minutes = Math.floor(totalSec / 60);
            const seconds = totalSec % 60;
            if (!Number.isFinite(totalSec)) return '-';
            if (minutes <= 0) return String(seconds) + 's';
            if (seconds === 0) return String(minutes) + 'm';
            return String(minutes) + 'm ' + String(seconds) + 's';
        };

        const joinParts = (parts) => parts.filter(p => p && String(p).trim() !== '').join(' | ');

        const formatGHz = (mhz) => {
            const n = Number(mhz);
            if (!Number.isFinite(n) || n <= 0) return '-';
            return (n / 1000).toFixed(2) + ' GHz';
        };

        const formatTempC = (c) => {
            const n = Number(c);
            if (!Number.isFinite(n) || n <= 0) return '-';
            return n.toFixed(0) + ' C';
        };

        const clearLevel = (el) => {
            if (!el) return;
            el.classList.remove('level-ok', 'level-warn', 'level-crit');
        };

        const setLevel = (el, cls) => {
            if (!el) return;
            clearLevel(el);
            if (cls) el.classList.add(cls);
        };

        const levelForPercent = (v, warn, crit) => {
            const n = Number(v);
            if (!Number.isFinite(n)) return '';
            if (n >= crit) return 'level-crit';
            if (n >= warn) return 'level-warn';
            return 'level-ok';
        };

        const levelForTemp = (c, warn, crit) => {
            const n = Number(c);
            if (!Number.isFinite(n)) return '';
            if (n >= crit) return 'level-crit';
            if (n >= warn) return 'level-warn';
            return 'level-ok';
        };

        const buildCpuMeta = (cpu) => {
            if (!cpu) return '-';

            const parts = [];
            if (cpu.model) parts.push(cpu.model);

            if (Number.isFinite(cpu.physicalCores) && cpu.physicalCores > 0 && Number.isFinite(cpu.logicalCores) && cpu.logicalCores > 0) {
                parts.push(String(cpu.physicalCores) + 'C/' + String(cpu.logicalCores) + 'T');
            }

            if (Number.isFinite(cpu.currentMHz) && cpu.currentMHz > 0 && Number.isFinite(cpu.maxMHz) && cpu.maxMHz > 0) {
                const pct = Number.isFinite(cpu.currentPercentOfMax) && cpu.currentPercentOfMax > 0 ? ' (' + cpu.currentPercentOfMax.toFixed(0) + '%)' : '';
                parts.push(formatGHz(cpu.currentMHz) + '/' + formatGHz(cpu.maxMHz) + pct);
            } else if (Number.isFinite(cpu.currentMHz) && cpu.currentMHz > 0) {
                parts.push(formatGHz(cpu.currentMHz));
            }

            if (Number.isFinite(cpu.performanceCores) && cpu.performanceCores > 0 && Number.isFinite(cpu.efficiencyCores) && cpu.efficiencyCores > 0) {
                const pThreads = Number.isFinite(cpu.performanceThreads) && cpu.performanceThreads > 0 ? cpu.performanceThreads : null;
                const eThreads = Number.isFinite(cpu.efficiencyThreads) && cpu.efficiencyThreads > 0 ? cpu.efficiencyThreads : null;

                const pLabel = pThreads ? ('P:' + cpu.performanceCores + 'C/' + pThreads + 'T') : ('P:' + cpu.performanceCores + 'C');
                const eLabel = eThreads ? ('E:' + cpu.efficiencyCores + 'C/' + eThreads + 'T') : ('E:' + cpu.efficiencyCores + 'C');
                parts.push(pLabel + ' ' + eLabel);
            }

            const out = joinParts(parts);
            return out !== '' ? out : '-';
        };

        const buildMemMeta = (memory) => {
            if (!memory) return '-';

            const parts = [];
            if (Array.isArray(memory.modules) && memory.modules.length > 0) {
                const sized = memory.modules.filter(m => m && Number(m.sizeBytes) > 0);
                if (sized.length > 0) {
                    const firstSize = Number(sized[0].sizeBytes);
                    const sameSize = sized.every(m => Number(m.sizeBytes) === firstSize);
                    const vendor = sized.map(m => m.vendor).find(v => v && String(v).trim() !== '');
                    if (sameSize) {
                        const sizeStr = formatGB(firstSize);
                        parts.push(String(sized.length) + 'x' + sizeStr + (vendor ? ' (' + vendor + ')' : ''));
                    } else {
                        parts.push(String(sized.length) + ' modules');
                    }
                }
            }

            const out = joinParts(parts);
            return out !== '' ? out : '-';
        };

        const buildSwapMeta = (memory) => {
            if (!memory || Number(memory.swapTotalBytes) <= 0) return 'Swap/pagefile: -';
            const pct = Number(memory.swapUsedPercent);
            const pctStr = Number.isFinite(pct) && pct > 0 ? (' (' + formatPercent(pct) + '%)') : '';
            return 'Swap/pagefile: ' + formatGB(memory.swapUsedBytes) + ' / ' + formatGB(memory.swapTotalBytes) + pctStr;
        };

        const renderDisks = (disks) => {
            const body = document.getElementById('diskTableBody');
            if (!body) return;
            if (!Array.isArray(disks) || disks.length === 0) {
                body.innerHTML = '<tr><td colspan="4" class="muted">No disk data</td></tr>';
                return;
            }
            body.innerHTML = disks.map(d => {
                const mount = d && d.mountpoint ? d.mountpoint : '';
                const device = d && d.device ? d.device : '';
                const filesystem = d && d.filesystem ? d.filesystem : '';
                const driveType = d && d.driveType ? d.driveType : '';
                const model = d && d.model ? d.model : '';
                const used = d ? d.usedBytes : null;
                const total = d ? d.totalBytes : null;
                const pct = d ? d.usedPercent : null;

                const metaParts = [];
                if (driveType) metaParts.push(driveType);
                if (filesystem) metaParts.push(filesystem);
                if (device) metaParts.push(device);
                if (model) metaParts.push(model);
                const meta = metaParts.join(' | ');

                const mountCell = (
                    '<div>' + escapeHtml(mount) + '</div>' +
                    (meta ? '<div class="muted disk-meta">' + escapeHtml(meta) + '</div>' : '')
                );

                const pctNum = Number(pct);
                const pctCls = levelForPercent(pctNum, 80, 90);
                const pctCell = '<span class="' + pctCls + '">' + escapeHtml(formatPercent(pct)) + '%</span>';
                return (
                    '<tr>' +
                        '<td>' + mountCell + '</td>' +
                        '<td>' + escapeHtml(formatGB(used)) + '</td>' +
                        '<td>' + escapeHtml(formatGB(total)) + '</td>' +
                        '<td>' + pctCell + '</td>' +
                    '</tr>'
                );
            }).join('');
        };

        const renderGPUs = (gpus) => {
            const section = document.getElementById('gpuSection');
            const body = document.getElementById('gpuTableBody');
            if (!section || !body) return;

            if (!Array.isArray(gpus) || gpus.length === 0) {
                section.style.display = 'none';
                return;
            }

            section.style.display = '';
            body.innerHTML = gpus.map(g => {
                const name = g && g.name ? g.name : '-';
                const vendor = g && g.vendor ? g.vendor : '';
                const driver = g && g.driver ? g.driver : '';
                const util = g && (g.utilizationPercent !== null && g.utilizationPercent !== undefined) ? g.utilizationPercent : null;
                const memUsed = g && (g.memoryUsedBytes !== null && g.memoryUsedBytes !== undefined) ? g.memoryUsedBytes : null;
                const memTotal = g && (g.memoryTotalBytes !== null && g.memoryTotalBytes !== undefined) ? g.memoryTotalBytes : null;
                const temp = g && (g.temperatureC !== null && g.temperatureC !== undefined) ? g.temperatureC : null;

                const meta = joinParts([vendor, driver]);
                const nameCell = (
                    '<div>' + escapeHtml(name) + '</div>' +
                    (meta !== '' ? '<div class="muted disk-meta">' + escapeHtml(meta) + '</div>' : '')
                );

                const utilCls = levelForPercent(util, 80, 95);
                const utilCell = (util === null) ? '-' : ('<span class="' + utilCls + '">' + escapeHtml(formatPercent(util)) + '%</span>');
                const vramCell = (memUsed === null || memTotal === null) ? '-' : (formatGB(memUsed) + ' / ' + formatGB(memTotal));
                const tempCls = levelForTemp(temp, 80, 90);
                const tempCell = (temp === null) ? '-' : ('<span class="' + tempCls + '">' + escapeHtml(formatTempC(temp)) + '</span>');

                return (
                    '<tr>' +
                        '<td>' + nameCell + '</td>' +
                        '<td>' + utilCell + '</td>' +
                        '<td>' + escapeHtml(vramCell) + '</td>' +
                        '<td>' + tempCell + '</td>' +
                    '</tr>'
                );
            }).join('');
        };

        const colorFor = (label) => {
            if (resourcesState.colors[label]) return resourcesState.colors[label];
            const used = new Set(Object.values(resourcesState.colors));
            for (const c of resourcesState.palette) {
                if (!used.has(c)) {
                    resourcesState.colors[label] = c;
                    return c;
                }
            }
            resourcesState.colors[label] = '#90a4ae';
            return resourcesState.colors[label];
        };

        const renderLegend = (disks) => {
            const el = document.getElementById('graphLegend');
            if (!el) return;

            const items = [
                { label: 'CPU', color: colorFor('CPU') },
                { label: 'RAM', color: colorFor('RAM') },
            ];

            if (Array.isArray(disks)) {
                for (const d of disks) {
                    if (d && d.mountpoint) {
                        items.push({ label: d.mountpoint, color: colorFor(d.mountpoint) });
                    }
                }
            }

            el.innerHTML = items.map(i => (
                '<div class="legend-item">' +
                    '<div class="legend-swatch" style="background:' + escapeHtml(i.color) + '"></div>' +
                    '<div>' + escapeHtml(i.label) + '</div>' +
                '</div>'
            )).join('');
        };

        const seedHistoryFromServer = (points) => {
            if (!Array.isArray(points) || points.length === 0) return;

            resourcesState.history.time = [];
            resourcesState.history.cpu = [];
            resourcesState.history.mem = [];
            resourcesState.history.disks = {};
            resourcesState.seriesLastSeen = {};
            resourcesState.selected = { label: null, index: null };
            resourcesState.tick = 0;

            for (const p of points) {
                const ts = Number(p.time);
                if (!Number.isFinite(ts)) {
                    continue;
                }
                const cpu = clampPercent(p.cpu);
                const mem = clampPercent(p.mem);

                resourcesState.history.time.push(ts);
                resourcesState.history.cpu.push(cpu);
                resourcesState.history.mem.push(mem);

                const disks = (p && p.disks && typeof p.disks === 'object') ? p.disks : {};
                const currentLen = resourcesState.history.cpu.length;
                for (const mount of Object.keys(disks)) {
                    if (!resourcesState.history.disks[mount]) {
                        resourcesState.history.disks[mount] = new Array(currentLen - 1).fill(null);
                    }
                    resourcesState.history.disks[mount].push(clampPercent(disks[mount]));
                    resourcesState.seriesLastSeen[mount] = resourcesState.tick;
                }
                for (const mount of Object.keys(resourcesState.history.disks)) {
                    if (!(mount in disks)) {
                        resourcesState.history.disks[mount].push(null);
                    }
                }
                resourcesState.tick += 1;
            }
            resourcesState.seeded = true;
        };

        const appendPoint = (snapshot) => {
            resourcesState.tick += 1;
            const tick = resourcesState.tick;

            const ts = snapshot && snapshot.updatedAt ? Number(snapshot.updatedAt) : Date.now();
            resourcesState.history.time.push(ts);

            const cpu = snapshot && snapshot.cpu ? snapshot.cpu.percent : 0;
            const mem = snapshot && snapshot.memory ? snapshot.memory.usedPercent : 0;

            resourcesState.history.cpu.push(clampPercent(cpu));
            resourcesState.history.mem.push(clampPercent(mem));

            const diskMap = {};
            if (snapshot && Array.isArray(snapshot.disks)) {
                for (const d of snapshot.disks) {
                    if (!d || !d.mountpoint) continue;
                    diskMap[d.mountpoint] = clampPercent(d.usedPercent);
                    resourcesState.seriesLastSeen[d.mountpoint] = tick;
                }
            }

            const currentLen = resourcesState.history.cpu.length;
            for (const mount of Object.keys(diskMap)) {
                if (!resourcesState.history.disks[mount]) {
                    resourcesState.history.disks[mount] = new Array(currentLen - 1).fill(null);
                }
                resourcesState.history.disks[mount].push(diskMap[mount]);
            }
            for (const mount of Object.keys(resourcesState.history.disks)) {
                if (!(mount in diskMap)) {
                    resourcesState.history.disks[mount].push(null);
                }
            }

            let shifted = 0;
            const nowTs = resourcesState.history.time[resourcesState.history.time.length - 1];
            while (resourcesState.history.time.length > 1 && resourcesState.history.time[0] < nowTs - resourcesState.maxAgeMs) {
                resourcesState.history.time.shift();
                resourcesState.history.cpu.shift();
                resourcesState.history.mem.shift();
                for (const mount of Object.keys(resourcesState.history.disks)) {
                    resourcesState.history.disks[mount].shift();
                }
                shifted += 1;
            }
            while (resourcesState.history.time.length > resourcesState.maxPoints) {
                resourcesState.history.time.shift();
                resourcesState.history.cpu.shift();
                resourcesState.history.mem.shift();
                for (const mount of Object.keys(resourcesState.history.disks)) {
                    resourcesState.history.disks[mount].shift();
                }
                shifted += 1;
            }

            if (shifted > 0 && resourcesState.selected && resourcesState.selected.index !== null && resourcesState.selected.index !== undefined) {
                resourcesState.selected.index -= shifted;
                if (resourcesState.selected.index < 0) {
                    resourcesState.selected = { label: null, index: null };
                }
            }

            for (const mount of Object.keys(resourcesState.seriesLastSeen)) {
                if (tick - resourcesState.seriesLastSeen[mount] > resourcesState.maxPoints) {
                    delete resourcesState.seriesLastSeen[mount];
                    delete resourcesState.history.disks[mount];
                    delete resourcesState.colors[mount];
                    if (resourcesState.selected && resourcesState.selected.label === mount) {
                        resourcesState.selected = { label: null, index: null };
                    }
                }
            }
        };

        const canvas = document.getElementById('resourcesGraph');
        const ctx = canvas ? canvas.getContext('2d') : null;

        const resizeCanvas = () => {
            if (!canvas || !ctx) return { width: 0, height: 0 };
            const dpr = window.devicePixelRatio || 1;
            const rect = canvas.getBoundingClientRect();
            const width = Math.max(1, Math.floor(rect.width));
            const height = Math.max(1, Math.floor(rect.height));
            if (canvas.width !== Math.floor(width * dpr) || canvas.height !== Math.floor(height * dpr)) {
                canvas.width = Math.floor(width * dpr);
                canvas.height = Math.floor(height * dpr);
            }
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            return { width, height };
        };

        const drawGrid = (width, height, padL, padT, padR, padB) => {
            ctx.strokeStyle = '#2f2f2f';
            ctx.lineWidth = 1;
            ctx.beginPath();
            const steps = 4;
            for (let i = 0; i <= steps; i++) {
                const y = padT + i * ((height - padT - padB) / steps);
                ctx.moveTo(padL, y);
                ctx.lineTo(width - padR, y);
            }
            ctx.stroke();

            ctx.fillStyle = '#888';
            ctx.font = '12px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
            ctx.textBaseline = 'middle';
            for (let i = 0; i <= steps; i++) {
                const value = Math.round(100 - i * (100 / steps));
                const y = padT + i * ((height - padT - padB) / steps);
                ctx.fillText(String(value) + '%', 6, y);
            }
        };

        const drawSeries = (values, color, width, height, padL, padT, padR, padB, opts) => {
            const n = values.length;
            if (n < 2) return;

            const graphW = width - padL - padR;
            const graphH = height - padT - padB;

            const lineWidth = opts && opts.lineWidth ? opts.lineWidth : 2;
            const alpha = opts && opts.alpha !== undefined ? opts.alpha : 1;
            const dash = opts && opts.dash ? opts.dash : [];

            ctx.strokeStyle = color;
            ctx.lineWidth = lineWidth;
            ctx.globalAlpha = alpha;
            ctx.setLineDash(dash);
            ctx.beginPath();

            let started = false;
            for (let i = 0; i < n; i++) {
                const v = values[i];
                if (v === null || v === undefined || Number.isNaN(v)) {
                    started = false;
                    continue;
                }
                const x = padL + (i / (n - 1)) * graphW;
                const y = padT + (1 - (v / 100)) * graphH;
                if (!started) {
                    ctx.moveTo(x, y);
                    started = true;
                } else {
                    ctx.lineTo(x, y);
                }
            }
            ctx.stroke();
            ctx.setLineDash([]);
            ctx.globalAlpha = 1;
        };

        const getSeriesList = () => {
            const series = [
                { label: 'CPU', values: resourcesState.history.cpu, dash: [] },
                { label: 'RAM', values: resourcesState.history.mem, dash: [] },
            ];
            const mounts = Object.keys(resourcesState.history.disks).sort();
            for (const mount of mounts) {
                series.push({ label: mount, values: resourcesState.history.disks[mount], dash: [6, 4] });
            }
            return series;
        };

        const exportGraphCSV = () => {
            const times = resourcesState.history.time;
            if (!times || times.length === 0) return;

            const seriesList = getSeriesList();
            const header = ['timestamp'].concat(seriesList.map(s => s.label));
            const lines = [header.join(',')];

            const latest = times[times.length - 1];
            const cutoff = latest - 30 * 1000; // last 30 seconds

            let startIdx = 0;
            for (let i = 0; i < times.length; i++) {
                if (times[i] >= cutoff) {
                    startIdx = i;
                    break;
                }
            }

            for (let i = startIdx; i < times.length; i++) {
                const row = [];
                const ts = times[i];
                row.push(new Date(ts).toISOString());

                for (const s of seriesList) {
                    const v = s.values && i < s.values.length ? s.values[i] : null;
                    if (v === null || v === undefined || Number.isNaN(v)) {
                        row.push('');
                    } else {
                        row.push(Number(v).toFixed(1));
                    }
                }
                lines.push(row.join(','));
            }

            const csv = lines.join('\n');
            const blob = new Blob([csv], { type: 'text/csv' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'resources_last_30m.csv';
            document.body.appendChild(a);
            a.click();
            a.remove();
            setTimeout(() => URL.revokeObjectURL(url), 1000);
        };

        const drawGraph = () => {
            if (!canvas || !ctx) return;
            const dim = resizeCanvas();
            const width = dim.width;
            const height = dim.height;
            if (!width || !height) return;

            ctx.clearRect(0, 0, width, height);
            ctx.fillStyle = '#1f1f1f';
            ctx.fillRect(0, 0, width, height);

            const padL = 44, padT = 14, padR = 12, padB = 26;
            const graphW = width - padL - padR;
            const graphH = height - padT - padB;

            drawGrid(width, height, padL, padT, padR, padB);

            const n = resourcesState.history.cpu.length;
            const times = resourcesState.history.time;
            const windowMs = (times && times.length >= 2) ? Math.max(0, times[times.length - 1] - times[0]) : 0;
            setText('graphMeta', 'Last ' + formatDuration(windowMs) + ' | ' + String(Math.round(resourcesState.intervalMs / 100) / 10) + 's/sample | Y: %');

            ctx.fillStyle = '#888';
            ctx.font = '12px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
            ctx.textBaseline = 'alphabetic';
            ctx.textAlign = 'left';
            if (times && times.length >= 2) {
                ctx.fillText(formatTime(times[0]), padL, height - 6);
            }
            ctx.textAlign = 'right';
            if (times && times.length >= 2) {
                ctx.fillText(formatTime(times[times.length - 1]), width - padR, height - 6);
            } else {
                ctx.fillText('now', width - padR, height - 6);
            }
            ctx.textAlign = 'left';

            const selectedLabel = resourcesState.selected && resourcesState.selected.label ? resourcesState.selected.label : null;
            const selectedIndex = resourcesState.selected ? resourcesState.selected.index : null;

            const seriesList = getSeriesList();
            const dimAlpha = selectedLabel ? 0.25 : 1;

            for (const s of seriesList) {
                if (selectedLabel && s.label === selectedLabel) continue;
                drawSeries(s.values, colorFor(s.label), width, height, padL, padT, padR, padB, { dash: s.dash, alpha: dimAlpha, lineWidth: 2 });
            }
            if (selectedLabel) {
                const sel = seriesList.find(s => s.label === selectedLabel);
                if (sel) {
                    drawSeries(sel.values, colorFor(sel.label), width, height, padL, padT, padR, padB, { dash: sel.dash, alpha: 1, lineWidth: 3 });
                }
            }

            if (selectedLabel && selectedIndex !== null && selectedIndex !== undefined && n >= 2 && graphW > 0 && graphH > 0) {
                const sel = seriesList.find(s => s.label === selectedLabel);
                const idx = Math.max(0, Math.min(n - 1, selectedIndex));
                if (sel && sel.values && idx < sel.values.length) {
                    const v = sel.values[idx];
                    if (v !== null && v !== undefined && !Number.isNaN(v)) {
                        const x = padL + (idx / (n - 1)) * graphW;
                        const y = padT + (1 - (v / 100)) * graphH;

                        ctx.strokeStyle = '#3a3a3a';
                        ctx.lineWidth = 1;
                        ctx.beginPath();
                        ctx.moveTo(x, padT);
                        ctx.lineTo(x, height - padB);
                        ctx.stroke();

                        ctx.fillStyle = colorFor(selectedLabel);
                        ctx.beginPath();
                        ctx.arc(x, y, 4, 0, Math.PI * 2);
                        ctx.fill();

                        const at = (times && idx < times.length) ? formatDateTime(times[idx]) : '-';
                        setText('graphSelection', 'Selected: ' + selectedLabel + ' @ ' + at + ' = ' + formatPercent(v) + '%');
                    }
                }
            }

            if (!selectedLabel) {
                setText('graphSelection', 'Click a line to select');
            }
        };

        const getCanvasXY = (e) => {
            if (!canvas) return null;
            const rect = canvas.getBoundingClientRect();
            return { x: e.clientX - rect.left, y: e.clientY - rect.top };
        };

        const pickSeriesAt = (x, y, width, height) => {
            const padL = 44, padT = 14, padR = 12, padB = 26;
            const graphW = width - padL - padR;
            const graphH = height - padT - padB;
            const n = resourcesState.history.cpu.length;
            if (n < 2 || graphW <= 0 || graphH <= 0) return null;
            if (x < padL || x > (width - padR) || y < padT || y > (height - padB)) return null;

            const idx = Math.max(0, Math.min(n - 1, Math.round(((x - padL) / graphW) * (n - 1))));
            const seriesList = getSeriesList();

            let best = null;
            let bestDist = Infinity;
            for (const s of seriesList) {
                const v = s.values && idx < s.values.length ? s.values[idx] : null;
                if (v === null || v === undefined || Number.isNaN(v)) continue;
                const ySeries = padT + (1 - (v / 100)) * graphH;
                const dist = Math.abs(y - ySeries);
                if (dist < bestDist) {
                    bestDist = dist;
                    best = { label: s.label, index: idx };
                }
            }
            if (!best || bestDist > 12) return null;
            return best;
        };

        if (canvas) {
            let down = null;

            canvas.addEventListener('pointerdown', (e) => {
                const pt = getCanvasXY(e);
                if (!pt) return;
                down = { id: e.pointerId, x: pt.x, y: pt.y };
                if (canvas.setPointerCapture) canvas.setPointerCapture(e.pointerId);
            });

            canvas.addEventListener('pointerup', (e) => {
                if (!down || down.id !== e.pointerId) return;
                const pt = getCanvasXY(e);
                if (!pt) return;

                const moved = Math.hypot(pt.x - down.x, pt.y - down.y);
                down = null;
                if (moved > 5) return;

                const dim = resizeCanvas();
                const width = dim.width;
                const height = dim.height;
                if (!width || !height) return;

                const best = pickSeriesAt(pt.x, pt.y, width, height);
                if (!best) {
                    if (resourcesState.selected && resourcesState.selected.label) {
                        resourcesState.selected = { label: null, index: null };
                        drawGraph();
                    }
                    return;
                }

                const sameLabel = resourcesState.selected && resourcesState.selected.label === best.label;
                const sameIndex = resourcesState.selected && resourcesState.selected.index === best.index;
                if (sameLabel && sameIndex) {
                    resourcesState.selected = { label: null, index: null };
                } else {
                    resourcesState.selected = best;
                }
                drawGraph();
            });

            canvas.addEventListener('pointercancel', () => {
                down = null;
            });
        }

        window.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                resourcesState.selected = { label: null, index: null };
                drawGraph();
            }
        });

        const updateResources = async () => {
            const statusEl = document.getElementById('resourcesStatus');
            try {
                const url = needHistory ? '/api/resources?history=1' : '/api/resources';
                const res = await fetch(url, { cache: 'no-store' });
                if (!res.ok) throw new Error(await res.text());
                const data = await res.json();

                const cpu = data && data.cpu ? data.cpu : null;
                const memory = data && data.memory ? data.memory : null;
                const historyPoints = data && Array.isArray(data.history) ? data.history : null;

                setText('hostIp', (data && data.hostIp) ? data.hostIp : '-');
                setText('cpuPercent', formatPercent(cpu ? cpu.percent : null));
                setText('cpuMeta', buildCpuMeta(cpu));
                setText('cpuTemp', formatTempC(cpu ? cpu.temperatureC : null));
                setLevel(document.getElementById('cpuPercentWrap'), levelForPercent(cpu ? cpu.percent : null, 60, 90));
                setLevel(document.getElementById('cpuTemp'), levelForTemp(cpu ? cpu.temperatureC : null, 80, 90));
                setText('processCount', data && Number.isFinite(Number(data.processes)) ? String(Number(data.processes)) : '-');

                setText('memUsed', formatGB(memory ? memory.usedBytes : null));
                setText('memTotal', formatGB(memory ? memory.totalBytes : null));
                setText('memPercent', formatPercent(memory ? memory.usedPercent : null));
                setText('memMeta', buildMemMeta(memory));
                setText('swapMeta', buildSwapMeta(memory));
                setLevel(document.getElementById('memPercent'), levelForPercent(memory ? memory.usedPercent : null, 60, 90));
                setText('updatedAt', formatTime(data ? data.updatedAt : null));

                renderDisks(data ? data.disks : null);
                renderGPUs(data ? data.gpus : null);
                renderLegend(data ? data.disks : null);

                if (!resourcesState.seeded && historyPoints && historyPoints.length > 0) {
                    seedHistoryFromServer(historyPoints);
                    needHistory = false;
                }
                const lastTs = resourcesState.history.time.length > 0 ? resourcesState.history.time[resourcesState.history.time.length - 1] : null;
                const snapTs = data && data.updatedAt ? Number(data.updatedAt) : null;
                if (!Number.isFinite(lastTs) || snapTs === null || snapTs === undefined || snapTs !== lastTs) {
                    appendPoint(data);
                }
                drawGraph();

                if (statusEl) statusEl.textContent = 'live';
            } catch (err) {
                console.error(err);
                if (statusEl) statusEl.textContent = 'error';
            }
        };

        const getPollDelayMs = () => (document.hidden ? 5000 : pollIntervalMs);

        const poll = async () => {
            if (!paused) {
                await updateResources();
            } else {
                setText('resourcesStatus', 'paused');
            }
            setTimeout(poll, getPollDelayMs());
        };

        window.addEventListener('resize', () => drawGraph());
        const exportBtn = document.getElementById('exportCsvBtn');
        if (exportBtn) {
            exportBtn.addEventListener('click', exportGraphCSV);
        }
        const pauseBtn = document.getElementById('pauseBtn');
        if (pauseBtn) {
            pauseBtn.addEventListener('click', () => {
                paused = !paused;
                pauseBtn.textContent = paused ? 'Resume' : 'Pause';
                if (!paused) {
                    updateResources();
                } else {
                    setText('resourcesStatus', 'paused');
                }
            });
        }
        poll();
    </script>
</body>
</html>`

var indexTmpl = template.Must(template.New("index").Parse(indexHTML))

func (s *Server) AddIndexRoute() {
	s.r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		links := s.dber.GetLinks()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		indexTmpl.Execute(w, links)
	})

	s.r.Post("/api/links", func(w http.ResponseWriter, r *http.Request) {
		var link domain.Link
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.dber.SaveLink(link)
		w.WriteHeader(http.StatusCreated)
	})

	s.r.Delete("/api/links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Url string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.dber.DeleteLink(req.Url)
		w.WriteHeader(http.StatusOK)
	})

	s.r.Get("/api/resources", func(w http.ResponseWriter, r *http.Request) {
		if s.resources == nil {
			http.Error(w, "resources not available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		withHistory := r.URL.Query().Get("history") == "1"
		json.NewEncoder(w).Encode(s.resources.Snapshot(withHistory))
	})
}
