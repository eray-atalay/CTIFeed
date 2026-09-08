import { useState, useEffect } from 'preact/hooks';
import { activeModal, closeModal } from '../../services/store';
import { fetchSources } from '../../services/api';
import type { SourceInfo } from '../../types/cti';

export default function SourcesModal() {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [sources, setSources] = useState<SourceInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    const unsub = activeModal.subscribe((val) => {
      const open = val === 'sources';
      setIsOpen(open);
      if (open) {
        setLoading(true);
        fetchSources()
          .then((res) => setSources(res.sources || []))
          .catch((err) => console.error('Kaynaklar yüklenirken hata:', err))
          .finally(() => setLoading(false));
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

  if (!isOpen) return null;

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
          <h2>İzlenen CTI Besleme Kaynakları ({sources.length})</h2>
          <p class="modal-sub">
            Siber tehditler, kritik sistem zafiyetleri ve Türkiye odağı için sürekli analiz edilen kaynaklar.
          </p>
        </div>

        {loading ? (
          <div style="text-align: center; padding: 40px; color: var(--text-muted);">
            Kaynaklar yükleniyor...
          </div>
        ) : (
          <div class="sources-list">
            {sources.map((src) => (
              <div key={src.name} class="source-item-card">
                <div class="source-info">
                  <h5>{src.name}</h5>
                  <span class="source-cat">{src.category || 'Genel CTI'}</span>
                </div>
                <a
                  href={src.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="btn-link"
                  title="RSS XML Besleme Bağlantısı"
                >
                  Besleme XML &rarr;
                </a>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
