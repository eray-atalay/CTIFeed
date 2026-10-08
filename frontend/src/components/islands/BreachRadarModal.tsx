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
    <div style={{ display: 'flex', flexDirection: 'column', width: '100%', gap: '20px', paddingBottom: '40px' }}>
      {/* Üst Bilgi ve Filtreleme Kartı (CVE Radarı Tarzı) */}
      <div style={{ background: 'linear-gradient(135deg, rgba(14, 20, 34, 0.9) 0%, rgba(10, 15, 26, 0.95) 100%)', border: '1px solid rgba(56, 189, 248, 0.15)', borderRadius: 'var(--radius-lg)', padding: '24px', boxShadow: '0 4px 20px rgba(0, 0, 0, 0.4)' }}>
        <div style={{ marginBottom: '16px' }}>
          <h1 style={{ fontSize: '1.5rem', fontWeight: 700, color: '#fff', margin: '0 0 6px 0', letterSpacing: '-0.02em' }}>
            Dark Web &amp; Sızıntı Radarı
          </h1>
          <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', margin: 0 }}>
            Telegram @breachdetect kanalından anlık toplanan kurumsal veri sızıntıları ve fidye yazılımı vakaları.
          </p>
        </div>

        {/* Kategori Butonları */}
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '16px', paddingBottom: '14px', borderBottom: '1px solid rgba(56, 189, 248, 0.1)' }}>
          {CATEGORIES.map((cat) => {
            const active = selectedCategory === cat.id;
            return (
              <button
                key={cat.id}
                type="button"
                onClick={() => setSelectedCategory(cat.id)}
                style={{
                  fontSize: '0.8rem',
                  padding: '6px 12px',
                  borderRadius: 'var(--radius-md)',
                  cursor: 'pointer',
                  transition: 'all 0.2s ease',
                  border: active ? '1px solid #38bdf8' : '1px solid var(--bg-card-border)',
                  background: active ? 'rgba(56, 189, 248, 0.15)' : '#070b14',
                  color: active ? '#38bdf8' : 'var(--text-secondary)',
                  fontWeight: active ? '600' : '500',
                  outline: 'none',
                }}
              >
                {cat.label}
              </button>
            );
          })}
        </div>

        {/* Arama ve Sıralama Çubuğu */}
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ position: 'relative', flex: 1, minWidth: '280px' }}>
            <input
              type="text"
              className="ioc-search-input"
              placeholder="Hedef kurum, domain, dark web forumu veya aktör ara..."
              value={search}
              onInput={(e: any) => handleSearchChange(e.target.value)}
              style={{ width: '100%', padding: '11px 16px', background: '#070b14', border: '1px solid rgba(56, 189, 248, 0.2)', borderRadius: 'var(--radius-md)', color: '#fff', fontSize: '0.9rem', outline: 'none' }}
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
            <div style={{ background: 'rgba(56, 189, 248, 0.08)', padding: '8px 14px', borderRadius: 'var(--radius-md)', border: '1px solid rgba(56, 189, 248, 0.15)' }}>
              <span style={{ fontSize: '0.825rem', color: 'var(--text-muted)' }}>Toplam Kayıt: </span>
              <strong style={{ color: '#38bdf8', fontSize: '0.9rem', marginLeft: '4px' }}>{filteredBreaches.length}</strong>
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
              Dark Web ve sızıntı kayıtları güvenli belleğe yükleniyor...
            </div>
          ) : (
            <table className="ioc-table" style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', tableLayout: 'fixed' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--bg-card-border)', background: 'rgba(14, 20, 34, 0.6)' }}>
                  <th style={{ width: '170px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Kategori</th>
                  <th style={{ padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Hedef Kurum &amp; Dark Web Özeti</th>
                  <th style={{ width: '190px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Kaynak &amp; Forum</th>
                  <th style={{ width: '120px', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Yayınlanma</th>
                  <th style={{ width: '100px', textAlign: 'center', padding: '16px 20px', fontSize: '0.8rem', textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>Bağlantı</th>
                </tr>
              </thead>
              <tbody>
                {filteredBreaches.map((art) => {
                  const timeAgo = formatTimeAgo(art.published_at);
                  const catInfo = getCategoryInfo(art);
                  const { forum, actor } = getForumAndActor(art);

                  return (
                    <tr 
                      key={art.id || art.link}
                      style={{ borderBottom: '1px solid var(--bg-card-border)', transition: 'background-color 0.15s ease' }}
                      onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(56, 189, 248, 0.03)')}
                      onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                    >
                      <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                        <span
                          style={{
                            display: 'inline-block',
                            fontSize: '0.75rem',
                            fontWeight: '600',
                            padding: '5px 10px',
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
                      <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                        <div style={{ fontWeight: 600, color: '#fff', marginBottom: '6px', fontSize: '0.9rem', lineHeight: '1.35' }}>
                          {art.title}
                        </div>
                        <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.4', wordBreak: 'break-word', display: '-webkit-box', WebkitLineClamp: '2', WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                          {art.summary || ''}
                        </div>
                      </td>
                      <td style={{ padding: '16px 20px', verticalAlign: 'middle' }}>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', alignItems: 'flex-start' }}>
                          <span className="ioc-source-tag" style={{ fontSize: '0.78rem', padding: '4px 8px', background: 'rgba(255, 255, 255, 0.05)', borderRadius: '4px' }}>{art.source}</span>
                          {forum && (
                            <span
                              style={{ fontSize: '0.72rem', color: '#cbd5e1', background: 'rgba(255,255,255,0.08)', padding: '2px 8px', borderRadius: '4px', border: '1px solid rgba(255,255,255,0.12)' }}
                              title="Dark Web Forum Kaynağı"
                            >
                              🌐 {forum}
                            </span>
                          )}
                          {actor && (
                            <span
                              style={{ fontSize: '0.7rem', color: '#fdba74', background: 'rgba(251,146,60,0.12)', padding: '2px 8px', borderRadius: '4px', border: '1px solid rgba(251,146,60,0.25)' }}
                              title="Tehdit Aktörü / Yazar"
                            >
                              👤 {actor}
                            </span>
                          )}
                        </div>
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
            <div className="ioc-empty-state" style={{ textAlign: 'center', padding: '60px', color: 'var(--text-muted)' }}>
              <p>Seçilen kategori veya arama kriterine uygun dark web / sızıntı kaydı bulunamadı.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}