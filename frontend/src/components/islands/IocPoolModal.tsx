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
      const [all, ip, domain, sha256, md5] = await Promise.all([
        fetchIoCs({ limit: 1 }),
        fetchIoCs({ type: 'ip', limit: 1 }),
        fetchIoCs({ type: 'domain', limit: 1 }),
        fetchIoCs({ type: 'sha256', limit: 1 }),
        fetchIoCs({ type: 'md5', limit: 1 }),
      ]);
      setCounts({
        all: all.total || 0,
        ip: ip.total || 0,
        domain: domain.total || 0,
        sha256: sha256.total || 0,
        md5: md5.total || 0,
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
    <div style={{ display: 'flex', flexDirection: 'column', width: '100%', gap: '20px', paddingBottom: '40px' }}>
      {/* Üst Bilgi ve Filtreleme Kartı */}
      <div style={{ background: 'linear-gradient(135deg, rgba(14, 20, 34, 0.9) 0%, rgba(10, 15, 26, 0.95) 100%)', border: '1px solid rgba(56, 189, 248, 0.15)', borderRadius: 'var(--radius-lg)', padding: '24px', boxShadow: '0 4px 20px rgba(0, 0, 0, 0.4)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px', marginBottom: '18px' }}>
          <div>
            <h1 style={{ fontSize: '1.5rem', fontWeight: 700, color: '#fff', margin: '0 0 6px 0', letterSpacing: '-0.02em' }}>
              Tehdit Göstergeleri (IoC) Havuzu
            </h1>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', margin: 0 }}>
              Toplanan istihbarat haberlerinden otomatik ayıklanan IP, Domain ve Hash kayıtları.
            </p>
          </div>
          <div className="ioc-export-actions" style={{ display: 'flex', gap: '10px' }}>
            <a href="/api/iocs/export?format=txt" download="ctifeed-blocklist.txt" className="btn-secondary btn-export" style={{ padding: '8px 14px', borderRadius: 'var(--radius-md)', fontSize: '0.85rem', textDecoration: 'none' }}>
              <span>TXT İndir</span>
            </a>
            <a href="/api/iocs/export?format=csv" download="ctifeed-iocs.csv" className="btn-primary btn-export" style={{ padding: '8px 14px', borderRadius: 'var(--radius-md)', fontSize: '0.85rem', textDecoration: 'none' }}>
              <span>CSV İndir</span>
            </a>
          </div>
        </div>

        {/* Filtreleme Hapları (Pills) */}
        <div className="ioc-type-pills" style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '16px', paddingBottom: '14px', borderBottom: '1px solid rgba(56, 189, 248, 0.1)' }}>
          <button className={`pill ${selectedType === '' ? 'active' : ''}`} onClick={() => handleTypeChange('')} style={{ fontSize: '0.8rem', padding: '6px 12px', borderRadius: 'var(--radius-md)', cursor: 'pointer', border: selectedType === '' ? '1px solid #38bdf8' : '1px solid var(--bg-card-border)', background: selectedType === '' ? 'rgba(56, 189, 248, 0.15)' : '#070b14', color: selectedType === '' ? '#38bdf8' : 'var(--text-secondary)', fontWeight: selectedType === '' ? '600' : '500' }}>
            Tümü ({counts.all})
          </button>
          <button className={`pill pill-ip ${selectedType === 'ip' ? 'active' : ''}`} onClick={() => handleTypeChange('ip')} style={{ fontSize: '0.8rem', padding: '6px 12px', borderRadius: 'var(--radius-md)', cursor: 'pointer', border: selectedType === 'ip' ? '1px solid #38bdf8' : '1px solid var(--bg-card-border)', background: selectedType === 'ip' ? 'rgba(56, 189, 248, 0.15)' : '#070b14', color: selectedType === 'ip' ? '#38bdf8' : 'var(--text-secondary)', fontWeight: selectedType === 'ip' ? '600' : '500' }}>
            IP Adresleri ({counts.ip})
          </button>
          <button className={`pill pill-domain ${selectedType === 'domain' ? 'active' : ''}`} onClick={() => handleTypeChange('domain')} style={{ fontSize: '0.8rem', padding: '6px 12px', borderRadius: 'var(--radius-md)', cursor: 'pointer', border: selectedType === 'domain' ? '1px solid #38bdf8' : '1px solid var(--bg-card-border)', background: selectedType === 'domain' ? 'rgba(56, 189, 248, 0.15)' : '#070b14', color: selectedType === 'domain' ? '#38bdf8' : 'var(--text-secondary)', fontWeight: selectedType === 'domain' ? '600' : '500' }}>
            Alan Adları ({counts.domain})
          </button>
          <button className={`pill pill-hash ${selectedType === 'sha256' ? 'active' : ''}`} onClick={() => handleTypeChange('sha256')} style={{ fontSize: '0.8rem', padding: '6px 12px', borderRadius: 'var(--radius-md)', cursor: 'pointer', border: selectedType === 'sha256' ? '1px solid #38bdf8' : '1px solid var(--bg-card-border)', background: selectedType === 'sha256' ? 'rgba(56, 189, 248, 0.15)' : '#070b14', color: selectedType === 'sha256' ? '#38bdf8' : 'var(--text-secondary)', fontWeight: selectedType === 'sha256' ? '600' : '500' }}>
            SHA256 ({counts.sha256})
          </button>
          <button className={`pill pill-hash ${selectedType === 'md5' ? 'active' : ''}`} onClick={() => handleTypeChange('md5')} style={{ fontSize: '0.8rem', padding: '6px 12px', borderRadius: 'var(--radius-md)', cursor: 'pointer', border: selectedType === 'md5' ? '1px solid #38bdf8' : '1px solid var(--bg-card-border)', background: selectedType === 'md5' ? 'rgba(56, 189, 248, 0.15)' : '#070b14', color: selectedType === 'md5' ? '#38bdf8' : 'var(--text-secondary)', fontWeight: selectedType === 'md5' ? '600' : '500' }}>
            MD5 ({counts.md5})
          </button>
        </div>

        {/* Arama Çubuğu */}
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center' }}>
          <div style={{ position: 'relative', flex: 1 }}>
            <input
              type="text"
              className="ioc-search-input"
              placeholder="IoC değeri ara..."
              value={search}
              onInput={(e: any) => handleSearchChange(e.target.value)}
              style={{ width: '100%', padding: '11px 16px', background: '#070b14', border: '1px solid rgba(56, 189, 248, 0.2)', borderRadius: 'var(--radius-md)', color: '#fff', fontSize: '0.9rem', outline: 'none' }}
            />
          </div>
        </div>
      </div>

      {/* Tablo Kartı */}
      <div style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', width: '100%', overflow: 'hidden', boxShadow: '0 4px 20px rgba(0, 0, 0, 0.3)' }}>
        <div style={{ width: '100%', overflowX: 'auto' }}>
          {loading ? (
            <div style={{ textAlign: 'center', padding: '60px', color: 'var(--text-muted)', fontSize: '0.95rem' }}>
              Tehdit göstergeleri güvenli belleğe yükleniyor...
            </div>
          ) : (
            <table className="ioc-table" style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', tableLayout: 'fixed' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--bg-card-border)', background: 'rgba(14, 20, 34, 0.6)' }}>
                  <th style={{ width: '110px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Tür</th>
                  <th style={{ width: '240px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Gösterge (Değer)</th>
                  <th style={{ width: '160px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Kaynak</th>
                  <th style={{ padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>İlişkili Tehdit Bağlamı</th>
                  <th style={{ width: '120px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>İlk Tespit</th>
                  <th style={{ width: '100px', textAlign: 'center', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Eylem</th>
                </tr>
              </thead>
              <tbody>
                {iocs.map((ioc, idx) => (
                  <tr 
                    key={ioc.id || idx}
                    style={{ borderBottom: '1px solid var(--bg-card-border)', transition: 'background-color 0.15s ease' }}
                    onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(56, 189, 248, 0.03)')}
                    onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                  >
                    <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                      <span className={`ioc-type-badge ioc-badge-${ioc.type}`} style={{ fontSize: '0.75rem', padding: '4px 8px', borderRadius: '4px', display: 'inline-block' }}>{ioc.type.toUpperCase()}</span>
                    </td>
                    <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                      <span className="ioc-val" style={{ fontFamily: 'monospace', fontSize: '0.85rem', color: '#38bdf8', wordBreak: 'break-all' }}>{ioc.value}</span>
                    </td>
                    <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                      {ioc.source ? <span className="ioc-source-tag" style={{ fontSize: '0.78rem', padding: '4px 8px', background: 'rgba(255, 255, 255, 0.05)', borderRadius: '4px' }}>{ioc.source}</span> : '-'}
                    </td>
                    <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                      <div className="ioc-context" style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.4', display: '-webkit-box', WebkitLineClamp: '2', WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                        {ioc.threat_context || ioc.context || '-'}
                      </div>
                    </td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.8rem', padding: '16px 20px', verticalAlign: 'middle', whiteSpace: 'nowrap' }}>
                      {formatTimeAgo(ioc.first_seen || ioc.created_at)}
                    </td>
                    <td style={{ textAlign: 'center', padding: '16px 20px', verticalAlign: 'middle' }}>
                      <button className={`btn-copy-ioc ${copiedVal === ioc.value ? 'copied' : ''}`} onClick={() => handleCopy(ioc.value)} style={{ padding: '6px 12px', fontSize: '0.8rem', borderRadius: 'var(--radius-md)', cursor: 'pointer', background: copiedVal === ioc.value ? '#10b981' : 'rgba(56, 189, 248, 0.1)', color: copiedVal === ioc.value ? '#fff' : '#38bdf8', border: '1px solid rgba(56, 189, 248, 0.2)', transition: 'all 0.2s' }}>
                        <span>{copiedVal === ioc.value ? 'Kopyalandı!' : 'Kopyala'}</span>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {!loading && iocs.length === 0 && (
            <div className="ioc-empty-state" style={{ textAlign: 'center', padding: '60px', color: 'var(--text-muted)' }}>
              <p>Eşleşen IoC kaydı bulunamadı.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}