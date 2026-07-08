document.addEventListener('DOMContentLoaded', () => {
    // Navigation tabs
    const navItems = document.querySelectorAll('.nav-item');
    const tabPanes = document.querySelectorAll('.tab-pane');

    navItems.forEach(item => {
        item.addEventListener('click', () => {
            navItems.forEach(nav => nav.classList.remove('active'));
            tabPanes.forEach(pane => pane.classList.remove('active'));
            
            item.classList.add('active');
            const targetTab = document.getElementById(item.dataset.tab);
            if (targetTab) targetTab.classList.add('active');

            if (item.dataset.tab === 'tab-history') {
                loadTargetsDropdown();
            } else if (item.dataset.tab === 'tab-overview') {
                loadOverviewData();
            }
        });
    });

    // Chart.js initialization
    const ctx = document.getElementById('rttChart').getContext('2d');
    const gradient = ctx.createLinearGradient(0, 0, 0, 300);
    gradient.addColorStop(0, 'rgba(6, 182, 212, 0.4)');
    gradient.addColorStop(1, 'rgba(6, 182, 212, 0.0)');

    const rttChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: Array(60).fill(''),
            datasets: [{
                label: 'Задержка RTT (мс)',
                data: Array(60).fill(0),
                borderColor: '#06b6d4',
                backgroundColor: gradient,
                borderWidth: 2,
                pointRadius: 0,
                pointHoverRadius: 5,
                pointHoverBackgroundColor: '#06b6d4',
                fill: true,
                tension: 0.3
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: { duration: 200 },
            scales: {
                x: {
                    grid: { color: 'rgba(255, 255, 255, 0.04)' },
                    ticks: { display: false }
                },
                y: {
                    beginAtZero: true,
                    grid: { color: 'rgba(255, 255, 255, 0.04)' },
                    ticks: { color: '#9ca3af', font: { family: 'Inter' } }
                }
            },
            plugins: {
                legend: { display: false },
                tooltip: {
                    mode: 'index',
                    intersect: false,
                    callbacks: {
                        label: (ctx) => `Задержка: ${ctx.raw === -1 ? 'ТАЙМАУТ / ПОТЕРЯ' : ctx.raw + ' мс'}`
                    }
                }
            }
        }
    });

    // Live Monitor state & polling
    let pollInterval = null;

    async function fetchStatus() {
        try {
            const res = await fetch('/api/monitor/status');
            if (!res.ok) return;
            const data = await res.json();
            updateLiveMonitorUI(data);
        } catch (e) {
            console.error('Ошибка получения статуса:', e);
        }
    }

    function updateLiveMonitorUI(data) {
        const banner = document.getElementById('status-banner');
        const statusText = document.getElementById('status-text');
        const bannerTarget = document.getElementById('banner-target');
        const btnStart = document.getElementById('btn-start');
        const btnStop = document.getElementById('btn-stop');

        if (data.is_running) {
            btnStart.disabled = true;
            btnStop.disabled = false;
            bannerTarget.textContent = data.target;

            if (data.last_status === 'success') {
                banner.className = 'status-banner online';
                statusText.textContent = `Связь стабильна (${data.target})`;
            } else if (data.last_status === 'idle') {
                banner.className = 'status-banner online';
                statusText.textContent = `Запуск мониторинга...`;
            } else {
                banner.className = 'status-banner error';
                statusText.textContent = `Сбой связи! Ошибка: ${data.last_error || 'таймаут'}`;
            }
        } else {
            btnStart.disabled = false;
            btnStop.disabled = true;
            banner.className = 'status-banner idle';
            statusText.textContent = 'Мониторинг остановлен';
            if (!data.target) bannerTarget.textContent = '—';
        }

        document.getElementById('metric-rtt').textContent = data.last_status === 'success' ? data.last_rtt_ms.toFixed(1) : '0';
        document.getElementById('metric-uptime').textContent = data.total_sent > 0 ? `${data.uptime_percent.toFixed(2)}%` : '100.00%';
        document.getElementById('metric-sent').textContent = data.total_sent;
        document.getElementById('metric-lost').textContent = data.total_lost;

        if (data.rtt_history && data.rtt_history.length > 0) {
            const padded = Array(60 - data.rtt_history.length).fill(0).concat(data.rtt_history);
            rttChart.data.datasets[0].data = padded.map(val => val < 0 ? 0 : val);
            rttChart.update();
        }
    }

    // Start polling
    fetchStatus();
    pollInterval = setInterval(fetchStatus, 1000);

    // Wire up buttons
    document.getElementById('btn-start').addEventListener('click', async () => {
        const target = document.getElementById('monitor-target').value.trim();
        const interval = parseInt(document.getElementById('monitor-interval').value, 10) || 1;
        if (!target) return alert('Введите IP или хост');

        await fetch('/api/monitor/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ target, interval })
        });
        fetchStatus();
    });

    document.getElementById('btn-stop').addEventListener('click', async () => {
        await fetch('/api/monitor/stop', { method: 'POST' });
        fetchStatus();
    });

    // History Tab logic
    const quickBtns = document.querySelectorAll('.quick-buttons button');
    quickBtns.forEach(btn => {
        btn.addEventListener('click', () => {
            quickBtns.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            document.getElementById('history-from').value = btn.dataset.range;
            document.getElementById('history-to').value = 'now';
        });
    });

    async function loadTargetsDropdown() {
        try {
            const res = await fetch('/api/targets');
            const data = await res.json();
            const select = document.getElementById('history-target');
            select.innerHTML = '<option value="">Выберите устройство...</option>';
            if (data.targets) {
                data.targets.forEach(t => {
                    const opt = document.createElement('option');
                    opt.value = t;
                    opt.textContent = t;
                    select.appendChild(opt);
                });
            }
        } catch (e) {
            console.error('Ошибка загрузки целей:', e);
        }
    }

    document.getElementById('btn-check-history').addEventListener('click', async () => {
        let target = document.getElementById('history-target').value;
        if (!target) target = document.getElementById('monitor-target').value.trim();
        if (!target) return alert('Выберите или укажите целевой IP/хост');

        const from = document.getElementById('history-from').value.trim() || '24h';
        const to = document.getElementById('history-to').value.trim() || 'now';

        try {
            const res = await fetch(`/api/history?target=${encodeURIComponent(target)}&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`);
            if (!res.ok) {
                const err = await res.json();
                return alert(err.error || 'Ошибка загрузки истории');
            }
            const data = await res.json();
            renderHistoryData(data);
        } catch (e) {
            console.error('Ошибка проверки истории:', e);
        }
    });

    function renderHistoryData(data) {
        document.getElementById('history-summary').style.display = 'block';
        const verdictBox = document.getElementById('verdict-box');
        const verdictTitle = document.getElementById('verdict-title');
        const verdictDesc = document.getElementById('verdict-desc');

        if (data.outage_count === 0) {
            verdictBox.className = 'verdict-box success';
            verdictTitle.textContent = '[ОТЛИЧНО] Связь в данном периоде НЕ ТЕРЯЛАСЬ!';
            verdictDesc.textContent = `Отправлено ${data.total_sent} запросов, потерь пакетов нет (Доступность: ${data.uptime_percent.toFixed(2)}%).`;
        } else {
            verdictBox.className = 'verdict-box danger';
            verdictTitle.textContent = `[ВНИМАНИЕ] Обнаружено сбоев связи: ${data.outage_count}!`;
            verdictDesc.textContent = `Потеряно ${data.total_lost} из ${data.total_sent} пакетов. Общее время простоя: ${data.total_outage_time} (Доступность: ${data.uptime_percent.toFixed(2)}%).`;
        }

        document.getElementById('hist-avg-rtt').textContent = `${data.avg_rtt_ms.toFixed(1)} мс`;
        document.getElementById('hist-min-rtt').textContent = `${data.min_rtt_ms.toFixed(1)} мс`;
        document.getElementById('hist-max-rtt').textContent = `${data.max_rtt_ms.toFixed(1)} мс`;
        document.getElementById('hist-downtime').textContent = data.total_outage_time;

        const tbody = document.getElementById('outages-tbody');
        if (data.outages && data.outages.length > 0) {
            tbody.innerHTML = data.outages.map((o, idx) => `
                <tr>
                    <td>${idx + 1}</td>
                    <td><strong>${o.start_time}</strong></td>
                    <td>${o.end_time}</td>
                    <td><span class="badge badge-danger">${o.duration}</span></td>
                    <td>${o.lost_count}</td>
                    <td class="text-secondary">${o.reason || 'Таймаут ICMP ответа'}</td>
                </tr>
            `).join('');
        } else {
            tbody.innerHTML = '<tr><td colspan="6" class="empty-state">Сбоев и отключений в указанном интервале не обнаружено</td></tr>';
        }
    }

    // Overview Tab logic
    document.getElementById('btn-refresh-overview').addEventListener('click', loadOverviewData);

    async function loadOverviewData() {
        const tbody = document.getElementById('overview-tbody');
        tbody.innerHTML = '<tr><td colspan="7" class="empty-state">Загрузка данных...</td></tr>';

        try {
            const res = await fetch('/api/stats/summary');
            const data = await res.json();
            if (!data.summaries || data.summaries.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7" class="empty-state">Нет сохраненных устройств в базе данных</td></tr>';
                return;
            }

            tbody.innerHTML = data.summaries.map(s => `
                <tr>
                    <td><strong>${s.target}</strong></td>
                    <td>${s.total_sent}</td>
                    <td class="${s.total_lost > 0 ? 'text-danger font-weight-bold' : ''}">${s.total_lost}</td>
                    <td>
                        <span class="badge ${s.uptime_percent === 100 ? 'badge-success' : (s.uptime_percent > 95 ? 'badge-warning' : 'badge-danger')}">
                            ${s.uptime_percent.toFixed(2)}%
                        </span>
                    </td>
                    <td>${s.outages}</td>
                    <td>${s.avg_rtt_ms.toFixed(1)} мс</td>
                    <td>
                        <button class="btn btn-outline" style="padding: 4px 8px; font-size: 12px;" onclick="quickMonitor('${s.target}')">⚡ Мониторить</button>
                    </td>
                </tr>
            `).join('');
        } catch (e) {
            console.error('Ошибка загрузки сводки:', e);
            tbody.innerHTML = '<tr><td colspan="7" class="empty-state text-danger">Ошибка загрузки сводки</td></tr>';
        }
    }

    window.quickMonitor = function(target) {
        document.getElementById('monitor-target').value = target;
        document.querySelector('.nav-item[data-tab="tab-monitor"]').click();
    };
});
