import { useState, useEffect } from 'preact/hooks';
import { activeModal, closeModal } from '../../services/store';
import { fetchSources, toggleSource } from '../../services/api';
import type { SourceInfo } from '../../types/cti';

export default function SourcesModal() {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [sources, setSources] = useState<SourceInfo[]>([]);
  const [search, setSearch] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [togglingId, setTogglingId] = useState<number | null>(null);

  const loadSources = () => {
    setLoading(true);
    fetchSources()
      .then((res) => setSources(res.sources || []))
      .catch((err) => console.error('Kaynaklar yüklenirken hata:', err))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    const unsub = activeModal.subscribe((val) => {
      const open = val === 'sources';
      setIsOpen(open);
      if (open) {
        loadSources();
      }
    });

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeModal();
    };
    window.addEventListener('keydown', handleKeyDown);

    return () => {
      unsub();
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const handleToggle = async (src: SourceInfo) => {
    if (!src.id || togglingId !== null) return;
    const previousState = src.is_active !== false;
    const nextState = !previousState;

    // Optimistic UI update
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
      // Revert on error
      setSources((prev) =>
        prev.map((item) => (item.id === src.id ? { ...item, is_active: previousState } : item))
      );
    } finally {
      setTogglingId(null);
    }
  };

  if (!isOpen) return null;

  const filteredSources = sources.filter((s) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return s.name.toLowerCase().includes(q) || (s.category && s.category.toLowerCase().includes(q));
  });

  const activeCount = sources.filter((s) => s.is_active !== false).length;

  return (
    <div
      class="modal-overlay"
      role="dialog"
      aria-modal="true"
      onClick={(e) => {
        if (e.target === e.currentTarget) closeModal();
      }}
    >
      <div class="modal-card modal-card-wide">
        <button class="modal-close" onClick={closeModal} aria-label="Modalı Kapat">&times;</button>
        <div class="modal-header">
          <div style="display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap;">
            <h2 style="margin: 0;">İzlenen CTI Besleme Kaynakları</h2>
            <span style="font-size: 0.85rem; color: var(--accent-primary, #38bdf8); font-weight: 600;">
              {activeCount} / {sources.length} Aktif
            </span>
          </div>
          <p class="modal-sub" style="margin-top: 6px;">
            Siber tehditler, kritik CVE zafiyetleri ve TR-Focus istihbaratı için taranan kaynaklar ve sağlık durumları.
          </p>

          <div style="margin-top: 14px;">
            <input
              type="text"
              class="search-input"
              placeholder="Kaynak veya kategori ara..."
              value={search}
              onInput={(e) => setSearch((e.target as HTMLInputElement).value)}
              style="width: 100%; max-width: 360px; font-size: 0.85rem; padding: 6px 12px; background: rgba(14,20,34,0.6); border: 1px solid var(--bg-card-border); border-radius: 6px; color: var(--text-base);"
            />
          </div>
        </div>

        {loading ? (
          <div style="text-align: center; padding: 40px; color: var(--text-muted);">
            Kaynaklar yükleniyor...
          </div>
        ) : (
          <div class="sources-list">
            {filteredSources.map((src) => {
              const isActive = src.is_active !== false;
              const status = src.last_status || 'pending';
              const latency = src.response_time_ms || 0;
              let latencyClass = 'fast';
              if (latency >= 3000) latencyClass = 'slow';
              else if (latency >= 1000) latencyClass = 'medium';

              return (
                <div
                  key={src.id || src.url}
                  class={`source-item-card ${!isActive ? 'is-inactive' : ''}`}
                >
                  <div class="source-info">
                    <div class="source-header-row">
                      <span
                        class={`source-health-dot ${status}`}
                        title={
                          status === 'ok'
                            ? 'Kaynak Sağlıklı'
                            : status === 'error'
                            ? `Hata: ${src.last_error || 'Bağlantı kurulamadı'}`
                            : 'Henüz taranmadı'
                        }
                      />
                      <h5 title={src.name}>{src.name}</h5>
                      {latency > 0 && isActive && (
                        <span
                          class={`source-latency-badge ${latencyClass}`}
                          title={`Son Yanıt Süresi: ${latency}ms`}
                        >
                          {latency}ms
                        </span>
                      )}
                    </div>
                    <div class="source-meta-row">
                      <span class="source-cat">{src.category || 'Genel CTI'}</span>
                      {src.last_error && status === 'error' && (
                        <span
                          style="font-size: 0.7rem; color: #f87171; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;"
                          title={src.last_error}
                        >
                          {src.last_error}
                        </span>
                      )}
                    </div>
                  </div>

                  <div class="source-actions-group">
                    {src.id && (
                      <label
                        class="source-toggle-switch"
                        title={isActive ? 'Beslemeyi Devre Dışı Bırak' : 'Beslemeyi Etkinleştir'}
                      >
                        <input
                          type="checkbox"
                          checked={isActive}
                          disabled={togglingId === src.id}
                          onChange={() => handleToggle(src)}
                        />
                        <span class="source-toggle-slider" />
                      </label>
                    )}
                    <a
                      href={src.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="btn-link"
                      title="Besleme Bağlantısı"
                      style="font-size: 0.78rem; text-decoration: none; padding: 4px 8px;"
                    >
                      XML &rarr;
                    </a>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
