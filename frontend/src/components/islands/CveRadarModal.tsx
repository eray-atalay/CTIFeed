import { useState, useEffect, useMemo, useRef } from 'preact/hooks';
import { formatTimeAgo } from '../../services/store';
import { fetchArticles } from '../../services/api';
import type { Article } from '../../types/cti';

export default function CveRadarModal() {
  const [allArticles, setAllArticles] = useState<Article[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [search, setSearch] = useState<string>('');
  const [debouncedSearch, setDebouncedSearch] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'desc' | 'asc'>('desc');

  const searchTimerRef = useRef<any>(null);

  useEffect(() => {
    loadCveData();
  }, []);

  const isCveNotifyArticle = (a: Article) => {
    const src = (a.source || '').toLowerCase();
    if (src.includes('breachdetect')) return false;
    return src.includes('cvenotify') || src.includes('cve notify');
  };

  const loadCveData = async () => {
    setLoading(true);
    try {
      let res = await fetchArticles({ source: 'cvenotify', limit: 300, sort: 'date' });
      let list = (res.articles || []).filter(isCveNotifyArticle);

      if (list.length === 0) {
        const allRes = await fetchArticles({ limit: 300, sort: 'date' });
        list = (allRes.articles || []).filter(isCveNotifyArticle);
      }

      setAllArticles(list);
    } catch (err) {
      console.error('CVE verisi alınırken hata:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSearchChange = (val: string) => {
    setSearch(val);
    clearTimeout(searchTimerRef.current);
    searchTimerRef.current = setTimeout(() => {
      setDebouncedSearch(val.trim().toLowerCase());
    }, 250);
  };

  const filteredCves = useMemo(() => {
    let items = allArticles.filter(isCveNotifyArticle);

    if (debouncedSearch) {
      items = items.filter(
        (a) =>
          (a.title && a.title.toLowerCase().includes(debouncedSearch)) ||
          (a.summary && a.summary.toLowerCase().includes(debouncedSearch)) ||
          (a.tags && a.tags.some((t) => t.toLowerCase().includes(debouncedSearch)))
      );
    }

    items.sort((a, b) => {
      const dateA = new Date(a.published_at).getTime();
      const dateB = new Date(b.published_at).getTime();
      return sortOrder === 'asc' ? dateA - dateB : dateB - dateA;
    });

    return items;
  }, [allArticles, debouncedSearch, sortOrder]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', width: '100%', gap: '20px', paddingBottom: '40px' }}>
      {/* Üst Bilgi ve Filtreleme Kartı */}
      <div style={{ background: 'linear-gradient(135deg, rgba(14, 20, 34, 0.9) 0%, rgba(10, 15, 26, 0.95) 100%)', border: '1px solid rgba(56, 189, 248, 0.15)', borderRadius: 'var(--radius-lg)', padding: '24px', boxShadow: '0 4px 20px rgba(0, 0, 0, 0.4)' }}>
        <div style={{ marginBottom: '16px' }}>
          <h1 style={{ fontSize: '1.5rem', fontWeight: 700, color: '#fff', margin: '0 0 6px 0', letterSpacing: '-0.02em' }}>
            CVE Zafiyet Akışı &amp; Bildirimleri
          </h1>
          <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', margin: 0 }}>
            Telegram @cvenotify kanalından toplanan anlık CVE güvenlik açıkları ve zafiyet bildirimleri.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '16px', alignItems: 'center', flexWrap: 'wrap', paddingTop: '8px', borderTop: '1px solid var(--bg-card-border)' }}>
          <div style={{ position: 'relative', flex: 1, minWidth: '280px' }}>
            <input
              type="text"
              className="ioc-search-input"
              placeholder="CVE kodu veya zafiyet adı ara (örn: CVE-2026, Fortinet, RCE)..."
              value={search}
              onInput={(e: any) => handleSearchChange(e.target.value)}
              style={{ width: '100%', padding: '11px 16px', background: '#070b14', border: '1px solid rgba(56, 189, 248, 0.2)', borderRadius: 'var(--radius-md)', color: '#fff', fontSize: '0.9rem', outline: 'none', transition: 'border-color 0.2s' }}
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
            <div style={{ background: 'rgba(56, 189, 248, 0.08)', padding: '8px 14px', borderRadius: 'var(--radius-md)', border: '1px solid rgba(56, 189, 248, 0.15)' }}>
              <span style={{ fontSize: '0.825rem', color: 'var(--text-muted)' }}>Toplam Kayıt: </span>
              <strong style={{ color: '#38bdf8', fontSize: '0.9rem', marginLeft: '4px' }}>{filteredCves.length}</strong>
            </div>
            <div className="select-wrapper">
              <select
                value={sortOrder}
                onChange={(e: any) => setSortOrder(e.target.value)}
                style={{ background: '#070b14', border: '1px solid rgba(56, 189, 248, 0.2)', color: 'var(--text-secondary)', padding: '10px 14px', borderRadius: 'var(--radius-md)', fontSize: '0.85rem', cursor: 'pointer', outline: 'none' }}
              >
                <option value="desc">En Yeni Tarih</option>
                <option value="asc">En Eski Tarih</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      {/* Tablo Kartı */}
      <div style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', width: '100%', overflow: 'hidden', boxShadow: '0 4px 20px rgba(0, 0, 0, 0.3)' }}>
        <div style={{ width: '100%', overflowX: 'auto' }}>
          {loading ? (
            <div style={{ textAlign: 'center', padding: '60px', color: 'var(--text-muted)', fontSize: '0.95rem' }}>
              CVE kayıtları güvenli belleğe yükleniyor...
            </div>
          ) : (
            <table className="ioc-table" style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', tableLayout: 'fixed' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--bg-card-border)', background: 'rgba(14, 20, 34, 0.6)' }}>
                  <th style={{ width: '180px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>CVE Kodu</th>
                  <th style={{ padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Zafiyet &amp; Tehdit Özeti</th>
                  <th style={{ width: '170px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Kaynak</th>
                  <th style={{ width: '120px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Yayınlanma</th>
                  <th style={{ width: '100px', textAlign: 'center', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Bağlantı</th>
                </tr>
              </thead>
              <tbody>
                {filteredCves.map((art) => {
                  const cveTags = (art.tags || []).filter((t) => t.toUpperCase().startsWith('CVE-'));
                  const match = (art.title + ' ' + (art.summary || '')).match(/CVE-\d{4}-\d{4,7}/i);
                  const cveLabel = cveTags.length > 0 ? cveTags.join(', ') : (match ? match[0].toUpperCase() : 'CVE Bildirimi');
                  const timeAgo = formatTimeAgo(art.published_at);

                  return (
                    <tr 
                      key={art.id} 
                      style={{ borderBottom: '1px solid var(--bg-card-border)', transition: 'background-color 0.15s ease' }}
                      onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(56, 189, 248, 0.03)')}
                      onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                    >
                      <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                        <span className="tag-item tag-cve" style={{ fontSize: '0.8rem', padding: '5px 10px', borderRadius: '4px', whiteSpace: 'nowrap', display: 'inline-block' }}>
                          {cveLabel}
                        </span>
                      </td>
                      <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                        <div style={{ fontWeight: 600, color: '#fff', marginBottom: '6px', fontSize: '0.9rem', lineHeight: '1.35' }}>
                          {art.title}
                        </div>
                        <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.4', display: '-webkit-box', WebkitLineClamp: '2', WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                          {art.summary || ''}
                        </div>
                      </td>
                      <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                        <span className="ioc-source-tag" style={{ fontSize: '0.78rem', padding: '4px 8px', background: 'rgba(255, 255, 255, 0.05)', borderRadius: '4px' }}>{art.source}</span>
                      </td>
                      <td style={{ color: 'var(--text-muted)', fontSize: '0.8rem', padding: '16px 20px', verticalAlign: 'middle', whiteSpace: 'nowrap' }}>
                        {timeAgo}
                      </td>
                      <td style={{ textAlign: 'center', padding: '16px 20px', verticalAlign: 'middle' }}>
                        <a
                          href={art.link}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="btn-link"
                          style={{ fontSize: '0.85rem', fontWeight: 500, color: '#38bdf8', textDecoration: 'none' }}
                        >
                          Rapor &rarr;
                        </a>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}

          {!loading && filteredCves.length === 0 && (
            <div className="ioc-empty-state" style={{ textAlign: 'center', padding: '60px', color: 'var(--text-muted)' }}>
              <p>Arama kriterine uygun veya @cvenotify kanalından kaydedilmiş CVE bildirimi bulunamadı.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}