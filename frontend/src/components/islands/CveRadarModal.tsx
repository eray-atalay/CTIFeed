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
    // Yalnızca Telegram @cveNotify kanalından gelen veriler kabul edilir.
    // breachdetect ve diğer CTI kaynakları kesinlikle dahil edilmez.
    if (src.includes('breachdetect')) return false;
    return src.includes('cvenotify') || src.includes('cve notify');
  };

  const loadCveData = async () => {
    setLoading(true);
    try {
      let res = await fetchArticles({ source: 'cvenotify', limit: 300, sort: 'date' });
      let list = (res.articles || []).filter(isCveNotifyArticle);

      // Eğer source filtresiyle backend'den kayıt dönmediyse tüm verileri çekip filtrele
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
    <div style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', padding: '20px' }}>
      <div className="ioc-filters-bar" style={{ display: 'flex', gap: '12px', alignItems: 'center', marginBottom: '16px', flexWrap: 'wrap' }}>
        <input
          type="text"
          className="ioc-search-input"
          placeholder="CVE kodu veya zafiyet adı ara (örn: CVE-2026, Fortinet, RCE)..."
          value={search}
          onInput={(e: any) => handleSearchChange(e.target.value)}
          style={{ flex: 1, minWidth: '240px' }}
        />
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>
            Toplam: <strong style={{ color: '#38bdf8' }}>{filteredCves.length}</strong>
          </span>
          <div className="select-wrapper">
            <select
              value={sortOrder}
              onChange={(e: any) => setSortOrder(e.target.value)}
              style={{ background: '#0e1422', border: '1px solid var(--bg-card-border)', color: 'var(--text-secondary)', padding: '7px 12px', borderRadius: 'var(--radius-md)', fontSize: '0.825rem', cursor: 'pointer' }}
            >
              <option value="desc">En Yeni Tarih</option>
              <option value="asc">En Eski Tarih</option>
            </select>
          </div>
        </div>
      </div>

      <div className="ioc-table-container" style={{ maxHeight: '70vh' }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
            CVE kayıtları yükleniyor...
          </div>
        ) : (
          <table className="ioc-table">
            <thead>
              <tr>
                <th style={{ width: '150px' }}>CVE Kodu</th>
                <th>Zafiyet &amp; Tehdit Özeti</th>
                <th style={{ width: '190px' }}>Kaynak</th>
                <th style={{ width: '120px' }}>Yayınlanma</th>
                <th style={{ width: '80px', textAlign: 'center' }}>Bağlantı</th>
              </tr>
            </thead>
            <tbody>
              {filteredCves.map((art) => {
                const cveTags = (art.tags || []).filter((t) => t.toUpperCase().startsWith('CVE-'));
                const match = (art.title + ' ' + (art.summary || '')).match(/CVE-\d{4}-\d{4,7}/i);
                const cveLabel = cveTags.length > 0 ? cveTags.join(', ') : (match ? match[0].toUpperCase() : 'CVE Bildirimi');
                const timeAgo = formatTimeAgo(art.published_at);

                return (
                  <tr key={art.id}>
                    <td>
                      <span className="tag-item tag-cve" style={{ fontSize: '0.8rem' }}>
                        {cveLabel}
                      </span>
                    </td>
                    <td>
                      <div style={{ fontWeight: 600, color: '#fff', marginBottom: '4px' }}>
                        {art.title}
                      </div>
                      <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', lineHeight: '1.4' }}>
                        {art.summary || ''}
                      </div>
                    </td>
                    <td>
                      <span className="ioc-source-tag">{art.source}</span>
                    </td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                      {timeAgo}
                    </td>
                    <td style={{ textAlign: 'center' }}>
                      <a
                        href={art.link}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="btn-link"
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
          <div className="ioc-empty-state">
            <p>Arama kriterine uygun veya @cvenotify kanalından kaydedilmiş CVE bildirimi bulunamadı.</p>
          </div>
        )}
      </div>
    </div>
  );
}