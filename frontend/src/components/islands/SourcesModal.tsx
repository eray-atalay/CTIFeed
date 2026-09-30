import { useState, useEffect } from 'preact/hooks';
import { fetchSources, toggleSource } from '../../services/api';
import type { SourceInfo } from '../../types/cti';

export default function SourcesModal() {
  const [sources, setSources] = useState<SourceInfo[]>([]);
  const [search, setSearch] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(true);
  const [togglingId, setTogglingId] = useState<number | null>(null);

  useEffect(() => {
    loadSources();
  }, []);

  const loadSources = () => {
    setLoading(true);
    fetchSources()
      .then((res) => setSources(res.sources || []))
      .catch((err) => console.error('Kaynaklar yüklenirken hata:', err))
      .finally(() => setLoading(false));
  };

  const handleToggle = async (src: SourceInfo) => {
    if (!src.id || togglingId !== null) return;
    const previousState = src.is_active !== false;
    const nextState = !previousState;

    setSources((prev) =>
      prev.map((item) => (item.id === src.id ? { ...item, is_active: nextState } : item))
    );
    setTogglingId(src.id);

    try {
      const res = await toggleSource(src.id);
      setSources((prev) =>
        prev.map((item) => (item.id === src.id ? { ...item, is_active: res.is_active } : item))
      );
    } catch (err) {
      console.error('Kaynak durumu değiştirilemedi:', err);
      setSources((prev) =>
        prev.map((item) => (item.id === src.id ? { ...item, is_active: previousState } : item))
      );
    } finally {
      setTogglingId(null);
    }
  };

  const filteredSources = sources.filter((s) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return s.name.toLowerCase().includes(q) || (s.category && s.category.toLowerCase().includes(q));
  });

  const activeCount = sources.filter((s) => s.is_active !== false).length;

  return (
    <div style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', padding: '20px' }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: '12px', flexWrap: 'wrap', marginBottom: '14px' }}>
        <h3 style={{ margin: 0, color: 'var(--text-primary)' }}>İzlenen CTI Besleme Kaynakları</h3>
        <span style={{ fontSize: '0.85rem', color: '#38bdf8', fontWeight: 600 }}>
          {activeCount} / {sources.length} Aktif
        </span>
      </div>

      <div style={{ marginBottom: '16px' }}>
        <input
          type="text"
          className="search-input"
          placeholder="Kaynak veya kategori ara..."
          value={search}
          onInput={(e) => setSearch((e.target as HTMLInputElement).value)}
          style={{ width: '100%', maxWidth: '360px', fontSize: '0.85rem', padding: '8px 12px', background: 'rgba(14,20,34,0.6)', border: '1px solid var(--bg-card-border)', borderRadius: '6px', color: 'var(--text-base)' }}
        />
      </div>

      {loading ? (
        <div style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
          Kaynaklar yükleniyor...
        </div>
      ) : (
        <div className="sources-list" style={{ maxHeight: '70vh', overflowY: 'auto' }}>
          {filteredSources.map((src) => {
            const isActive = src.is_active !== false;
            const status = src.last_status || 'pending';
            const latency = src.response_time_ms || 0;

            return (
              <div key={src.id || src.url} className={`source-item-card ${!isActive ? 'is-inactive' : ''}`}>
                <div className="source-info">
                  <div className="source-header-row">
                    <span className={`source-health-dot ${status}`} />
                    <h5 title={src.name}>{src.name}</h5>
                    {latency > 0 && isActive && (
                      <span className="source-latency-badge fast">{latency}ms</span>
                    )}
                  </div>
                  <div className="source-meta-row">
                    <span className="source-cat">{src.category || 'Genel CTI'}</span>
                  </div>
                </div>

                <div className="source-actions-group">
                  {src.id && (
                    <label className="source-toggle-switch">
                      <input
                        type="checkbox"
                        checked={isActive}
                        disabled={togglingId === src.id}
                        onChange={() => handleToggle(src)}
                      />
                      <span className="source-toggle-slider" />
                    </label>
                  )}
                  <a href={src.url} target="_blank" rel="noopener noreferrer" className="btn-link" style={{ fontSize: '0.78rem' }}>
                    XML &rarr;
                  </a>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}