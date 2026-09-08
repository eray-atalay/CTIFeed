import { useState, useEffect, useRef } from 'preact/hooks';
import { fetchAnalytics } from '../../services/api';
import { refreshTrigger } from '../../services/store';
import type { AnalyticsData } from '../../types/cti';

declare const Chart: any;

export default function AnalyticsDashboard() {
  const [collapsed, setCollapsed] = useState<boolean>(() => {
    if (typeof window !== 'undefined') {
      return localStorage.getItem('ctifeed_analytics_collapsed') === 'true';
    }
    return false;
  });

  const [analyticsData, setAnalyticsData] = useState<AnalyticsData | null>(null);

  const tagsCanvasRef = useRef<HTMLCanvasElement>(null);
  const vendorsCanvasRef = useRef<HTMLCanvasElement>(null);
  const timelineCanvasRef = useRef<HTMLCanvasElement>(null);

  const tagsChartRef = useRef<any>(null);
  const vendorsChartRef = useRef<any>(null);
  const timelineChartRef = useRef<any>(null);

  const loadData = async () => {
    try {
      const data = await fetchAnalytics();
      setAnalyticsData(data);
    } catch (err) {
      console.error('Analitik yüklenirken hata:', err);
    }
  };

  useEffect(() => {
    loadData();
    return refreshTrigger.subscribe(() => {
      loadData();
    });
  }, []);

  const toggleCollapsed = () => {
    const next = !collapsed;
    setCollapsed(next);
    localStorage.setItem('ctifeed_analytics_collapsed', String(next));
    if (!next) {
      setTimeout(() => {
        tagsChartRef.current?.resize();
        vendorsChartRef.current?.resize();
        timelineChartRef.current?.resize();
      }, 50);
    }
  };

  // Grafikleri çizme
  useEffect(() => {
    if (!analyticsData || collapsed || typeof Chart === 'undefined') return;

    Chart.defaults.color = '#94a3b8';
    Chart.defaults.font.family = "'Inter', -apple-system, BlinkMacSystemFont, sans-serif";

    // 1. Tehdit Dağılımı (Doughnut)
    if (tagsCanvasRef.current && analyticsData.top_tags?.length > 0) {
      if (tagsChartRef.current) tagsChartRef.current.destroy();

      const colors = ['#00f2fe', '#ff3366', '#ffb703', '#9d4edd', '#06d6a0', '#3a86ff', '#fb5607', '#a2d2ff'];
      tagsChartRef.current = new Chart(tagsCanvasRef.current, {
        type: 'doughnut',
        data: {
          labels: analyticsData.top_tags.map(t => t.label),
          datasets: [{
            data: analyticsData.top_tags.map(t => t.count),
            backgroundColor: colors.slice(0, analyticsData.top_tags.length),
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
          },
        },
      });
    }

    // 2. Üreticiler (Horizontal Bar)
    if (vendorsCanvasRef.current && analyticsData.top_vendors?.length > 0) {
      if (vendorsChartRef.current) vendorsChartRef.current.destroy();

      vendorsChartRef.current = new Chart(vendorsCanvasRef.current, {
        type: 'bar',
        data: {
          labels: analyticsData.top_vendors.map(v => v.vendor),
          datasets: [{
            label: 'Tespit Sayısı',
            data: analyticsData.top_vendors.map(v => v.count),
            backgroundColor: 'rgba(0, 242, 254, 0.55)',
            borderColor: '#00f2fe',
            borderWidth: 1.5,
            borderRadius: 4,
          }],
        },
        options: {
          indexAxis: 'y',
          responsive: true,
          maintainAspectRatio: false,
          plugins: { legend: { display: false } },
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

    // 3. Aktivite Nabzı (Line/Area)
    if (timelineCanvasRef.current && analyticsData.timeline?.length > 0) {
      if (timelineChartRef.current) timelineChartRef.current.destroy();

      timelineChartRef.current = new Chart(timelineCanvasRef.current, {
        type: 'line',
        data: {
          labels: analyticsData.timeline.map(t => {
            const parts = t.date.split('-');
            return parts.length === 3 ? `${parts[2]}/${parts[1]}` : t.date;
          }),
          datasets: [
            {
              label: 'Toplam Tehditler',
              data: analyticsData.timeline.map(t => t.total),
              borderColor: '#00f2fe',
              backgroundColor: 'rgba(0, 242, 254, 0.12)',
              fill: true,
              tension: 0.35,
              borderWidth: 2,
              pointBackgroundColor: '#00f2fe',
            },
            {
              label: 'Kritik Alarmlar (50+)',
              data: analyticsData.timeline.map(t => t.critical),
              borderColor: '#ff3366',
              backgroundColor: 'rgba(255, 51, 102, 0.12)',
              fill: true,
              tension: 0.35,
              borderWidth: 2,
              pointBackgroundColor: '#ff3366',
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
              },
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

    return () => {
      tagsChartRef.current?.destroy();
      vendorsChartRef.current?.destroy();
      timelineChartRef.current?.destroy();
    };
  }, [analyticsData, collapsed]);

  return (
    <section class="analytics-section" id="analytics-section" aria-label="Tehdit Analitiği ve Grafikler">
      <div class="analytics-header">
        <div class="analytics-title-group">
          <div class="analytics-pulse-dot"></div>
          <h3 class="analytics-title">Siber Tehdit Analitiği &amp; Trendler</h3>
          <span class="analytics-badge">Canlı Analiz</span>
        </div>
        <button
          class={`btn-toggle-analytics ${collapsed ? 'collapsed' : ''}`}
          title="Analitik Grafikleri Göster/Gizle"
          onClick={toggleCollapsed}
        >
          <span>{collapsed ? 'Grafikleri Göster' : 'Grafikleri Gizle'}</span>
          <svg class="toggle-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="18 15 12 9 6 15"></polyline>
          </svg>
        </button>
      </div>

      <div class={`analytics-charts-grid ${collapsed ? 'collapsed' : ''}`} id="analytics-charts-container">
        {/* 1. Tehdit Türü Dağılımı (Doughnut Chart) */}
        <div class="chart-card">
          <div class="chart-card-header">
            <h4>Tehdit Türü Dağılımı</h4>
            <span class="chart-tag">Kategoriler</span>
          </div>
          <div class="chart-body">
            <canvas ref={tagsCanvasRef}></canvas>
          </div>
        </div>

        {/* 2. En Çok Hedeflenen Üreticiler (Bar Chart) */}
        <div class="chart-card">
          <div class="chart-card-header">
            <h4>Hedeflenen Teknolojiler</h4>
            <span class="chart-tag">Üreticiler</span>
          </div>
          <div class="chart-body">
            <canvas ref={vendorsCanvasRef}></canvas>
          </div>
        </div>

        {/* 3. Aktivite Nabzı & Zaman Çizelgesi (Line/Area Chart) */}
        <div class="chart-card chart-card-wide">
          <div class="chart-card-header">
            <h4>Tehdit Aktivite Nabzı (Son 7 Gün)</h4>
            <span class="chart-tag">Zaman Çizelgesi</span>
          </div>
          <div class="chart-body">
            <canvas ref={timelineCanvasRef}></canvas>
          </div>
        </div>
      </div>
    </section>
  );
}
