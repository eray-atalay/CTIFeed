import { useState, useEffect, useRef } from 'preact/hooks';
import { fetchStats } from '../../services/api';
import { refreshTrigger } from '../../services/store';
import type { ThreatStats } from '../../types/cti';

export default function MetricsGrid() {
  const [stats, setStats] = useState<ThreatStats | null>(null);
  const [displayTotal, setDisplayTotal] = useState<number>(0);
  const [displayCritical, setDisplayCritical] = useState<number>(0);
  const [displayCve, setDisplayCve] = useState<number>(0);
  const [displayTr, setDisplayTr] = useState<number>(0);

  const loadStats = async () => {
    try {
      const data = await fetchStats();
      setStats(data);
    } catch (err) {
      console.error('İstatistikler yüklenirken hata:', err);
    }
  };

  useEffect(() => {
    loadStats();
    return refreshTrigger.subscribe(() => {
      loadStats();
    });
  }, []);

  // Animasyonlu sayaçlar
  useEffect(() => {
    if (!stats) return;

    const animate = (target: number, current: number, setter: (val: number) => void) => {
      if (target === current) return;
      const duration = 600;
      const startTime = performance.now();

      const update = (currentTime: number) => {
        const elapsed = currentTime - startTime;
        const progress = Math.min(elapsed / duration, 1);
        const nextVal = Math.floor(current + (target - current) * progress);
        setter(nextVal);
        if (progress < 1) {
          requestAnimationFrame(update);
        } else {
          setter(target);
        }
      };
      requestAnimationFrame(update);
    };

    animate(stats.total_articles || 0, displayTotal, setDisplayTotal);
    animate(stats.high_priority_count || 0, displayCritical, setDisplayCritical);
    animate(stats.critical_vulnerabilities || 0, displayCve, setDisplayCve);
    animate(stats.tr_focus_count || 0, displayTr, setDisplayTr);
  }, [stats]);

  return (
    <section class="metrics-grid" aria-label="Tehdit İstatistikleri">
      <div class="metric-card metric-total">
        <div class="metric-header">
          <span class="metric-title">Toplanan İstihbarat</span>
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
              <polyline points="14 2 14 8 20 8"></polyline>
              <line x1="16" y1="13" x2="8" y2="13"></line>
              <line x1="16" y1="17" x2="8" y2="17"></line>
            </svg>
          </div>
        </div>
        <div class="metric-value">{stats ? displayTotal : '--'}</div>
        <div class="metric-footnote">Son 48 saatteki aktif tehdit kayıtları</div>
      </div>

      <div class="metric-card metric-critical">
        <div class="metric-header">
          <span class="metric-title">Kritik Öncelikli Alarmlar</span>
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
              <line x1="12" y1="9" x2="12" y2="13"></line>
              <line x1="12" y1="17" x2="12.01" y2="17"></line>
            </svg>
          </div>
        </div>
        <div class="metric-value">{stats ? displayCritical : '--'}</div>
        <div class="metric-footnote">Skor &ge; 50 (Acil müdahale gerektiren)</div>
      </div>

      <div class="metric-card metric-cve">
        <div class="metric-header">
          <span class="metric-title">Aktif CVE Zafiyetleri</span>
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
              <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
            </svg>
          </div>
        </div>
        <div class="metric-value">{stats ? displayCve : '--'}</div>
        <div class="metric-footnote">Regex ile tespit edilen CVE kodları</div>
      </div>

      <div class="metric-card metric-tr">
        <div class="metric-header">
          <span class="metric-title">Türkiye Odağı (TR-Focus)</span>
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <polygon points="12 8 8 12 12 16 16 12 12 8"></polygon>
            </svg>
          </div>
        </div>
        <div class="metric-value">{stats ? displayTr : '--'}</div>
        <div class="metric-footnote">Türk kurumları ve USOM uyarıları</div>
      </div>
    </section>
  );
}
