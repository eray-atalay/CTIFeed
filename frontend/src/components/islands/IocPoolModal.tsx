import { useState, useEffect, useRef } from 'preact/hooks';
import { activeModal, closeModal, formatTimeAgo } from '../../services/store';
import { fetchIoCs } from '../../services/api';
import type { IoCItem, IoCCounts } from '../../types/cti';

export default function IocPoolModal() {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [iocs, setIocs] = useState<IoCItem[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [selectedType, setSelectedType] = useState<string>('');
  const [search, setSearch] = useState<string>('');
  const [debouncedSearch, setDebouncedSearch] = useState<string>('');
  const [counts, setCounts] = useState<IoCCounts>({ all: 0, ip: 0, domain: 0, sha256: 0, md5: 0 });
  const [copiedVal, setCopiedVal] = useState<string | null>(null);

  const searchTimerRef = useRef<any>(null);

  useEffect(() => {
    const unsub = activeModal.subscribe((val) => {
      const open = val === 'ioc';
      setIsOpen(open);
      if (open) {
        loadCounts();
        loadIocs('', '');
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

  const loadCounts = async () => {
    try {
      const res = await fetchIoCs({ limit: 1000 });
      const all = res.iocs || [];
      setCounts({
        all: all.length,
        ip: all.filter((i) => i.type === 'ip').length,
        domain: all.filter((i) => i.type === 'domain').length,
        sha256: all.filter((i) => i.type === 'sha256').length,
        md5: all.filter((i) => i.type === 'md5').length,
      });
    } catch (err) {
      console.error('IoC sayıları yüklenemedi:', err);
    }
  };

  const loadIocs = async (type: string, query: string) => {
    setLoading(true);
    try {
      const res = await fetchIoCs({
        type: type || undefined,
        search: query || undefined,
        limit: 250,
      });
      setIocs(res.iocs || []);
    } catch (err) {
      console.error('IoC listesi yüklenemedi:', err);
      setIocs([]);
    } finally {
      setLoading(false);
    }
  };

  const handleTypeChange = (type: string) => {
    setSelectedType(type);
    loadIocs(type, debouncedSearch);
  };

  const handleSearchChange = (val: string) => {
    setSearch(val);
    clearTimeout(searchTimerRef.current);
    searchTimerRef.current = setTimeout(() => {
      const q = val.trim();
      setDebouncedSearch(q);
      loadIocs(selectedType, q);
    }, 250);
  };

  const handleCopy = (val: string) => {
    navigator.clipboard.writeText(val).then(() => {
      setCopiedVal(val);
      setTimeout(() => setCopiedVal(null), 1500);
    });
  };

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
          <div class="ioc-modal-header-top">
            <div>
              <h2>Tehdit Göstergeleri (IoC Havuzu)</h2>
              <p class="modal-sub">
                Toplanan istihbarat haberlerinden otomatik ayıklanan ve doğrulanmış IP, Alan Adı ve Hash kayıtları.
              </p>
            </div>
            <div class="ioc-export-actions">
              <a
                href="/api/iocs/export?format=txt"
                download="ctifeed-blocklist.txt"
                class="btn-secondary btn-export"
                title="Firewall / EDR için saf blok listesi indir"
              >
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                  <polyline points="7 10 12 15 17 10"></polyline>
                  <line x1="12" y1="15" x2="12" y2="3"></line>
                </svg>
                <span>TXT İndir</span>
              </a>
              <a
                href="/api/iocs/export?format=csv"
                download="ctifeed-iocs.csv"
                class="btn-primary btn-export"
                title="Excel / SIEM için CSV raporu indir"
              >
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                  <polyline points="7 10 12 15 17 10"></polyline>
                  <line x1="12" y1="15" x2="12" y2="3"></line>
                </svg>
                <span>CSV İndir</span>
              </a>
            </div>
          </div>

          <div class="ioc-filters-bar">
            <div class="ioc-type-pills">
              <button
                class={`pill ${selectedType === '' ? 'active' : ''}`}
                onClick={() => handleTypeChange('')}
              >
                Tümü ({counts.all})
              </button>
              <button
                class={`pill pill-ip ${selectedType === 'ip' ? 'active' : ''}`}
                onClick={() => handleTypeChange('ip')}
              >
                IP Adresleri ({counts.ip})
              </button>
              <button
                class={`pill pill-domain ${selectedType === 'domain' ? 'active' : ''}`}
                onClick={() => handleTypeChange('domain')}
              >
                Alan Adları ({counts.domain})
              </button>
              <button
                class={`pill pill-hash ${selectedType === 'sha256' ? 'active' : ''}`}
                onClick={() => handleTypeChange('sha256')}
              >
                SHA256 ({counts.sha256})
              </button>
              <button
                class={`pill pill-hash ${selectedType === 'md5' ? 'active' : ''}`}
                onClick={() => handleTypeChange('md5')}
              >
                MD5 ({counts.md5})
              </button>
            </div>
            <input
              type="text"
              class="ioc-search-input"
              placeholder="IoC değeri veya haber içeriğinde ara..."
              value={search}
              onInput={(e: any) => handleSearchChange(e.target.value)}
            />
          </div>
        </div>

        <div class="ioc-table-container">
          {loading ? (
            <div style="text-align: center; padding: 40px; color: var(--text-muted);">
              Tehdit göstergeleri yükleniyor...
            </div>
          ) : (
            <table class="ioc-table">
              <thead>
                <tr>
                  <th style="width: 90px;">Tür</th>
                  <th>Gösterge (Değer)</th>
                  <th style="width: 140px;">Kaynak</th>
                  <th>İlişkili Tehdit Bağlamı</th>
                  <th style="width: 110px;">İlk Tespit</th>
                  <th style="width: 80px; text-align: center;">Eylem</th>
                </tr>
              </thead>
              <tbody>
                {iocs.map((ioc, idx) => {
                  const threatContext = ioc.threat_context || ioc.context || '';
                  const timeDisplay = formatTimeAgo(ioc.first_seen || ioc.created_at);

                  return (
                    <tr key={ioc.id || idx}>
                      <td>
                        <span class={`ioc-type-badge ioc-badge-${ioc.type}`}>
                          {ioc.type.toUpperCase()}
                        </span>
                      </td>
                      <td>
                        <span class="ioc-val">{ioc.value}</span>
                      </td>
                      <td>
                        {ioc.source ? (
                          <span class="ioc-source-tag">{ioc.source}</span>
                        ) : (
                          <span style="color: var(--text-muted);">-</span>
                        )}
                      </td>
                      <td>
                        <div class="ioc-context">
                          {threatContext ? (
                            ioc.url ? (
                              <a
                                href={ioc.url}
                                target="_blank"
                                rel="noopener noreferrer"
                                class="ioc-news-link"
                              >
                                <span>{threatContext}</span>
                              </a>
                            ) : (
                              <span>{threatContext}</span>
                            )
                          ) : (
                            <span style="color: var(--text-muted);">-</span>
                          )}
                        </div>
                      </td>
                      <td style="color: var(--text-muted); font-size: 0.8rem;">
                        {timeDisplay}
                      </td>
                      <td style="text-align: center;">
                        <button
                          class={`btn-copy-ioc ${copiedVal === ioc.value ? 'copied' : ''}`}
                          onClick={() => handleCopy(ioc.value)}
                          title="Panoya Kopyala"
                        >
                          <span>{copiedVal === ioc.value ? 'Kopyalandı!' : 'Kopyala'}</span>
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}

          {!loading && iocs.length === 0 && (
            <div class="ioc-empty-state">
              <p>Arama kriterine uygun IoC kaydı bulunamadı.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
