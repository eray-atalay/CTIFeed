/**
 * CTIFeed Tehdit Radari - Istemci Mantigi (Sifir Emoji - %100 Turkce)
 */

(function () {
  'use strict';

  // Uygulama Durumu (State)
  const state = {
    search: '',
    tag: '',
    source: '',
    minScore: 0,
    sortBy: 'score',
    articles: [],
    sources: [],
    stats: null,
    analytics: null,
    chartTagsInstance: null,
    chartVendorsInstance: null,
    chartTimelineInstance: null,
    analyticsCollapsed: localStorage.getItem('ctifeed_analytics_collapsed') === 'true',
    isScanning: false,
    selectedArticle: null,
  };

  // DOM Ogeleri
  const el = {
    statTotal: document.getElementById('stat-total'),
    statCritical: document.getElementById('stat-critical'),
    statCve: document.getElementById('stat-cve'),
    statTr: document.getElementById('stat-tr'),
    searchInput: document.getElementById('search-input'),
    searchClearBtn: document.getElementById('search-clear-btn'),
    tagPills: document.getElementById('tag-pills'),
    sourceSelect: document.getElementById('source-select'),
    scoreSelect: document.getElementById('score-select'),
    sortSelect: document.getElementById('sort-select'),
    articlesGrid: document.getElementById('articles-grid'),
    feedCount: document.getElementById('feed-count'),
    lastUpdatedText: document.getElementById('last-updated-text'),
    emptyState: document.getElementById('empty-state'),
    loadingGrid: document.getElementById('loading-grid'),
    activeFilterBar: document.getElementById('active-filter-bar'),
    filterSummaryText: document.getElementById('filter-summary-text'),
    btnResetFilters: document.getElementById('btn-reset-filters'),
    btnEmptyReset: document.getElementById('btn-empty-reset'),
    btnScanNow: document.getElementById('btn-scan-now'),
    scanBtnText: document.getElementById('scan-btn-text'),
    scanBanner: document.getElementById('scan-banner'),
    scanBannerText: document.getElementById('scan-banner-text'),
    btnSources: document.getElementById('btn-sources'),
    sourcesCountBadge: document.getElementById('sources-count-badge'),
    sourcesModal: document.getElementById('sources-modal'),
    sourcesCloseBtn: document.getElementById('sources-close-btn'),
    sourcesListContainer: document.getElementById('sources-list-container'),
    articleModal: document.getElementById('article-modal'),
    modalCloseBtn: document.getElementById('modal-close-btn'),
    modalCloseFooter: document.getElementById('modal-close-footer'),
    modalSource: document.getElementById('modal-source'),
    modalDate: document.getElementById('modal-date'),
    modalScoreBadge: document.getElementById('modal-score-badge'),
    modalTitle: document.getElementById('modal-title'),
    modalBreakdown: document.getElementById('modal-breakdown'),
    modalTags: document.getElementById('modal-tags'),
    modalSummaryText: document.getElementById('modal-summary-text'),
    modalLinkBtn: document.getElementById('modal-link-btn'),
    btnToggleAnalytics: document.getElementById('btn-toggle-analytics'),
    toggleAnalyticsText: document.getElementById('toggle-analytics-text'),
    analyticsChartsContainer: document.getElementById('analytics-charts-container'),
    chartTags: document.getElementById('chart-tags'),
    chartVendors: document.getElementById('chart-vendors'),
    chartTimeline: document.getElementById('chart-timeline'),
  };

  // --- API Istekleri ---

  async function fetchStats() {
    try {
      const res = await fetch('/api/stats');
      if (!res.ok) throw new Error('Istatistik istegi basarisiz');
      const data = await res.json();
      state.stats = data;
      renderStats(data);
    } catch (err) {
      console.error('Istatistikler yuklenirken hata:', err);
    }
  }

  async function fetchAnalytics() {
    try {
      const res = await fetch('/api/analytics');
      if (!res.ok) throw new Error('Analitik verileri alinamadi');
      const data = await res.json();
      state.analytics = data;
      renderAnalytics(data);
    } catch (err) {
      console.error('Analitik yuklenirken hata:', err);
    }
  }

  async function fetchSources() {
    try {
      const res = await fetch('/api/sources');
      if (!res.ok) throw new Error('Kaynaklar istegi basarisiz');
      const data = await res.json();
      state.sources = data.sources || [];
      populateSourcesDropdown(state.sources);
      if (el.sourcesCountBadge) {
        el.sourcesCountBadge.textContent = state.sources.length;
      }
    } catch (err) {
      console.error('Kaynaklar yuklenirken hata:', err);
    }
  }

  async function fetchArticles() {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (state.search) params.set('search', state.search);
      if (state.tag) params.set('tag', state.tag);
      if (state.source) params.set('source', state.source);
      if (state.minScore > 0) params.set('min_score', state.minScore);
      if (state.sortBy) params.set('sort', state.sortBy);
      params.set('limit', '60');

      const res = await fetch(`/api/articles?${params.toString()}`);
      if (!res.ok) throw new Error('Tehdit akisi istegi basarisiz');
      const data = await res.json();

      state.articles = data.articles || [];
      renderArticles(state.articles, data.total || 0);
      updateFilterSummary();
    } catch (err) {
      console.error('Haberler yuklenirken hata:', err);
      el.articlesGrid.innerHTML = `<div class="empty-state"><h3>Tehdit beslemeleri yuklenemedi</h3><p>${escapeHtml(err.message)}</p></div>`;
    } finally {
      setLoading(false);
    }
  }

  async function triggerScan() {
    if (state.isScanning) return;
    state.isScanning = true;
    el.btnScanNow.classList.add('scanning');
    el.scanBtnText.textContent = '18 Kaynak Taraniyor...';
    el.scanBanner.classList.remove('hidden');
    el.scanBannerText.textContent = '18 CTI kaynagina baglaniliyor, beslemeler ayristiriliyor ve puanlaniyor...';

    try {
      const res = await fetch('/api/scan', { method: 'POST' });
      if (!res.ok) throw new Error('Tarama dongusu basarisiz');
      const data = await res.json();
      
      el.scanBannerText.textContent = `Tarama tamamlandi: ${data.new_inserted} yeni tehdit maddesi eklendi, ${data.duplicates_skipped} mukerrer kayit atlandi.`;
      setTimeout(() => {
        el.scanBanner.classList.add('hidden');
      }, 5000);

      // Verileri guncelle
      await fetchStats();
      await fetchAnalytics();
      await fetchArticles();
    } catch (err) {
      console.error('Tarama hatasi:', err);
      el.scanBannerText.textContent = `Tarama hatasi: ${err.message}`;
      setTimeout(() => el.scanBanner.classList.add('hidden'), 4000);
    } finally {
      state.isScanning = false;
      el.btnScanNow.classList.remove('scanning');
      el.scanBtnText.textContent = 'Beslemeleri Tara';
    }
  }

  // --- Render Fonksiyonlari ---

  function renderStats(stats) {
    if (!stats) return;
    animateValue(el.statTotal, stats.total_articles || 0);
    animateValue(el.statCritical, stats.high_priority_count || 0);
    animateValue(el.statCve, stats.critical_vulnerabilities || 0);
    animateValue(el.statTr, stats.tr_focus_count || 0);
  }

  function renderAnalytics(data) {
    if (!data || typeof Chart === 'undefined') return;

    // Chart.js Global Tema Ayarlari
    Chart.defaults.color = '#94a3b8';
    Chart.defaults.font.family = "'Inter', -apple-system, BlinkMacSystemFont, sans-serif";

    // 1. Tehdit Turu Dagilimi (Doughnut Chart)
    if (el.chartTags && data.top_tags && data.top_tags.length > 0) {
      if (state.chartTagsInstance) {
        state.chartTagsInstance.destroy();
      }

      const colors = ['#00f2fe', '#ff3366', '#ffb703', '#9d4edd', '#06d6a0', '#3a86ff', '#fb5607', '#a2d2ff'];
      state.chartTagsInstance = new Chart(el.chartTags, {
        type: 'doughnut',
        data: {
          labels: data.top_tags.map(t => t.label),
          datasets: [{
            data: data.top_tags.map(t => t.count),
            backgroundColor: colors.slice(0, data.top_tags.length),
            borderColor: '#0b111e',
            borderWidth: 2,
            hoverOffset: 4,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          cutout: '68%',
          plugins: {
            legend: {
              position: 'right',
              labels: {
                boxWidth: 10,
                boxHeight: 10,
                usePointStyle: true,
                pointStyle: 'circle',
                font: { size: 11 },
                color: '#cbd5e1',
                padding: 10,
              },
            },
            tooltip: {
              backgroundColor: '#152033',
              borderColor: 'rgba(255, 255, 255, 0.1)',
              borderWidth: 1,
              titleFont: { size: 12, weight: 'bold' },
              bodyFont: { size: 12 },
              padding: 10,
              callbacks: {
                label: function(ctx) {
                  const total = ctx.dataset.data.reduce((a, b) => a + b, 0);
                  const val = ctx.parsed;
                  const pct = total > 0 ? ((val / total) * 100).toFixed(1) : 0;
                  return ` ${ctx.label}: ${val} (%${pct})`;
                }
              }
            },
          },
        },
      });
    }

    // 2. En Cok Hedeflenen Ureticiler (Horizontal Bar Chart)
    if (el.chartVendors && data.top_vendors && data.top_vendors.length > 0) {
      if (state.chartVendorsInstance) {
        state.chartVendorsInstance.destroy();
      }

      state.chartVendorsInstance = new Chart(el.chartVendors, {
        type: 'bar',
        data: {
          labels: data.top_vendors.map(v => v.vendor),
          datasets: [{
            label: 'Tespit Sayisi',
            data: data.top_vendors.map(v => v.count),
            backgroundColor: 'rgba(0, 242, 254, 0.55)',
            borderColor: '#00f2fe',
            borderWidth: 1.5,
            borderRadius: 4,
            hoverBackgroundColor: 'rgba(0, 242, 254, 0.85)',
          }],
        },
        options: {
          indexAxis: 'y',
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: { display: false },
            tooltip: {
              backgroundColor: '#152033',
              borderColor: 'rgba(255, 255, 255, 0.1)',
              borderWidth: 1,
              padding: 10,
            },
          },
          scales: {
            x: {
              grid: { color: 'rgba(255, 255, 255, 0.04)' },
              ticks: { color: '#64748b', font: { size: 10 }, stepSize: 1 },
            },
            y: {
              grid: { display: false },
              ticks: { color: '#cbd5e1', font: { size: 11, weight: '500' } },
            },
          },
        },
      });
    }

    // 3. Aktivite Nabzi (Timeline Area Chart)
    if (el.chartTimeline && data.timeline && data.timeline.length > 0) {
      if (state.chartTimelineInstance) {
        state.chartTimelineInstance.destroy();
      }

      state.chartTimelineInstance = new Chart(el.chartTimeline, {
        type: 'line',
        data: {
          labels: data.timeline.map(t => {
            const parts = t.date.split('-');
            return parts.length === 3 ? `${parts[2]}/${parts[1]}` : t.date;
          }),
          datasets: [
            {
              label: 'Toplam Tehditler',
              data: data.timeline.map(t => t.total),
              borderColor: '#00f2fe',
              backgroundColor: 'rgba(0, 242, 254, 0.12)',
              fill: true,
              tension: 0.35,
              borderWidth: 2,
              pointBackgroundColor: '#00f2fe',
              pointRadius: 4,
              pointHoverRadius: 6,
            },
            {
              label: 'Kritik Alarmlar (50+)',
              data: data.timeline.map(t => t.critical),
              borderColor: '#ff3366',
              backgroundColor: 'rgba(255, 51, 102, 0.12)',
              fill: true,
              tension: 0.35,
              borderWidth: 2,
              pointBackgroundColor: '#ff3366',
              pointRadius: 4,
              pointHoverRadius: 6,
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: {
              position: 'top',
              align: 'end',
              labels: {
                boxWidth: 10,
                boxHeight: 10,
                usePointStyle: true,
                pointStyle: 'circle',
                font: { size: 11 },
                color: '#cbd5e1',
                padding: 14,
              },
            },
            tooltip: {
              backgroundColor: '#152033',
              borderColor: 'rgba(255, 255, 255, 0.1)',
              borderWidth: 1,
              padding: 10,
            },
          },
          scales: {
            x: {
              grid: { color: 'rgba(255, 255, 255, 0.04)' },
              ticks: { color: '#64748b', font: { size: 11 } },
            },
            y: {
              grid: { color: 'rgba(255, 255, 255, 0.04)' },
              ticks: { color: '#64748b', font: { size: 11 }, stepSize: 1 },
              beginAtZero: true,
            },
          },
        },
      });
    }
  }

  function animateValue(elem, end) {
    if (!elem) return;
    const start = parseInt(elem.textContent, 10) || 0;
    if (start === end) {
      elem.textContent = end;
      return;
    }
    const duration = 600;
    const startTime = performance.now();

    function update(currentTime) {
      const elapsed = currentTime - startTime;
      const progress = Math.min(elapsed / duration, 1);
      const current = Math.floor(start + (end - start) * progress);
      elem.textContent = current;
      if (progress < 1) {
        requestAnimationFrame(update);
      } else {
        elem.textContent = end;
      }
    }
    requestAnimationFrame(update);
  }

  function populateSourcesDropdown(sources) {
    el.sourceSelect.innerHTML = '<option value="">Tum Kaynaklar</option>';
    sources.forEach(src => {
      const opt = document.createElement('option');
      opt.value = src.name;
      opt.textContent = src.name;
      el.sourceSelect.appendChild(opt);
    });
  }

  function renderArticles(articles, totalCount) {
    el.feedCount.textContent = `${totalCount} oge`;
    el.lastUpdatedText.textContent = `Son guncelleme: ${new Date().toLocaleTimeString('tr-TR')}`;

    if (!articles || articles.length === 0) {
      el.articlesGrid.innerHTML = '';
      el.emptyState.classList.remove('hidden');
      return;
    }

    el.emptyState.classList.add('hidden');
    el.articlesGrid.innerHTML = articles.map(article => createArticleCardHTML(article)).join('');

    // Tiklama olaylarini bagla
    el.articlesGrid.querySelectorAll('.article-card').forEach(card => {
      const articleId = parseInt(card.dataset.id, 10);
      const article = articles.find(a => a.id === articleId);

      card.addEventListener('click', (e) => {
        if (e.target.closest('.tag-item') || e.target.closest('.card-action-link')) {
          return;
        }
        openArticleModal(article);
      });
    });

    // Kart ici etiket filtre tiklamalari
    el.articlesGrid.querySelectorAll('.tag-item').forEach(tagBtn => {
      tagBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        const tag = tagBtn.dataset.tag;
        selectTag(tag);
      });
    });
  }

  function createArticleCardHTML(article) {
    const isCritical = article.score >= 50;
    const scoreBadge = getScoreBadge(article.score);
    const timeAgo = formatTimeAgo(article.published_at);

    const tagsHTML = (article.tags || []).map(t => {
      const isCVE = t.toUpperCase().startsWith('CVE-');
      const isTR = t === 'TR-Focus';
      let tagClass = 'tag-item';
      if (isCVE) tagClass += ' tag-cve';
      if (isTR) tagClass += ' tag-tr';
      return `<button class="${tagClass}" data-tag="${escapeHtml(t)}">${escapeHtml(t)}</button>`;
    }).join('');

    return `
      <article class="article-card ${isCritical ? 'is-critical' : ''}" data-id="${article.id}">
        <div>
          <div class="card-top-row">
            <span class="source-badge">
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="2" y1="12" x2="22" y2="12"></line>
              </svg>
              ${escapeHtml(article.source)}
            </span>
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="card-time">${timeAgo}</span>
              ${scoreBadge}
            </div>
          </div>

          <h3 class="card-title">${escapeHtml(article.title)}</h3>
          <p class="card-summary">${escapeHtml(article.summary || 'Bu kayit icin ozet metin bulunmuyor.')}</p>
        </div>

        <div>
          ${tagsHTML ? `<div class="card-tags">${tagsHTML}</div>` : ''}
          <div class="card-footer">
            <span class="card-action-inspect">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="16" x2="12" y2="12"></line>
                <line x1="12" y1="8" x2="12.01" y2="8"></line>
              </svg>
              Detaylari Incele
            </span>
            <a href="${escapeHtml(article.link)}" target="_blank" rel="noopener noreferrer" class="card-action-link" title="Kaynak Haberi Goruntule">
              <span>Kaynak</span>
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path>
                <polyline points="15 3 21 3 21 9"></polyline>
                <line x1="10" y1="14" x2="21" y2="3"></line>
              </svg>
            </a>
          </div>
        </div>
      </article>
    `;
  }

  function getScoreBadge(score) {
    if (score >= 80) {
      return `<span class="score-badge score-critical">${score} KRITIK</span>`;
    } else if (score >= 50) {
      return `<span class="score-badge score-high">${score} YUKSEK</span>`;
    } else if (score >= 30) {
      return `<span class="score-badge score-medium">${score} ORTA</span>`;
    } else {
      return `<span class="score-badge score-low">${score} DUSUK</span>`;
    }
  }

  function setLoading(loading) {
    if (loading) {
      el.loadingGrid.classList.remove('hidden');
      el.emptyState.classList.add('hidden');
    } else {
      el.loadingGrid.classList.add('hidden');
    }
  }

  function updateFilterSummary() {
    const filters = [];
    if (state.search) filters.push(`Arama: "${state.search}"`);
    if (state.tag) filters.push(`Etiket: [${state.tag}]`);
    if (state.source) filters.push(`Kaynak: ${state.source}`);
    if (state.minScore > 0) filters.push(`Min Skor: ${state.minScore}+`);

    if (filters.length > 0) {
      el.filterSummaryText.textContent = filters.join(' - ');
      el.activeFilterBar.classList.remove('hidden');
    } else {
      el.activeFilterBar.classList.add('hidden');
    }
  }

  function selectTag(tag) {
    state.tag = tag;
    el.tagPills.querySelectorAll('.pill').forEach(pill => {
      if (pill.dataset.tag === tag) {
        pill.classList.add('active');
      } else {
        pill.classList.remove('active');
      }
    });
    fetchArticles();
  }

  // --- Modal Mantigi ---

  function openArticleModal(article) {
    if (!article) return;
    state.selectedArticle = article;

    el.modalSource.textContent = article.source;
    el.modalDate.textContent = `Yayinlanma: ${new Date(article.published_at).toLocaleString('tr-TR')}`;
    el.modalScoreBadge.outerHTML = getScoreBadge(article.score);
    el.modalScoreBadge = document.getElementById('modal-score-badge') || el.modalDate.nextElementSibling;
    el.modalTitle.textContent = article.title;
    el.modalSummaryText.textContent = article.summary || 'Bu kayit icin ozet metin bulunmuyor.';
    el.modalLinkBtn.href = article.link;

    // Skor Dagilimi
    const breakdownHTML = [];

    if (article.tags && article.tags.includes('TR-Focus')) {
      breakdownHTML.push(`
        <div class="breakdown-item">
          <span>Turkiye Odagi Tespit Edildi (USOM / TR Kurumlari)</span>
          <span class="breakdown-badge" style="background: rgba(239, 68, 68, 0.2); color: #fca5a5;">+50 PUAN</span>
        </div>
      `);
    }

    const matchedCVEs = (article.tags || []).filter(t => t.toUpperCase().startsWith('CVE-'));
    if (matchedCVEs.length > 0) {
      breakdownHTML.push(`
        <div class="breakdown-item">
          <span>CVE Guvenlik Zafiyeti Tespit Edildi (${matchedCVEs.join(', ')})</span>
          <span class="breakdown-badge" style="background: rgba(244, 63, 94, 0.2); color: #fda4af;">+35 PUAN</span>
        </div>
      `);
    }

    const prodKeywords = ['fortinet', 'cisco', 'wordpress', 'vmware', 'palo-alto', 'microsoft-exchange', 'active-directory', 'ivanti'];
    const matchedProds = (article.tags || []).filter(t => prodKeywords.includes(t));
    if (matchedProds.length > 0) {
      breakdownHTML.push(`
        <div class="breakdown-item">
          <span>Kritik Kurumsal Sistemler (${matchedProds.join(', ')})</span>
          <span class="breakdown-badge" style="background: rgba(245, 158, 11, 0.2); color: #fde047;">+30 PUAN</span>
        </div>
      `);
    }

    const threatVecs = ['zero-day', 'rce', 'ransomware', 'data-breach', 'leak', 'apt'];
    const matchedThreats = (article.tags || []).filter(t => threatVecs.includes(t));
    if (matchedThreats.length > 0) {
      breakdownHTML.push(`
        <div class="breakdown-item">
          <span>Kritik Tehdit Vektorleri (${matchedThreats.join(', ')})</span>
          <span class="breakdown-badge" style="background: rgba(99, 102, 241, 0.2); color: #c7d2fe;">+20 PUAN</span>
        </div>
      `);
    }

    if (breakdownHTML.length === 0) {
      breakdownHTML.push(`
        <div class="breakdown-item">
          <span>Standart Tehdit Bulteni (Kritik tetikleyici kelime eslesmedi)</span>
          <span class="breakdown-badge">+0 PUAN</span>
        </div>
      `);
    }

    el.modalBreakdown.innerHTML = breakdownHTML.join('');

    // NIST NVD Baglantili CVE Etiketleri
    if (article.tags && article.tags.length > 0) {
      el.modalTags.innerHTML = article.tags.map(t => {
        if (t.toUpperCase().startsWith('CVE-')) {
          return `<a href="https://nvd.nist.gov/vuln/detail/${encodeURIComponent(t)}" target="_blank" rel="noopener noreferrer" class="tag-item tag-cve" title="NIST NVD uzerinde incele">[CVE] ${escapeHtml(t)}</a>`;
        }
        return `<span class="tag-item">${escapeHtml(t)}</span>`;
      }).join('');
    } else {
      el.modalTags.innerHTML = '<span style="color: var(--text-muted); font-size: 0.85rem;">Etiket atanmadi</span>';
    }

    el.articleModal.classList.remove('hidden');
  }

  function closeArticleModal() {
    el.articleModal.classList.add('hidden');
  }

  function openSourcesModal() {
    el.sourcesListContainer.innerHTML = state.sources.map(src => `
      <div class="source-item-card">
        <div class="source-info">
          <h5>${escapeHtml(src.name)}</h5>
          <span class="source-cat">${escapeHtml(src.category || 'Genel CTI')}</span>
        </div>
        <a href="${escapeHtml(src.url)}" target="_blank" rel="noopener noreferrer" class="btn-link" title="RSS XML Besleme Baglantisi">
          Besleme XML &rarr;
        </a>
      </div>
    `).join('');
    el.sourcesModal.classList.remove('hidden');
  }

  function closeSourcesModal() {
    el.sourcesModal.classList.add('hidden');
  }

  // --- Yardimci Fonksiyonlar ---

  function formatTimeAgo(dateString) {
    if (!dateString) return '';
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now - date;
    const diffSec = Math.floor(diffMs / 1000);
    const diffMin = Math.floor(diffSec / 60);
    const diffHour = Math.floor(diffMin / 60);
    const diffDay = Math.floor(diffHour / 24);

    if (diffMin < 1) return 'Simdi';
    if (diffMin < 60) return `${diffMin} dk once`;
    if (diffHour < 24) return `${diffHour} saat once`;
    if (diffDay === 1) return 'Dun';
    return `${diffDay} gun once`;
  }

  function escapeHtml(str) {
    if (!str) return '';
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  let searchTimeout = null;
  function debounceSearch(query) {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      state.search = query.trim();
      if (state.search) {
        el.searchClearBtn.classList.remove('hidden');
      } else {
        el.searchClearBtn.classList.add('hidden');
      }
      fetchArticles();
    }, 300);
  }

  function resetAllFilters() {
    state.search = '';
    state.tag = '';
    state.source = '';
    state.minScore = 0;
    el.searchInput.value = '';
    el.searchClearBtn.classList.add('hidden');
    el.sourceSelect.value = '';
    el.scoreSelect.value = '0';
    el.sortSelect.value = 'score';
    el.tagPills.querySelectorAll('.pill').forEach((p, idx) => {
      if (idx === 0) p.classList.add('active');
      else p.classList.remove('active');
    });
    fetchArticles();
  }

  // --- Olay Dinleyicileri ---

  function initListeners() {
    el.searchInput.addEventListener('input', (e) => debounceSearch(e.target.value));
    el.searchClearBtn.addEventListener('click', () => {
      el.searchInput.value = '';
      debounceSearch('');
    });

    el.tagPills.querySelectorAll('.pill').forEach(pill => {
      pill.addEventListener('click', () => {
        selectTag(pill.dataset.tag);
      });
    });

    el.sourceSelect.addEventListener('change', (e) => {
      state.source = e.target.value;
      fetchArticles();
    });

    el.scoreSelect.addEventListener('change', (e) => {
      state.minScore = parseInt(e.target.value, 10) || 0;
      fetchArticles();
    });

    el.sortSelect.addEventListener('change', (e) => {
      state.sortBy = e.target.value;
      fetchArticles();
    });

    el.btnResetFilters.addEventListener('click', resetAllFilters);
    el.btnEmptyReset.addEventListener('click', resetAllFilters);
    el.btnScanNow.addEventListener('click', triggerScan);

    el.btnSources.addEventListener('click', openSourcesModal);
    el.sourcesCloseBtn.addEventListener('click', closeSourcesModal);
    el.sourcesModal.addEventListener('click', (e) => {
      if (e.target === el.sourcesModal) closeSourcesModal();
    });

    el.modalCloseBtn.addEventListener('click', closeArticleModal);
    el.modalCloseFooter.addEventListener('click', closeArticleModal);
    el.articleModal.addEventListener('click', (e) => {
      if (e.target === el.articleModal) closeArticleModal();
    });

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        closeArticleModal();
        closeSourcesModal();
      }
    });

    // Analitik Panelini Gizle / Goster
    if (el.btnToggleAnalytics && el.analyticsChartsContainer) {
      if (state.analyticsCollapsed) {
        el.analyticsChartsContainer.classList.add('collapsed');
        el.btnToggleAnalytics.classList.add('collapsed');
        if (el.toggleAnalyticsText) el.toggleAnalyticsText.textContent = 'Grafikleri Goster';
      }

      el.btnToggleAnalytics.addEventListener('click', () => {
        state.analyticsCollapsed = !state.analyticsCollapsed;
        localStorage.setItem('ctifeed_analytics_collapsed', state.analyticsCollapsed);
        if (state.analyticsCollapsed) {
          el.analyticsChartsContainer.classList.add('collapsed');
          el.btnToggleAnalytics.classList.add('collapsed');
          if (el.toggleAnalyticsText) el.toggleAnalyticsText.textContent = 'Grafikleri Goster';
        } else {
          el.analyticsChartsContainer.classList.remove('collapsed');
          el.btnToggleAnalytics.classList.remove('collapsed');
          if (el.toggleAnalyticsText) el.toggleAnalyticsText.textContent = 'Grafikleri Gizle';
          if (state.chartTagsInstance) state.chartTagsInstance.resize();
          if (state.chartVendorsInstance) state.chartVendorsInstance.resize();
          if (state.chartTimelineInstance) state.chartTimelineInstance.resize();
        }
      });
    }

    setInterval(() => {
      fetchStats();
      fetchAnalytics();
    }, 30000);
  }

  async function init() {
    initListeners();
    await Promise.all([
      fetchStats(),
      fetchAnalytics(),
      fetchSources(),
      fetchArticles(),
    ]);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
