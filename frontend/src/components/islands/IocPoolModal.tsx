import { useState, useEffect, useRef } from 'preact/hooks';
import { formatTimeAgo } from '../../services/store';
import { fetchIoCs } from '../../services/api';
import type { IoCItem, IoCCounts } from '../../types/cti';

export default function IocPoolModal() {
  const [iocs, setIocs] = useState<IoCItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [selectedType, setSelectedType] = useState<string>('');
  const [search, setSearch] = useState<string>('');
  const [debouncedSearch, setDebouncedSearch] = useState<string>('');
  const [counts, setCounts] = useState<IoCCounts>({ all: 0, ip: 0, domain: 0, sha256: 0, md5: 0 });
  const [copiedVal, setCopiedVal] = useState<string | null>(null);

  const searchTimerRef = useRef<any>(null);

  useEffect(() => {
    loadCounts();
    loadIocs('', '');
  }, []);

  const isNonTelegramIoC = (i: IoCItem) => {
    const src = (i.source || '').toLowerCase();
    return !src.includes('telegram');
  };

  const loadCounts = async () => {
    try {
      const res = await fetchIoCs({ limit: 1000 });
      const all = (res.iocs || []).filter(isNonTelegramIoC);
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
      const list = (res.iocs || []).filter(isNonTelegramIoC);
      setIocs(list);
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

  return (
    <div style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', padding: '20px' }}>
      <div className="ioc-modal-header-top" style={{ marginBottom: '16px' }}>
        <div className="ioc-export-actions">
          <a href="/api/iocs/export?format=txt" download="ctifeed-blocklist.txt" className="btn-secondary btn-export">
            <span>TXT İndir</span>
          </a>
          <a href="/api/iocs/export?format=csv" download="ctifeed-iocs.csv" className="btn-primary btn-export">
            <span>CSV İndir</span>
          </a>
        </div>
      </div>

      <div className="ioc-filters-bar" style={{ marginBottom: '16px' }}>
        <div className="ioc-type-pills">
          <button className={`pill ${selectedType === '' ? 'active' : ''}`} onClick={() => handleTypeChange('')}>
            Tümü ({counts.all})
          </button>
          <button className={`pill pill-ip ${selectedType === 'ip' ? 'active' : ''}`} onClick={() => handleTypeChange('ip')}>
            IP Adresleri ({counts.ip})
          </button>
          <button className={`pill pill-domain ${selectedType === 'domain' ? 'active' : ''}`} onClick={() => handleTypeChange('domain')}>
            Alan Adları ({counts.domain})
          </button>
          <button className={`pill pill-hash ${selectedType === 'sha256' ? 'active' : ''}`} onClick={() => handleTypeChange('sha256')}>
            SHA256 ({counts.sha256})
          </button>
          <button className={`pill pill-hash ${selectedType === 'md5' ? 'active' : ''}`} onClick={() => handleTypeChange('md5')}>
            MD5 ({counts.md5})
          </button>
        </div>
        <input
          type="text"
          className="ioc-search-input"
          placeholder="IoC değeri ara..."
          value={search}
          onInput={(e: any) => handleSearchChange(e.target.value)}
        />
      </div>

      <div className="ioc-table-container" style={{ maxHeight: '70vh' }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
            Tehdit göstergeleri yükleniyor...
          </div>
        ) : (
          <table className="ioc-table">
            <thead>
              <tr>
                <th style={{ width: '90px' }}>Tür</th>
                <th>Gösterge (Değer)</th>
                <th style={{ width: '140px' }}>Kaynak</th>
                <th>İlişkili Tehdit Bağlamı</th>
                <th style={{ width: '110px' }}>İlk Tespit</th>
                <th style={{ width: '80px', textAlign: 'center' }}>Eylem</th>
              </tr>
            </thead>
            <tbody>
              {iocs.map((ioc, idx) => (
                <tr key={ioc.id || idx}>
                  <td>
                    <span className={`ioc-type-badge ioc-badge-${ioc.type}`}>{ioc.type.toUpperCase()}</span>
                  </td>
                  <td>
                    <span className="ioc-val">{ioc.value}</span>
                  </td>
                  <td>{ioc.source ? <span className="ioc-source-tag">{ioc.source}</span> : '-'}</td>
                  <td>
                    <div className="ioc-context">
                      {ioc.threat_context || ioc.context || '-'}
                    </div>
                  </td>
                  <td style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                    {formatTimeAgo(ioc.first_seen || ioc.created_at)}
                  </td>
                  <td style={{ textAlign: 'center' }}>
                    <button className={`btn-copy-ioc ${copiedVal === ioc.value ? 'copied' : ''}`} onClick={() => handleCopy(ioc.value)}>
                      <span>{copiedVal === ioc.value ? 'Kopyalandı!' : 'Kopyala'}</span>
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {!loading && iocs.length === 0 && (
          <div className="ioc-empty-state">
            <p>Eşleşen IoC kaydı bulunamadı.</p>
          </div>
        )}
      </div>
    </div>
  );
}