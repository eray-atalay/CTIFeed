import { useState, useEffect, useMemo, useRef } from 'preact/hooks';
import { formatTimeAgo } from '../../services/store';
import { fetchArticles } from '../../services/api';
import type { Article } from '../../types/cti';

interface CategoryPill {
  id: string;
  label: string;
}

const CATEGORIES: CategoryPill[] = [
  { id: '', label: 'Tüm Kayıtlar' },
  { id: 'ransomware', label: '🔴 Fidye Yazılımı (Ransomware)' },
  { id: 'combolist', label: '🟠 Combolist & Hesaplar' },
  { id: 'data-leak', label: '🔵 Veri Sızıntısı' },
  { id: 'initial-access', label: '🟣 İlk Erişim (Access)' },
  { id: 'database-dump', label: '🟢 Veritabanı Dökümü' },
  { id: 'infra-proxy', label: '⚪ Proxy & Altyapı' },
];

export default function BreachRadarModal() {
  const [allArticles, setAllArticles] = useState<Article[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [search, setSearch] = useState<string>('');
  const [debouncedSearch, setDebouncedSearch] = useState<string>('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'desc' | 'asc'>('desc');

  const searchTimerRef = useRef<any>(null);

  useEffect(() => {
    loadBreachData();
  }, []);

  const loadBreachData = async () => {
    setLoading(true);
    try {
      let res = await fetchArticles({ source: 'breachdetect', limit: 300, sort: 'date' });
      if (!res.articles || res.articles.length === 0) {
        res = await fetchArticles({ limit: 300, sort: 'date' });
      }
      setAllArticles(res.articles || []);
    } catch (err) {
      console.error('Sızıntı verisi alınırken hata:', err);
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

  const getCategoryInfo = (art: Article) => {
    const tags = art.tags || [];
    const text = (art.title + ' ' + (art.summary || '') + ' ' + tags.join(' ')).toLowerCase();

    if (tags.includes('ransomware') || text.includes('ransomware') || text.includes('lockbit') || text.includes('published a new victim')) {
      return {
        category: 'ransomware',
        label: 'FIDYE YAZILIMI',
        color: '#f87171',
        bg: 'rgba(239, 68, 68, 0.2)',
        border: 'rgba(239, 68, 68, 0.35)',
      };
    }
    if (tags.includes('combolist') || text.includes('combolist') || text.includes('combo list') || text.includes('webmail login') || text.includes('stealer') || text.includes('email:pass')) {
      return {
        category: 'combolist',
        label: 'COMBOLIST / HESAP',
        color: '#fb923c',
        bg: 'rgba(249, 115, 22, 0.2)',
        border: 'rgba(249, 115, 22, 0.35)',
      };
    }
    if (tags.includes('initial-access') || text.includes('network access') || text.includes('rdp access') || text.includes('vpn access') || text.includes('initial access') || text.includes('domain admin')) {
      return {
        category: 'initial-access',
        label: 'ERISIM SATISI',
        color: '#c084fc',
        bg: 'rgba(192, 132, 252, 0.2)',
        border: 'rgba(192, 132, 252, 0.35)',
      };
    }
    if (tags.includes('database-dump') || text.includes('sql dump') || text.includes('database dump') || text.includes('db dump')) {
      return {
        category: 'database-dump',
        label: 'VERITABANI DOKUMU',
        color: '#38bdf8',
        bg: 'rgba(56, 189, 248, 0.2)',
        border: 'rgba(56, 189, 248, 0.35)',
      };
    }
    if (tags.includes('infra-proxy') || text.includes('socks5') || text.includes('proxies') || text.includes('botnet')) {
      return {
        category: 'infra-proxy',
        label: 'ALTYAPI / PROXY',
        color: '#94a3b8',
        bg: 'rgba(148, 163, 184, 0.2)',
        border: 'rgba(148, 163, 184, 0.35)',
      };
    }
    return {
      category: 'data-leak',
      label: 'VERI SIZINTISI',
      color: '#fca5a5',
      bg: 'rgba(244, 63, 94, 0.2)',
      border: 'rgba(244, 63, 94, 0.35)',
    };
  };

  const getForumAndActor = (art: Article) => {
    let forum = '';
    let actor = '';
    for (const tag of art.tags || []) {
      if (tag.startsWith('forum:')) {
        forum = tag.replace('forum:', '');
      } else if (tag.startsWith('actor:')) {
        actor = tag.replace('actor:', '');
      }
    }
    return { forum, actor };
  };

  const filteredBreaches = useMemo(() => {
    let items = allArticles.filter((a) => {
      const src = (a.source || '').toLowerCase();
      const link = (a.link || '').toLowerCase();
      const isTelegram = src.startsWith('telegram:') || link.includes('t.me/');
      const isBreach = src.includes('breachdetect') || link.includes('breachdetect');
      return isTelegram && isBreach;
    });

    if (selectedCategory) {
      items = items.filter((a) => {
        const info = getCategoryInfo(a);
        return info.category === selectedCategory;
      });
    }

    if (debouncedSearch) {
      items = items.filter((a) => {
        const title = (a.title || '').toLowerCase();
        const summary = (a.summary || '').toLowerCase();
        const src = (a.source || '').toLowerCase();
        const tags = (a.tags || []).join(' ').toLowerCase();
        return (
          title.includes(debouncedSearch) ||
          summary.includes(debouncedSearch) ||
          src.includes(debouncedSearch) ||
          tags.includes(debouncedSearch)
        );
      });
    }

    items.sort((a, b) => {
      const dateA = new Date(a.published_at).getTime();
      const dateB = new Date(b.published_at).getTime();
      return sortOrder === 'asc' ? dateA - dateB : dateB - dateA;
    });

    return items;
  }, [allArticles, selectedCategory, debouncedSearch, sortOrder]);

  return (
    <div style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', padding: '20px' }}>
      <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginBottom: '12px', padding: '4px 0' }}>
        {CATEGORIES.map((cat) => {
          const active = selectedCategory === cat.id;
          return (
            <button
              key={cat.id}
              type="button"
              onClick={() => setSelectedCategory(cat.id)}
              style={{
                fontSize: '0.78rem',
                padding: '5px 11px',
                borderRadius: '6px',
                cursor: 'pointer',
                transition: 'all 0.2s ease',
                border: active ? '1px solid #3b82f6' : '1px solid var(--bg-card-border)',
                background: active ? '#1e3a5f' : '#182030',
                color: active ? '#fff' : 'var(--text-secondary)',
                fontWeight: active ? '600' : '500',
              }}
            >
              {cat.label}
            </button>
          );
        })}
      </div>

      <div className="ioc-filters-bar" style={{ display: 'flex', gap: '12px', alignItems: 'center', marginBottom: '16px' }}>
        <input
          type="text"
          className="ioc-search-input"
          placeholder="Hedef kurum, domain, dark web forumu veya aktör ara..."
          value={search}
          onInput={(e: any) => handleSearchChange(e.target.value)}
          style={{ flex: 1 }}
        />
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

      <div className="ioc-table-container" style={{ maxHeight: '70vh' }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
            Dark Web ve sızıntı kayıtları yükleniyor...
          </div>
        ) : (
          <table className="ioc-table">
            <thead>
              <tr>
                <th style={{ width: '155px' }}>Kategori</th>
                <th>Hedef Kurum &amp; Dark Web Özeti</th>
                <th style={{ width: '200px' }}>Kaynak &amp; Forum</th>
                <th style={{ width: '110px' }}>Yayınlanma</th>
                <th style={{ width: '80px', textAlign: 'center' }}>Bağlantı</th>
              </tr>
            </thead>
            <tbody>
              {filteredBreaches.map((art) => {
                const timeAgo = formatTimeAgo(art.published_at);
                const catInfo = getCategoryInfo(art);
                const { forum, actor } = getForumAndActor(art);

                return (
                  <tr key={art.id || art.link}>
                    <td>
                      <span
                        style={{
                          display: 'inline-block',
                          fontSize: '0.72rem',
                          fontWeight: '600',
                          padding: '3px 8px',
                          borderRadius: '4px',
                          background: catInfo.bg,
                          color: catInfo.color,
                          border: `1px solid ${catInfo.border}`,
                          letterSpacing: '0.02em',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {catInfo.label}
                      </span>
                    </td>
                    <td>
                      <div style={{ fontWeight: 600, color: '#fff', marginBottom: '4px', fontSize: '0.88rem' }}>
                        {art.title}
                      </div>
                      <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', lineHeight: '1.4', wordBreak: 'break-word' }}>
                        {art.summary || ''}
                      </div>
                    </td>
                    <td>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', alignItems: 'flex-start' }}>
                        <span className="ioc-source-tag" style={{ fontSize: '0.72rem' }}>{art.source}</span>
                        {forum && (
                          <span
                            style={{ fontSize: '0.7rem', color: '#cbd5e1', background: 'rgba(255,255,255,0.08)', padding: '1px 6px', borderRadius: '4px', border: '1px solid rgba(255,255,255,0.12)' }}
                            title="Dark Web Forum Kaynağı"
                          >
                            🌐 {forum}
                          </span>
                        )}
                        {actor && (
                          <span
                            style={{ fontSize: '0.68rem', color: '#fdba74', background: 'rgba(251,146,60,0.12)', padding: '1px 6px', borderRadius: '4px', border: '1px solid rgba(251,146,60,0.25)' }}
                            title="Tehdit Aktörü / Yazar"
                          >
                            👤 {actor}
                          </span>
                        )}
                      </div>
                    </td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.8rem', whiteSpace: 'nowrap' }}>
                      {timeAgo}
                    </td>
                    <td style={{ textAlign: 'center' }}>
                      <a
                        href={art.link}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="btn-link"
                      >
                        Kanal &rarr;
                      </a>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}

        {!loading && filteredBreaches.length === 0 && (
          <div className="ioc-empty-state">
            <p>Seçilen kategori veya arama kriterine uygun dark web / sızıntı kaydı bulunamadı.</p>
          </div>
        )}
      </div>
    </div>
  );
}