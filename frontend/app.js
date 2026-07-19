// ──────────────────────────────────────────────
//  AEGIS Dashboard — Application Logic
// ──────────────────────────────────────────────

// ── Mock Data ──────────────────────────────────
const MOCK_DATA = {
    stats: {
        totalItems: 12847,
        activeItems: 9631,
        draftItems: 2104,
        flaggedItems: 312,
        totalExams: 47,
        activeExams: 3,
        completedExams: 38,
        totalCandidates: 2841500,
        avgReliability: 0.91,
        avgDiscrimination: 1.42,
    },

    items: [
        { id: "ITM-00001", externalId: "PHY-2026-00142", subject: "Physics", chapter: "Electromagnetism", type: "MCQ_SINGLE", status: "ACTIVE", difficulty: "HARD", cognitive: "ANALYZE", irt: { a: 1.85, b: 1.20, c: 0.18 }, pValue: 0.38, discrimination: 0.62, exposure: 12, createdAt: "2025-11-15" },
        { id: "ITM-00002", externalId: "CHM-2026-00089", subject: "Chemistry", chapter: "Organic Chemistry", type: "MCQ_SINGLE", status: "ACTIVE", difficulty: "MEDIUM", cognitive: "APPLY", irt: { a: 1.45, b: 0.30, c: 0.22 }, pValue: 0.55, discrimination: 0.48, exposure: 8, createdAt: "2025-12-03" },
        { id: "ITM-00003", externalId: "MAT-2026-00231", subject: "Mathematics", chapter: "Calculus", type: "INTEGER", status: "REVIEW", difficulty: "VERY_HARD", cognitive: "EVALUATE", irt: null, pValue: null, discrimination: null, exposure: 0, createdAt: "2026-03-20" },
        { id: "ITM-00004", externalId: "PHY-2026-00198", subject: "Physics", chapter: "Optics", type: "MCQ_SINGLE", status: "CALIBRATION", difficulty: "EASY", cognitive: "REMEMBER", irt: null, pValue: 0.82, discrimination: 0.25, exposure: 0, createdAt: "2026-04-10" },
        { id: "ITM-00005", externalId: "BIO-2026-00045", subject: "Biology", chapter: "Genetics", type: "MCQ_MULTI", status: "ACTIVE", difficulty: "MEDIUM", cognitive: "UNDERSTAND", irt: { a: 1.22, b: -0.45, c: 0.15 }, pValue: 0.64, discrimination: 0.51, exposure: 15, createdAt: "2025-09-22" },
        { id: "ITM-00006", externalId: "CHM-2026-00156", subject: "Chemistry", chapter: "Physical Chemistry", type: "MCQ_SINGLE", status: "DRAFT", difficulty: "HARD", cognitive: "APPLY", irt: null, pValue: null, discrimination: null, exposure: 0, createdAt: "2026-06-01" },
        { id: "ITM-00007", externalId: "MAT-2026-00087", subject: "Mathematics", chapter: "Linear Algebra", type: "MCQ_SINGLE", status: "ACTIVE", difficulty: "MEDIUM", cognitive: "APPLY", irt: { a: 1.68, b: 0.10, c: 0.20 }, pValue: 0.52, discrimination: 0.58, exposure: 22, createdAt: "2025-08-14" },
        { id: "ITM-00008", externalId: "PHY-2026-00310", subject: "Physics", chapter: "Thermodynamics", type: "MCQ_SINGLE", status: "ACTIVE", difficulty: "HARD", cognitive: "ANALYZE", irt: { a: 2.10, b: 1.50, c: 0.12 }, pValue: 0.31, discrimination: 0.71, exposure: 5, createdAt: "2026-01-28" },
    ],

    exams: [
        { id: "EXM-001", code: "JEE-MAIN-2026-JAN", name: "JEE Main January 2026", type: "FIXED_FORM", status: "COMPLETED", totalMarks: 300, questions: 75, duration: 180, candidates: 1245000, forms: 8, reliability: 0.92, meanScore: 142.5 },
        { id: "EXM-002", code: "JEE-MAIN-2026-APR", name: "JEE Main April 2026", type: "FIXED_FORM", status: "COMPLETED", totalMarks: 300, questions: 75, duration: 180, candidates: 1180000, forms: 8, reliability: 0.91, meanScore: 138.2 },
        { id: "EXM-003", code: "JEE-ADV-2026", name: "JEE Advanced 2026", type: "FIXED_FORM", status: "ACTIVE", totalMarks: 360, questions: 54, duration: 180, candidates: 250000, forms: 4, reliability: null, meanScore: null },
        { id: "EXM-004", code: "NEET-2026", name: "NEET UG 2026", type: "FIXED_FORM", status: "SCHEDULED", totalMarks: 720, questions: 200, duration: 200, candidates: 2400000, forms: 12, reliability: null, meanScore: null },
        { id: "EXM-005", code: "CUET-2026", name: "CUET UG 2026", type: "LINEAR_ON_FLY", status: "PAPERS_GENERATED", totalMarks: 400, questions: 100, duration: 180, candidates: 1500000, forms: 20, reliability: null, meanScore: null },
    ],

    auditLog: [
        { time: "2026-07-19 21:35:12", type: "ITEM_CALIBRATED", actor: "Dr. Priya Sharma", resource: "PHY-2026-00142", detail: "IRT params set: a=1.85, b=1.20, c=0.18" },
        { time: "2026-07-19 21:30:45", type: "PAPER_GENERATED", actor: "SYSTEM", resource: "JEE-ADV-2026", detail: "4 forms generated, solver: OPTIMAL, gap: 0.001" },
        { time: "2026-07-19 21:22:18", type: "ITEM_REVIEWED", actor: "Prof. Rajesh Kumar", resource: "MAT-2026-00231", detail: "Decision: APPROVED, moved to CALIBRATION" },
        { time: "2026-07-19 21:15:03", type: "DIF_DETECTED", actor: "SYSTEM", resource: "CHM-2026-00089", detail: "Category B DIF detected (gender), Δ_MH = 1.32" },
        { time: "2026-07-19 21:10:30", type: "EXAM_STARTED", actor: "SYSTEM", resource: "JEE-ADV-2026", detail: "Session 1 started, 125,000 candidates online" },
        { time: "2026-07-19 20:55:22", type: "ITEM_FLAGGED", actor: "SYSTEM", resource: "BIO-2026-00045", detail: "Low discrimination: D=0.15, flagged for review" },
        { time: "2026-07-19 20:40:11", type: "PERSON_FIT_FLAGGED", actor: "SYSTEM", resource: "Session-28451", detail: "Lz = -3.42, p < 0.001, aberrant pattern detected" },
    ]
};

// ── Navigation ─────────────────────────────────
function navigateTo(page) {
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    const navItem = document.querySelector(`[data-page="${page}"]`);
    if (navItem) navItem.classList.add('active');

    const titles = {
        dashboard: 'Dashboard',
        items: 'Question Bank',
        blueprints: 'Blueprints',
        exams: 'Examinations',
        papers: 'Generated Papers',
        analytics: 'IRT Analytics',
        audit: 'Audit Log'
    };
    document.getElementById('page-title').textContent = titles[page] || page;

    const renderers = {
        dashboard: renderDashboard,
        items: renderItems,
        exams: renderExams,
        analytics: renderAnalytics,
        audit: renderAudit,
        blueprints: renderBlueprints,
        papers: renderPapers,
    };

    const render = renderers[page] || renderPlaceholder;
    document.getElementById('page-content').innerHTML = '';
    render();
}

function toggleSidebar() {
    document.getElementById('sidebar').classList.toggle('open');
}

// ── Dashboard Page ─────────────────────────────
function renderDashboard() {
    const container = document.getElementById('page-content');
    const s = MOCK_DATA.stats;

    container.innerHTML = `
        <div class="stats-grid">
            <div class="stat-card animate-in animate-delay-1">
                <div class="stat-header">
                    <span class="stat-label">Total Items</span>
                    <div class="stat-icon indigo">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/></svg>
                    </div>
                </div>
                <div class="stat-value">${s.totalItems.toLocaleString()}</div>
                <div class="stat-change positive">↑ 342 this month</div>
            </div>
            <div class="stat-card animate-in animate-delay-2">
                <div class="stat-header">
                    <span class="stat-label">Active Items</span>
                    <div class="stat-icon green">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
                    </div>
                </div>
                <div class="stat-value">${s.activeItems.toLocaleString()}</div>
                <div class="stat-change positive">↑ 128 calibrated</div>
            </div>
            <div class="stat-card animate-in animate-delay-3">
                <div class="stat-header">
                    <span class="stat-label">Flagged Items</span>
                    <div class="stat-icon red">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
                    </div>
                </div>
                <div class="stat-value">${s.flaggedItems}</div>
                <div class="stat-change negative">↓ 18 need review</div>
            </div>
            <div class="stat-card animate-in animate-delay-4">
                <div class="stat-header">
                    <span class="stat-label">Avg Reliability (α)</span>
                    <div class="stat-icon purple">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
                    </div>
                </div>
                <div class="stat-value">${s.avgReliability.toFixed(2)}</div>
                <div class="stat-change positive">↑ Excellent</div>
            </div>
            <div class="stat-card animate-in animate-delay-5">
                <div class="stat-header">
                    <span class="stat-label">Total Candidates</span>
                    <div class="stat-icon amber">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                    </div>
                </div>
                <div class="stat-value">${(s.totalCandidates / 1000000).toFixed(1)}M</div>
                <div class="stat-change positive">Across ${s.totalExams} exams</div>
            </div>
        </div>

        <div class="charts-grid">
            <div class="chart-card animate-in animate-delay-2">
                <div class="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-primary)" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
                    Item Difficulty Distribution (IRT-b)
                </div>
                <div class="chart-body" id="difficulty-chart"></div>
            </div>
            <div class="chart-card animate-in animate-delay-3">
                <div class="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-secondary)" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                    Item Response Curve (3PL)
                </div>
                <div class="irt-display" id="irt-curve"></div>
                <div class="irt-params">
                    <div class="irt-param"><div class="irt-param-label">Discrimination (a)</div><div class="irt-param-value a">1.85</div></div>
                    <div class="irt-param"><div class="irt-param-label">Difficulty (b)</div><div class="irt-param-value b">1.20</div></div>
                    <div class="irt-param"><div class="irt-param-label">Guessing (c)</div><div class="irt-param-value c">0.18</div></div>
                </div>
            </div>
        </div>

        <div class="data-section animate-in animate-delay-4">
            <div class="data-section-header">
                <span class="data-section-title">Recent Activity</span>
                <button class="btn btn-secondary" onclick="navigateTo('audit')">View All</button>
            </div>
            <table class="data-table">
                <thead><tr><th>Time</th><th>Event</th><th>Actor</th><th>Resource</th><th>Detail</th></tr></thead>
                <tbody>
                    ${MOCK_DATA.auditLog.slice(0, 5).map(e => `
                        <tr>
                            <td style="font-family:var(--font-mono);font-size:12px;color:var(--text-tertiary)">${e.time.split(' ')[1]}</td>
                            <td><span class="badge ${getEventBadgeClass(e.type)}"><span class="badge-dot"></span>${formatEventType(e.type)}</span></td>
                            <td><span class="timeline-actor">${e.actor}</span></td>
                            <td style="font-family:var(--font-mono);font-size:13px">${e.resource}</td>
                            <td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${e.detail}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
    `;

    renderDifficultyChart();
    renderIRTCurve(1.85, 1.20, 0.18);
}

// ── Items Page ──────────────────────────────────
function renderItems() {
    const container = document.getElementById('page-content');
    const items = MOCK_DATA.items;

    container.innerHTML = `
        <div class="data-section-header" style="background:none;border:none;padding:0 0 16px 0">
            <div class="filters-row">
                <select class="filter-select" id="filter-status" onchange="filterItems()">
                    <option value="">All Statuses</option>
                    <option value="ACTIVE">Active</option>
                    <option value="DRAFT">Draft</option>
                    <option value="REVIEW">Review</option>
                    <option value="CALIBRATION">Calibration</option>
                    <option value="RETIRED">Retired</option>
                </select>
                <select class="filter-select" id="filter-subject" onchange="filterItems()">
                    <option value="">All Subjects</option>
                    <option value="Physics">Physics</option>
                    <option value="Chemistry">Chemistry</option>
                    <option value="Mathematics">Mathematics</option>
                    <option value="Biology">Biology</option>
                </select>
                <select class="filter-select" id="filter-difficulty" onchange="filterItems()">
                    <option value="">All Difficulties</option>
                    <option value="EASY">Easy</option>
                    <option value="MEDIUM">Medium</option>
                    <option value="HARD">Hard</option>
                    <option value="VERY_HARD">Very Hard</option>
                </select>
            </div>
            <button class="btn btn-primary">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                New Item
            </button>
        </div>

        <div class="data-section">
            <table class="data-table" id="items-table">
                <thead>
                    <tr>
                        <th>External ID</th>
                        <th>Subject</th>
                        <th>Type</th>
                        <th>Status</th>
                        <th>Difficulty</th>
                        <th>IRT-b</th>
                        <th>IRT-a</th>
                        <th>p-value</th>
                        <th>Disc.</th>
                        <th>Exposure</th>
                    </tr>
                </thead>
                <tbody id="items-tbody">
                    ${renderItemRows(items)}
                </tbody>
            </table>
        </div>
    `;
}

function renderItemRows(items) {
    return items.map(item => `
        <tr class="animate-in" style="cursor:pointer" onclick="showItemDetail('${item.id}')">
            <td style="font-family:var(--font-mono);font-size:13px;color:var(--accent-primary)">${item.externalId}</td>
            <td>${item.subject}</td>
            <td style="font-size:12px">${item.type.replace('_', ' ')}</td>
            <td><span class="badge ${item.status.toLowerCase()}"><span class="badge-dot"></span>${item.status}</span></td>
            <td>${item.difficulty.replace('_', ' ')}</td>
            <td style="font-family:var(--font-mono)">${item.irt ? item.irt.b.toFixed(2) : '—'}</td>
            <td style="font-family:var(--font-mono)">${item.irt ? item.irt.a.toFixed(2) : '—'}</td>
            <td style="font-family:var(--font-mono)">${item.pValue !== null ? item.pValue.toFixed(2) : '—'}</td>
            <td style="font-family:var(--font-mono)">${item.discrimination !== null ? item.discrimination.toFixed(2) : '—'}</td>
            <td>${item.exposure}</td>
        </tr>
    `).join('');
}

function filterItems() {
    const status = document.getElementById('filter-status').value;
    const subject = document.getElementById('filter-subject').value;
    const difficulty = document.getElementById('filter-difficulty').value;

    let filtered = MOCK_DATA.items;
    if (status) filtered = filtered.filter(i => i.status === status);
    if (subject) filtered = filtered.filter(i => i.subject === subject);
    if (difficulty) filtered = filtered.filter(i => i.difficulty === difficulty);

    document.getElementById('items-tbody').innerHTML = renderItemRows(filtered);
}

function showItemDetail(itemId) {
    const item = MOCK_DATA.items.find(i => i.id === itemId);
    if (!item) return;
    // In production, this would open a detail modal or navigate to item detail page
    alert(`Item: ${item.externalId}\nSubject: ${item.subject}\nStatus: ${item.status}\nIRT: a=${item.irt?.a || 'N/A'}, b=${item.irt?.b || 'N/A'}, c=${item.irt?.c || 'N/A'}`);
}

// ── Exams Page ──────────────────────────────────
function renderExams() {
    const container = document.getElementById('page-content');
    const exams = MOCK_DATA.exams;

    container.innerHTML = `
        <div class="stats-grid" style="margin-bottom:20px">
            <div class="stat-card animate-in animate-delay-1">
                <div class="stat-header"><span class="stat-label">Active Exams</span><div class="stat-icon green"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></div></div>
                <div class="stat-value">${exams.filter(e => e.status === 'ACTIVE').length}</div>
            </div>
            <div class="stat-card animate-in animate-delay-2">
                <div class="stat-header"><span class="stat-label">Scheduled</span><div class="stat-icon amber"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg></div></div>
                <div class="stat-value">${exams.filter(e => e.status === 'SCHEDULED').length}</div>
            </div>
            <div class="stat-card animate-in animate-delay-3">
                <div class="stat-header"><span class="stat-label">Completed</span><div class="stat-icon indigo"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg></div></div>
                <div class="stat-value">${exams.filter(e => e.status === 'COMPLETED').length}</div>
            </div>
        </div>

        <div class="data-section animate-in animate-delay-4">
            <div class="data-section-header">
                <span class="data-section-title">All Examinations</span>
                <button class="btn btn-primary"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>New Exam</button>
            </div>
            <table class="data-table">
                <thead><tr><th>Code</th><th>Name</th><th>Type</th><th>Status</th><th>Marks</th><th>Questions</th><th>Duration</th><th>Candidates</th><th>Forms</th><th>Reliability</th></tr></thead>
                <tbody>
                    ${exams.map(e => `
                        <tr>
                            <td style="font-family:var(--font-mono);font-size:13px;color:var(--accent-primary)">${e.code}</td>
                            <td style="font-weight:500">${e.name}</td>
                            <td style="font-size:12px">${e.type.replace(/_/g, ' ')}</td>
                            <td><span class="badge ${e.status.toLowerCase()}"><span class="badge-dot"></span>${e.status}</span></td>
                            <td>${e.totalMarks}</td>
                            <td>${e.questions}</td>
                            <td>${e.duration} min</td>
                            <td>${e.candidates ? (e.candidates / 1000).toFixed(0) + 'K' : '—'}</td>
                            <td>${e.forms}</td>
                            <td style="font-family:var(--font-mono)">${e.reliability ? e.reliability.toFixed(2) : '—'}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
    `;
}

// ── Analytics Page ──────────────────────────────
function renderAnalytics() {
    const container = document.getElementById('page-content');

    container.innerHTML = `
        <div class="charts-grid">
            <div class="chart-card animate-in animate-delay-1">
                <div class="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-secondary)" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                    Test Information Function (TIF)
                </div>
                <div class="irt-display" id="tif-chart"></div>
            </div>
            <div class="chart-card animate-in animate-delay-2">
                <div class="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-green)" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
                    Score Distribution — JEE Main Jan 2026
                </div>
                <div class="chart-body" id="score-dist-chart"></div>
            </div>
            <div class="chart-card animate-in animate-delay-3">
                <div class="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-amber)" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
                    DIF Analysis Summary
                </div>
                <div style="padding:10px 0">
                    <table class="data-table">
                        <thead><tr><th>Item</th><th>Grouping</th><th>Δ_MH</th><th>Category</th><th>p-value</th></tr></thead>
                        <tbody>
                            <tr><td style="font-family:var(--font-mono)">CHM-2026-00089</td><td>Gender</td><td style="font-family:var(--font-mono)">1.32</td><td><span class="badge draft"><span class="badge-dot"></span>B</span></td><td style="font-family:var(--font-mono)">0.012</td></tr>
                            <tr><td style="font-family:var(--font-mono)">PHY-2026-00142</td><td>Language</td><td style="font-family:var(--font-mono)">0.45</td><td><span class="badge active"><span class="badge-dot"></span>A</span></td><td style="font-family:var(--font-mono)">0.342</td></tr>
                            <tr><td style="font-family:var(--font-mono)">BIO-2026-00045</td><td>Gender</td><td style="font-family:var(--font-mono)">1.78</td><td><span class="badge retired"><span class="badge-dot"></span>C</span></td><td style="font-family:var(--font-mono)">0.001</td></tr>
                            <tr><td style="font-family:var(--font-mono)">MAT-2026-00087</td><td>Language</td><td style="font-family:var(--font-mono)">0.22</td><td><span class="badge active"><span class="badge-dot"></span>A</span></td><td style="font-family:var(--font-mono)">0.614</td></tr>
                        </tbody>
                    </table>
                </div>
            </div>
            <div class="chart-card animate-in animate-delay-4">
                <div class="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-red)" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
                    Person-Fit Flags (Lz < -2.0)
                </div>
                <div style="padding:10px 0">
                    <table class="data-table">
                        <thead><tr><th>Session</th><th>Candidate</th><th>Lz</th><th>p-value</th><th>Reason</th></tr></thead>
                        <tbody>
                            <tr><td style="font-family:var(--font-mono)">S-28451</td><td>CND-****-7823</td><td style="font-family:var(--font-mono);color:var(--accent-red)">-3.42</td><td style="font-family:var(--font-mono)">&lt;0.001</td><td>Hard items correct, easy items wrong</td></tr>
                            <tr><td style="font-family:var(--font-mono)">S-31024</td><td>CND-****-1456</td><td style="font-family:var(--font-mono);color:var(--accent-red)">-2.87</td><td style="font-family:var(--font-mono)">0.002</td><td>Unexpected response pattern</td></tr>
                            <tr><td style="font-family:var(--font-mono)">S-29183</td><td>CND-****-9012</td><td style="font-family:var(--font-mono);color:var(--accent-amber)">-2.14</td><td style="font-family:var(--font-mono)">0.016</td><td>Rapid guessing on final 20 items</td></tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    `;

    renderTIFChart();
    renderScoreDistChart();
}

// ── Audit Page ──────────────────────────────────
function renderAudit() {
    const container = document.getElementById('page-content');

    container.innerHTML = `
        <div class="data-section animate-in">
            <div class="data-section-header">
                <div class="filters-row">
                    <select class="filter-select"><option>All Events</option><option>ITEM_*</option><option>EXAM_*</option><option>SESSION_*</option><option>SCORING_*</option></select>
                    <select class="filter-select"><option>All Actors</option><option>SYSTEM</option><option>USER</option></select>
                </div>
                <button class="btn btn-secondary">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                    Export
                </button>
            </div>
            <div style="padding:20px">
                <div class="timeline">
                    ${MOCK_DATA.auditLog.map(e => `
                        <div class="timeline-item">
                            <div class="timeline-dot" style="background:${getEventColor(e.type)}"></div>
                            <div class="timeline-time">${e.time}</div>
                            <div class="timeline-event">
                                <span class="badge ${getEventBadgeClass(e.type)}" style="margin-right:8px"><span class="badge-dot"></span>${formatEventType(e.type)}</span>
                                by <span class="timeline-actor">${e.actor}</span>
                                on <span style="font-family:var(--font-mono);color:var(--accent-primary)">${e.resource}</span>
                            </div>
                            <div style="margin-top:4px;font-size:13px;color:var(--text-tertiary)">${e.detail}</div>
                        </div>
                    `).join('')}
                </div>
            </div>
        </div>
    `;
}

// ── Blueprints Page ─────────────────────────────
function renderBlueprints() {
    const container = document.getElementById('page-content');
    container.innerHTML = `
        <div class="data-section-header" style="background:none;border:none;padding:0 0 16px 0">
            <span></span>
            <button class="btn btn-primary"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>New Blueprint</button>
        </div>
        <div class="data-section animate-in">
            <table class="data-table">
                <thead><tr><th>Name</th><th>Subject</th><th>Items</th><th>Chapters</th><th>Status</th><th>Time Budget</th><th>Version</th></tr></thead>
                <tbody>
                    <tr><td style="font-weight:600">JEE Main Physics Blueprint</td><td>Physics</td><td>25</td><td>8</td><td><span class="badge active"><span class="badge-dot"></span>ACTIVE</span></td><td>60 min</td><td>v3</td></tr>
                    <tr><td style="font-weight:600">JEE Main Chemistry Blueprint</td><td>Chemistry</td><td>25</td><td>7</td><td><span class="badge active"><span class="badge-dot"></span>ACTIVE</span></td><td>60 min</td><td>v2</td></tr>
                    <tr><td style="font-weight:600">JEE Main Mathematics Blueprint</td><td>Mathematics</td><td>25</td><td>9</td><td><span class="badge active"><span class="badge-dot"></span>ACTIVE</span></td><td>60 min</td><td>v3</td></tr>
                    <tr><td style="font-weight:600">NEET Biology Blueprint</td><td>Biology</td><td>90</td><td>15</td><td><span class="badge draft"><span class="badge-dot"></span>DRAFT</span></td><td>100 min</td><td>v1</td></tr>
                    <tr><td style="font-weight:600">CUET General Test Blueprint</td><td>General</td><td>60</td><td>5</td><td><span class="badge draft"><span class="badge-dot"></span>DRAFT</span></td><td>60 min</td><td>v1</td></tr>
                </tbody>
            </table>
        </div>
    `;
}

// ── Papers Page ─────────────────────────────────
function renderPapers() {
    const container = document.getElementById('page-content');
    container.innerHTML = `
        <div class="data-section animate-in">
            <div class="data-section-header">
                <span class="data-section-title">Generated Papers</span>
            </div>
            <table class="data-table">
                <thead><tr><th>Paper Code</th><th>Exam</th><th>Form</th><th>Items</th><th>Mean b</th><th>Std b</th><th>TIF(0)</th><th>Reliability</th><th>Solver</th><th>Generated</th></tr></thead>
                <tbody>
                    <tr><td style="font-family:var(--font-mono)">JEE-ADV-2026-F1</td><td>JEE Advanced 2026</td><td>Form 1</td><td>54</td><td style="font-family:var(--font-mono)">0.85</td><td style="font-family:var(--font-mono)">1.12</td><td style="font-family:var(--font-mono)">24.5</td><td style="font-family:var(--font-mono)">0.96</td><td><span class="badge active"><span class="badge-dot"></span>OPTIMAL</span></td><td>2026-07-18</td></tr>
                    <tr><td style="font-family:var(--font-mono)">JEE-ADV-2026-F2</td><td>JEE Advanced 2026</td><td>Form 2</td><td>54</td><td style="font-family:var(--font-mono)">0.82</td><td style="font-family:var(--font-mono)">1.08</td><td style="font-family:var(--font-mono)">23.8</td><td style="font-family:var(--font-mono)">0.95</td><td><span class="badge active"><span class="badge-dot"></span>OPTIMAL</span></td><td>2026-07-18</td></tr>
                    <tr><td style="font-family:var(--font-mono)">JEE-ADV-2026-F3</td><td>JEE Advanced 2026</td><td>Form 3</td><td>54</td><td style="font-family:var(--font-mono)">0.88</td><td style="font-family:var(--font-mono)">1.15</td><td style="font-family:var(--font-mono)">25.1</td><td style="font-family:var(--font-mono)">0.96</td><td><span class="badge active"><span class="badge-dot"></span>OPTIMAL</span></td><td>2026-07-18</td></tr>
                    <tr><td style="font-family:var(--font-mono)">JEE-ADV-2026-F4</td><td>JEE Advanced 2026</td><td>Form 4</td><td>54</td><td style="font-family:var(--font-mono)">0.80</td><td style="font-family:var(--font-mono)">1.10</td><td style="font-family:var(--font-mono)">24.0</td><td style="font-family:var(--font-mono)">0.95</td><td><span class="badge active"><span class="badge-dot"></span>OPTIMAL</span></td><td>2026-07-18</td></tr>
                </tbody>
            </table>
        </div>
    `;
}

function renderPlaceholder() {
    document.getElementById('page-content').innerHTML = '<div style="padding:40px;text-align:center;color:var(--text-tertiary)">Page under construction</div>';
}

// ── Chart Renderers ─────────────────────────────

function renderDifficultyChart() {
    const chart = document.getElementById('difficulty-chart');
    if (!chart) return;

    const bins = [
        { label: '-3', count: 120, color: '#10b981' },
        { label: '-2', count: 450, color: '#10b981' },
        { label: '-1', count: 1200, color: '#06b6d4' },
        { label: '0', count: 2800, color: '#6366f1' },
        { label: '1', count: 2200, color: '#6366f1' },
        { label: '2', count: 1500, color: '#f59e0b' },
        { label: '3', count: 350, color: '#ef4444' },
    ];
    const maxCount = Math.max(...bins.map(b => b.count));

    chart.innerHTML = bins.map(b => `
        <div class="bar-group">
            <div class="bar" style="height:${(b.count / maxCount) * 190}px;background:${b.color}"></div>
            <span class="bar-label">${b.label}</span>
        </div>
    `).join('');
}

function renderIRTCurve(a, b, c) {
    const el = document.getElementById('irt-curve');
    if (!el) return;

    const W = 500, H = 220;
    let path = '';
    for (let i = 0; i <= W; i++) {
        const theta = -4 + (i / W) * 8;
        const p = c + (1 - c) / (1 + Math.exp(-a * (theta - b)));
        const y = H - (p * H * 0.9) - H * 0.05;
        path += (i === 0 ? 'M' : 'L') + `${i},${y.toFixed(1)} `;
    }

    el.innerHTML = `
        <svg class="irt-svg" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none">
            <defs>
                <linearGradient id="curveGrad" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0%" stop-color="#6366f1" stop-opacity="0.3"/>
                    <stop offset="50%" stop-color="#06b6d4" stop-opacity="0.8"/>
                    <stop offset="100%" stop-color="#10b981" stop-opacity="0.3"/>
                </linearGradient>
            </defs>
            <!-- Grid -->
            <line x1="0" y1="${H*0.05}" x2="${W}" y2="${H*0.05}" stroke="rgba(255,255,255,0.05)" />
            <line x1="0" y1="${H*0.5}" x2="${W}" y2="${H*0.5}" stroke="rgba(255,255,255,0.05)" />
            <line x1="0" y1="${H*0.95}" x2="${W}" y2="${H*0.95}" stroke="rgba(255,255,255,0.08)" />
            <line x1="${W/2}" y1="0" x2="${W/2}" y2="${H}" stroke="rgba(255,255,255,0.05)" />
            <!-- Labels -->
            <text x="4" y="${H*0.05+12}" fill="rgba(255,255,255,0.3)" font-size="10" font-family="var(--font-mono)">1.0</text>
            <text x="4" y="${H*0.5+4}" fill="rgba(255,255,255,0.3)" font-size="10" font-family="var(--font-mono)">0.5</text>
            <text x="4" y="${H*0.95-2}" fill="rgba(255,255,255,0.3)" font-size="10" font-family="var(--font-mono)">0.0</text>
            <text x="${W/2-4}" y="${H-2}" fill="rgba(255,255,255,0.3)" font-size="10" font-family="var(--font-mono)">θ=0</text>
            <!-- Guessing line -->
            <line x1="0" y1="${H - (c * H * 0.9) - H*0.05}" x2="${W}" y2="${H - (c * H * 0.9) - H*0.05}" stroke="rgba(16,185,129,0.3)" stroke-dasharray="4,4" />
            <!-- Curve -->
            <path d="${path}" fill="none" stroke="url(#curveGrad)" stroke-width="3" />
            <!-- b marker -->
            <circle cx="${((b + 4) / 8) * W}" cy="${H - (0.5 * (1-c) + c) * H * 0.9 - H*0.05}" r="5" fill="#f59e0b" stroke="rgba(0,0,0,0.3)" stroke-width="1.5"/>
        </svg>
    `;
}

function renderTIFChart() {
    const el = document.getElementById('tif-chart');
    if (!el) return;

    const W = 500, H = 220;
    const items = MOCK_DATA.items.filter(i => i.irt);

    let path = '';
    for (let i = 0; i <= W; i++) {
        const theta = -4 + (i / W) * 8;
        let totalInfo = 0;
        items.forEach(item => {
            const { a, b, c } = item.irt;
            const p = c + (1 - c) / (1 + Math.exp(-a * (theta - b)));
            const q = 1 - p;
            if (p > 0 && q > 0) {
                totalInfo += (a * a * Math.pow(p - c, 2) * q) / (Math.pow(1 - c, 2) * p);
            }
        });
        const y = H - (totalInfo / 8) * H * 0.85 - H * 0.05;
        path += (i === 0 ? 'M' : 'L') + `${i},${Math.max(0, y).toFixed(1)} `;
    }

    el.innerHTML = `
        <svg class="irt-svg" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none">
            <defs>
                <linearGradient id="tifGrad" x1="0" y1="1" x2="0" y2="0">
                    <stop offset="0%" stop-color="#06b6d4" stop-opacity="0"/>
                    <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.2"/>
                </linearGradient>
            </defs>
            <line x1="${W/2}" y1="0" x2="${W/2}" y2="${H}" stroke="rgba(255,255,255,0.05)"/>
            <line x1="0" y1="${H*0.95}" x2="${W}" y2="${H*0.95}" stroke="rgba(255,255,255,0.08)"/>
            <text x="${W/2-4}" y="${H-2}" fill="rgba(255,255,255,0.3)" font-size="10" font-family="var(--font-mono)">θ=0</text>
            <text x="4" y="14" fill="rgba(255,255,255,0.3)" font-size="10" font-family="var(--font-mono)">I(θ)</text>
            <path d="${path}L${W},${H} L0,${H} Z" fill="url(#tifGrad)"/>
            <path d="${path}" fill="none" stroke="#06b6d4" stroke-width="2.5"/>
        </svg>
    `;
}

function renderScoreDistChart() {
    const chart = document.getElementById('score-dist-chart');
    if (!chart) return;

    // Simulated normal-ish distribution
    const bins = [
        { label: '0-30', count: 45000 },
        { label: '30-60', count: 89000 },
        { label: '60-90', count: 145000 },
        { label: '90-120', count: 210000 },
        { label: '120-150', count: 280000 },
        { label: '150-180', count: 195000 },
        { label: '180-210', count: 135000 },
        { label: '210-240', count: 78000 },
        { label: '240-270', count: 42000 },
        { label: '270-300', count: 18000 },
    ];
    const maxCount = Math.max(...bins.map(b => b.count));

    chart.innerHTML = bins.map((b, i) => {
        const intensity = Math.round(180 + (i / bins.length) * 75);
        return `
            <div class="bar-group">
                <div class="bar" style="height:${(b.count / maxCount) * 190}px;background:linear-gradient(180deg, hsl(${230 + i*8}, 70%, ${40+i*3}%) 0%, hsl(${230 + i*8}, 60%, 25%) 100%)"></div>
                <span class="bar-label">${b.label.split('-')[0]}</span>
            </div>
        `;
    }).join('');
}

// ── Utility Functions ───────────────────────────

function formatEventType(type) {
    return type.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase()).replace(/^(\w+)/, (m) => m);
}

function getEventBadgeClass(type) {
    if (type.includes('CREATED') || type.includes('STARTED') || type.includes('ACTIVATED')) return 'active';
    if (type.includes('REVIEWED') || type.includes('CALIBRATED') || type.includes('GENERATED')) return 'review';
    if (type.includes('FLAGGED') || type.includes('DIF_DETECTED')) return 'draft';
    if (type.includes('TERMINATED') || type.includes('PERSON_FIT')) return 'retired';
    if (type.includes('COMPLETED')) return 'completed';
    return 'review';
}

function getEventColor(type) {
    if (type.includes('FLAGGED') || type.includes('DIF') || type.includes('PERSON_FIT')) return '#ef4444';
    if (type.includes('CREATED') || type.includes('ACTIVATED') || type.includes('STARTED')) return '#10b981';
    if (type.includes('GENERATED') || type.includes('CALIBRATED')) return '#6366f1';
    if (type.includes('REVIEWED')) return '#06b6d4';
    return '#6366f1';
}

// ── Initialize ──────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    navigateTo('dashboard');
});
