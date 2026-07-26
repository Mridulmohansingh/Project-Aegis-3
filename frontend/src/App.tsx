import React, { useState, useEffect } from 'react';

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
    { id: "ITM-00006", externalId: "CHM-2026-00156", subject: "Chemistry", chapter: "Physical Chemistry", type: "DRAFT", status: "DRAFT", difficulty: "HARD", cognitive: "APPLY", irt: null, pValue: null, discrimination: null, exposure: 0, createdAt: "2026-06-01" },
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

export default function App() {
  const [currentPage, setCurrentPage] = useState<string>('dashboard');
  const [sidebarOpen, setSidebarOpen] = useState<boolean>(false);
  const [filterStatus, setFilterStatus] = useState<string>('');
  const [filterSubject, setFilterSubject] = useState<string>('');
  const [filterDifficulty, setFilterDifficulty] = useState<string>('');

  const navigateTo = (page: string) => {
    setCurrentPage(page);
    setSidebarOpen(false);
  };

  const getEventBadgeClass = (type: string) => {
    if (type.includes('CREATED') || type.includes('STARTED') || type.includes('ACTIVATED')) return 'active';
    if (type.includes('REVIEWED') || type.includes('CALIBRATED') || type.includes('GENERATED')) return 'review';
    if (type.includes('FLAGGED') || type.includes('DIF_DETECTED')) return 'draft';
    if (type.includes('TERMINATED') || type.includes('PERSON_FIT')) return 'retired';
    if (type.includes('COMPLETED')) return 'completed';
    return 'review';
  };

  const getEventColor = (type: string) => {
    if (type.includes('FLAGGED') || type.includes('DIF') || type.includes('PERSON_FIT')) return '#ef4444';
    if (type.includes('CREATED') || type.includes('ACTIVATED') || type.includes('STARTED')) return '#10b981';
    if (type.includes('GENERATED') || type.includes('CALIBRATED')) return '#6366f1';
    if (type.includes('REVIEWED')) return '#06b6d4';
    return '#6366f1';
  };

  const formatEventType = (type: string) => {
    return type.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
  };

  // ── Difficulty Chart Data ───────────────────
  const difficultyBins = [
    { label: '-3', count: 120, color: '#10b981' },
    { label: '-2', count: 450, color: '#10b981' },
    { label: '-1', count: 1200, color: '#06b6d4' },
    { label: '0', count: 2800, color: '#6366f1' },
    { label: '1', count: 2200, color: '#6366f1' },
    { label: '2', count: 1500, color: '#f59e0b' },
    { label: '3', count: 350, color: '#ef4444' },
  ];
  const maxDifficultyCount = Math.max(...difficultyBins.map(b => b.count));

  // ── Score Dist Chart Data ────────────────────
  const scoreBins = [
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
  const maxScoreCount = Math.max(...scoreBins.map(b => b.count));

  // ── IRT Curve SVG generator ────────────────
  const renderIRTCurveSVG = (a: number, b: number, c: number) => {
    const W = 500, H = 220;
    let path = '';
    for (let i = 0; i <= W; i++) {
      const theta = -4 + (i / W) * 8;
      const p = c + (1 - c) / (1 + Math.exp(-a * (theta - b)));
      const y = H - (p * H * 0.9) - H * 0.05;
      path += (i === 0 ? 'M' : 'L') + `${i},${y.toFixed(1)} `;
    }
    const bX = ((b + 4) / 8) * W;
    const bY = H - (0.5 * (1 - c) + c) * H * 0.9 - H * 0.05;

    return (
      <svg className="irt-svg" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" style={{ width: '100%', height: '100%' }}>
        <defs>
          <linearGradient id="curveGrad" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="#6366f1" stopOpacity="0.3"/>
            <stop offset="50%" stopColor="#06b6d4" stopOpacity="0.8"/>
            <stop offset="100%" stopColor="#10b981" stopOpacity="0.3"/>
          </linearGradient>
        </defs>
        <line x1="0" y1={H * 0.05} x2={W} y2={H * 0.05} stroke="rgba(255,255,255,0.05)" />
        <line x1="0" y1={H * 0.5} x2={W} y2={H * 0.5} stroke="rgba(255,255,255,0.05)" />
        <line x1="0" y1={H * 0.95} x2={W} y2={H * 0.95} stroke="rgba(255,255,255,0.08)" />
        <line x1={W / 2} y1="0" x2={W / 2} y2={H} stroke="rgba(255,255,255,0.05)" />
        <text x="4" y={H * 0.05 + 12} fill="rgba(255,255,255,0.3)" fontSize="10" fontFamily="var(--font-mono)">1.0</text>
        <text x="4" y={H * 0.5 + 4} fill="rgba(255,255,255,0.3)" fontSize="10" fontFamily="var(--font-mono)">0.5</text>
        <text x="4" y={H * 0.95 - 2} fill="rgba(255,255,255,0.3)" fontSize="10" fontFamily="var(--font-mono)">0.0</text>
        <text x={W / 2 - 4} y={H - 2} fill="rgba(255,255,255,0.3)" fontSize="10" fontFamily="var(--font-mono)">θ=0</text>
        <line x1="0" y1={H - (c * H * 0.9) - H * 0.05} x2={W} y2={H - (c * H * 0.9) - H * 0.05} stroke="rgba(16,185,129,0.3)" strokeDasharray="4,4" />
        <path d={path} fill="none" stroke="url(#curveGrad)" strokeWidth="3" />
        <circle cx={bX} cy={bY} r="5" fill="#f59e0b" stroke="rgba(0,0,0,0.3)" strokeWidth="1.5"/>
      </svg>
    );
  };

  // ── TIF Curve SVG generator ──────────────────
  const renderTIFCurveSVG = () => {
    const W = 500, H = 220;
    const items = MOCK_DATA.items.filter(i => i.irt);
    let path = '';
    for (let i = 0; i <= W; i++) {
      const theta = -4 + (i / W) * 8;
      let totalInfo = 0;
      items.forEach(item => {
        if (item.irt) {
          const { a, b, c } = item.irt;
          const p = c + (1 - c) / (1 + Math.exp(-a * (theta - b)));
          const q = 1 - p;
          if (p > 0 && q > 0) {
            totalInfo += (a * a * Math.pow(p - c, 2) * q) / (Math.pow(1 - c, 2) * p);
          }
        }
      });
      const y = H - (totalInfo / 8) * H * 0.85 - H * 0.05;
      path += (i === 0 ? 'M' : 'L') + `${i},${Math.max(0, y).toFixed(1)} `;
    }

    return (
      <svg className="irt-svg" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" style={{ width: '100%', height: '100%' }}>
        <defs>
          <linearGradient id="tifGrad" x1="0" y1="1" x2="0" y2="0">
            <stop offset="0%" stopColor="#06b6d4" stopOpacity="0"/>
            <stop offset="100%" stopColor="#06b6d4" stopOpacity="0.2"/>
          </linearGradient>
        </defs>
        <line x1={W / 2} y1="0" x2={W / 2} y2={H} stroke="rgba(255,255,255,0.05)"/>
        <line x1="0" y1={H * 0.95} x2={W} y2={H * 0.95} stroke="rgba(255,255,255,0.08)"/>
        <text x={W / 2 - 4} y={H - 2} fill="rgba(255,255,255,0.3)" fontSize="10" fontFamily="var(--font-mono)">θ=0</text>
        <text x="4" y="14" fill="rgba(255,255,255,0.3)" fontSize="10" fontFamily="var(--font-mono)">I(θ)</text>
        <path d={`${path}L${W},${H} L0,${H} Z`} fill="url(#tifGrad)"/>
        <path d={path} fill="none" stroke="#06b6d4" strokeWidth="2.5"/>
      </svg>
    );
  };

  const filteredItems = MOCK_DATA.items.filter(item => {
    if (filterStatus && item.status !== filterStatus) return false;
    if (filterSubject && item.subject !== filterSubject) return false;
    if (filterDifficulty && item.difficulty !== filterDifficulty) return false;
    return true;
  });

  return (
    <>
      {/* Sidebar */}
      <aside className={`sidebar ${sidebarOpen ? 'open' : ''}`} id="sidebar">
        <div className="sidebar-header">
          <div className="logo">
            <div className="logo-icon">Æ</div>
            <div className="logo-text">
              <span className="logo-name">AEGIS</span>
              <span className="logo-sub">NDAP v1.0</span>
            </div>
          </div>
        </div>
        <nav className="sidebar-nav">
          <div className="nav-section-label">Overview</div>
          <div className={`nav-item ${currentPage === 'dashboard' ? 'active' : ''}`} onClick={() => navigateTo('dashboard')}>
            <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>
            Dashboard
          </div>
          <div className="nav-section-label">Question Bank</div>
          <div className={`nav-item ${currentPage === 'items' ? 'active' : ''}`} onClick={() => navigateTo('items')}>
            <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>
            Items
          </div>
          <div className={`nav-item ${currentPage === 'blueprints' ? 'active' : ''}`} onClick={() => navigateTo('blueprints')}>
            <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>
            Blueprints
          </div>
          <div className="nav-section-label">Examinations</div>
          <div className={`nav-item ${currentPage === 'exams' ? 'active' : ''}`} onClick={() => navigateTo('exams')}>
            <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
            Exams
          </div>
          <div className={`nav-item ${currentPage === 'papers' ? 'active' : ''}`} onClick={() => navigateTo('papers')}>
            <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>
            Papers
          </div>
          <div className="nav-section-label">Analytics</div>
          <div className={`nav-item ${currentPage === 'analytics' ? 'active' : ''}`} onClick={() => navigateTo('analytics')}>
            <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
            IRT Analytics
          </div>
          <div className={`nav-item ${currentPage === 'audit' ? 'active' : ''}`} onClick={() => navigateTo('audit')}>
            <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
            Audit Log
          </div>
        </nav>
        <div class="sidebar-footer">
          <div class="user-info">
            <div class="user-avatar">MS</div>
            <div class="user-details">
              <span class="user-name">Mridul Singh</span>
              <span class="user-role">Super Admin</span>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="main-content">
        <header className="top-bar">
          <div className="top-bar-left">
            <button className="menu-toggle" onClick={() => setSidebarOpen(!sidebarOpen)}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="22" height="22"><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
            </button>
            <h1 className="page-title">
              {currentPage.charAt(0).toUpperCase() + currentPage.slice(1)}
            </h1>
          </div>
          <div className="top-bar-right">
            <div className="search-box">
              <svg className="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
              <input type="text" placeholder="Search items, exams..." />
            </div>
            <button className="icon-btn notification-btn">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
              <span className="notification-badge">3</span>
            </button>
          </div>
        </header>

        <div className="page-content">
          {/* Dashboard Page */}
          {currentPage === 'dashboard' && (
            <div>
              <div className="stats-grid">
                <div className="stat-card animate-in animate-delay-1">
                  <div className="stat-header">
                    <span className="stat-label">Total Items</span>
                    <div className="stat-icon indigo">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/></svg>
                    </div>
                  </div>
                  <div className="stat-value">{MOCK_DATA.stats.totalItems.toLocaleString()}</div>
                  <div className="stat-change positive">↑ 342 this month</div>
                </div>
                <div className="stat-card animate-in animate-delay-2">
                  <div className="stat-header">
                    <span className="stat-label">Active Items</span>
                    <div className="stat-icon green">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
                    </div>
                  </div>
                  <div className="stat-value">{MOCK_DATA.stats.activeItems.toLocaleString()}</div>
                  <div className="stat-change positive">↑ 128 calibrated</div>
                </div>
                <div className="stat-card animate-in animate-delay-3">
                  <div className="stat-header">
                    <span className="stat-label">Flagged Items</span>
                    <div className="stat-icon red">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
                    </div>
                  </div>
                  <div className="stat-value">{MOCK_DATA.stats.flaggedItems}</div>
                  <div className="stat-change negative">↓ 18 need review</div>
                </div>
                <div className="stat-card animate-in animate-delay-4">
                  <div className="stat-header">
                    <span className="stat-label">Avg Reliability (α)</span>
                    <div className="stat-icon purple">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
                    </div>
                  </div>
                  <div className="stat-value">{MOCK_DATA.stats.avgReliability.toFixed(2)}</div>
                  <div className="stat-change positive">↑ Excellent</div>
                </div>
                <div className="stat-card animate-in animate-delay-5">
                  <div className="stat-header">
                    <span className="stat-label">Total Candidates</span>
                    <div className="stat-icon amber">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                    </div>
                  </div>
                  <div className="stat-value">{(MOCK_DATA.stats.totalCandidates / 1000000).toFixed(1)}M</div>
                  <div className="stat-change positive">Across {MOCK_DATA.stats.totalExams} exams</div>
                </div>
              </div>

              <div className="charts-grid">
                <div className="chart-card animate-in animate-delay-2">
                  <div className="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-primary)" stroke-width="2" style={{ marginRight: 8 }}><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
                    Item Difficulty Distribution (IRT-b)
                  </div>
                  <div className="chart-body">
                    {difficultyBins.map((b, idx) => (
                      <div key={idx} className="bar-group">
                        <div className="bar" style={{ height: `${(b.count / maxDifficultyCount) * 190}px`, background: b.color }} />
                        <span className="bar-label">{b.label}</span>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="chart-card animate-in animate-delay-3">
                  <div className="chart-title">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-secondary)" stroke-width="2" style={{ marginRight: 8 }}><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                    Item Response Curve (3PL)
                  </div>
                  <div className="irt-display">
                    {renderIRTCurveSVG(1.85, 1.20, 0.18)}
                  </div>
                  <div className="irt-params">
                    <div className="irt-param"><div className="irt-param-label">Discrimination (a)</div><div className="irt-param-value a">1.85</div></div>
                    <div className="irt-param"><div className="irt-param-label">Difficulty (b)</div><div className="irt-param-value b">1.20</div></div>
                    <div className="irt-param"><div className="irt-param-label">Guessing (c)</div><div className="irt-param-value c">0.18</div></div>
                  </div>
                </div>
              </div>

              <div className="data-section animate-in animate-delay-4">
                <div className="data-section-header">
                  <span className="data-section-title">Recent Activity</span>
                  <button className="btn btn-secondary" onClick={() => navigateTo('audit')}>View All</button>
                </div>
                <table className="data-table">
                  <thead><tr><th>Time</th><th>Event</th><th>Actor</th><th>Resource</th><th>Detail</th></tr></thead>
                  <tbody>
                    {MOCK_DATA.auditLog.slice(0, 5).map((e, idx) => (
                      <tr key={idx}>
                        <td style={{ fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--text-tertiary)' }}>{e.time.split(' ')[1]}</td>
                        <td><span className={`badge ${getEventBadgeClass(e.type)}`}><span className="badge-dot" />{formatEventType(e.type)}</span></td>
                        <td><span className="timeline-actor">{e.actor}</span></td>
                        <td style={{ fontFamily: 'var(--font-mono)', fontSize: '13px' }}>{e.resource}</td>
                        <td style={{ maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{e.detail}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Items Page */}
          {currentPage === 'items' && (
            <div>
              <div className="data-section-header" style={{ background: 'none', border: 'none', padding: '0 0 16px 0' }}>
                <div className="filters-row">
                  <select className="filter-select" value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}>
                    <option value="">All Statuses</option>
                    <option value="ACTIVE">Active</option>
                    <option value="DRAFT">Draft</option>
                    <option value="REVIEW">Review</option>
                    <option value="CALIBRATION">Calibration</option>
                  </select>
                  <select className="filter-select" value={filterSubject} onChange={(e) => setFilterSubject(e.target.value)}>
                    <option value="">All Subjects</option>
                    <option value="Physics">Physics</option>
                    <option value="Chemistry">Chemistry</option>
                    <option value="Mathematics">Mathematics</option>
                    <option value="Biology">Biology</option>
                  </select>
                  <select className="filter-select" value={filterDifficulty} onChange={(e) => setFilterDifficulty(e.target.value)}>
                    <option value="">All Difficulties</option>
                    <option value="EASY">Easy</option>
                    <option value="MEDIUM">Medium</option>
                    <option value="HARD">Hard</option>
                    <option value="VERY_HARD">Very Hard</option>
                  </select>
                </div>
                <button className="btn btn-primary">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                  New Item
                </button>
              </div>

              <div className="data-section">
                <table className="data-table">
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
                  <tbody>
                    {filteredItems.map((item, idx) => (
                      <tr key={idx} className="animate-in">
                        <td style={{ fontFamily: 'var(--font-mono)', fontSize: '13px', color: 'var(--accent-primary)' }}>{item.externalId}</td>
                        <td>{item.subject}</td>
                        <td style={{ fontSize: '12px' }}>{item.type.replace('_', ' ')}</td>
                        <td><span className={`badge ${item.status.toLowerCase()}`}><span className="badge-dot" />{item.status}</span></td>
                        <td>{item.difficulty}</td>
                        <td style={{ fontFamily: 'var(--font-mono)' }}>{item.irt ? item.irt.b.toFixed(2) : '—'}</td>
                        <td style={{ fontFamily: 'var(--font-mono)' }}>{item.irt ? item.irt.a.toFixed(2) : '—'}</td>
                        <td style={{ fontFamily: 'var(--font-mono)' }}>{item.pValue !== null ? item.pValue.toFixed(2) : '—'}</td>
                        <td style={{ fontFamily: 'var(--font-mono)' }}>{item.discrimination !== null ? item.discrimination.toFixed(2) : '—'}</td>
                        <td>{item.exposure}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Blueprints Page */}
          {currentPage === 'blueprints' && (
            <div className="data-section animate-in">
              <table className="data-table">
                <thead><tr><th>Name</th><th>Subject</th><th>Items</th><th>Chapters</th><th>Status</th><th>Time Budget</th><th>Version</th></tr></thead>
                <tbody>
                  <tr><td style={{ fontWeight: 600 }}>JEE Main Physics Blueprint</td><td>Physics</td><td>25</td><td>8</td><td><span className="badge active"><span className="badge-dot" />ACTIVE</span></td><td>60 min</td><td>v3</td></tr>
                  <tr><td style={{ fontWeight: 600 }}>JEE Main Chemistry Blueprint</td><td>Chemistry</td><td>25</td><td>7</td><td><span class="badge active"><span class="badge-dot" />ACTIVE</span></td><td>60 min</td><td>v2</td></tr>
                  <tr><td style={{ fontWeight: 600 }}>JEE Main Mathematics Blueprint</td><td>Mathematics</td><td>25</td><td>9</td><td><span class="badge active"><span class="badge-dot" />ACTIVE</span></td><td>60 min</td><td>v3</td></tr>
                  <tr><td style={{ fontWeight: 600 }}>NEET Biology Blueprint</td><td>Biology</td><td>90</td><td>15</td><td><span className="badge draft"><span class="badge-dot" />DRAFT</span></td><td>100 min</td><td>v1</td></tr>
                </tbody>
              </table>
            </div>
          )}

          {/* Exams Page */}
          {currentPage === 'exams' && (
            <div className="data-section animate-in">
              <table className="data-table">
                <thead><tr><th>Code</th><th>Name</th><th>Type</th><th>Status</th><th>Marks</th><th>Questions</th><th>Duration</th><th>Candidates</th></tr></thead>
                <tbody>
                  {MOCK_DATA.exams.map((e, idx) => (
                    <tr key={idx}>
                      <td style={{ fontFamily: 'var(--font-mono)', fontSize: '13px', color: 'var(--accent-primary)' }}>{e.code}</td>
                      <td style={{ fontWeight: 500 }}>{e.name}</td>
                      <td style={{ fontSize: '12px' }}>{e.type.replace(/_/g, ' ')}</td>
                      <td><span className={`badge ${e.status.toLowerCase()}`}><span className="badge-dot" />{e.status}</span></td>
                      <td>{e.totalMarks}</td>
                      <td>{e.questions}</td>
                      <td>{e.duration} min</td>
                      <td>{e.candidates ? (e.candidates / 1000).toFixed(0) + 'K' : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Papers Page */}
          {currentPage === 'papers' && (
            <div className="data-section animate-in">
              <table className="data-table">
                <thead><tr><th>Paper Code</th><th>Exam</th><th>Form</th><th>Items</th><th>Mean b</th><th>TIF(0)</th><th>Status</th></tr></thead>
                <tbody>
                  <tr><td style={{ fontFamily: 'var(--font-mono)' }}>JEE-ADV-2026-F1</td><td>JEE Advanced 2026</td><td>Form 1</td><td>54</td><td style={{ fontFamily: 'var(--font-mono)' }}>0.85</td><td style={{ fontFamily: 'var(--font-mono)' }}>24.5</td><td><span className="badge active"><span class="badge-dot" />OPTIMAL</span></td></tr>
                  <tr><td style={{ fontFamily: 'var(--font-mono)' }}>JEE-ADV-2026-F2</td><td>JEE Advanced 2026</td><td>Form 2</td><td>54</td><td style={{ fontFamily: 'var(--font-mono)' }}>0.82</td><td style={{ fontFamily: 'var(--font-mono)' }}>23.8</td><td><span className="badge active"><span class="badge-dot" />OPTIMAL</span></td></tr>
                </tbody>
              </table>
            </div>
          )}

          {/* IRT Analytics Page */}
          {currentPage === 'analytics' && (
            <div className="charts-grid">
              <div className="chart-card animate-in animate-delay-1">
                <div className="chart-title">
                  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-secondary)" strokeWidth="2" style={{ marginRight: 8 }}><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                  Test Information Function (TIF)
                </div>
                <div className="irt-display">
                  {renderTIFCurveSVG()}
                </div>
              </div>
              <div className="chart-card animate-in animate-delay-2">
                <div className="chart-title">
                  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="var(--accent-green)" strokeWidth="2" style={{ marginRight: 8 }}><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
                  Score Distribution — JEE Main Jan 2026
                </div>
                <div className="chart-body">
                  {scoreBins.map((b, idx) => (
                    <div key={idx} className="bar-group">
                      <div className="bar" style={{ height: `${(b.count / maxScoreCount) * 190}px`, background: `linear-gradient(180deg, hsl(${230 + idx * 8}, 70%, ${40 + idx * 3}%) 0%, hsl(${230 + idx * 8}, 60%, 25%) 100%)` }} />
                      <span className="bar-label">{b.label.split('-')[0]}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* Audit Log Page */}
          {currentPage === 'audit' && (
            <div className="data-section animate-in">
              <div style={{ padding: '20px' }}>
                <div className="timeline">
                  {MOCK_DATA.auditLog.map((e, idx) => (
                    <div key={idx} className="timeline-item">
                      <div className="timeline-dot" style={{ backgroundColor: getEventColor(e.type) }} />
                      <div className="timeline-time">{e.time}</div>
                      <div className="timeline-event">
                        <span className={`badge ${getEventBadgeClass(e.type)}`} style={{ marginRight: 8 }}>
                          <span className="badge-dot" />{formatEventType(e.type)}
                        </span>
                        by <span className="timeline-actor">{e.actor}</span> on <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--accent-primary)' }}>{e.resource}</span>
                      </div>
                      <div style={{ marginTop: '4px', fontSize: '13px', color: 'var(--text-tertiary)' }}>{e.detail}</div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      </main>
    </>
  );
}
