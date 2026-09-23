import { useState, useEffect, useMemo, useRef } from 'preact/hooks';
import { activeModal, closeModal, formatTimeAgo } from '../../services/store';
import { fetchArticles } from '../../services/api';
import type { Article } from '../../types/cti';

interface CategoryPill {
  id: string;
  label: string;
  count?: number;
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
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [allArticles, setAllArticles] = useState<Article[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [search, setSearch] = useState<string>('');
  const [debouncedSearch, setDebouncedSearch] = useState<string>('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'desc' | 'asc'>('desc');

  const searchTimerRef = useRef<any>(null);

  useEffect(() => {
    const unsub = activeModal.subscribe((val) => {
      const open = val === 'breach';
      setIsOpen(open);
      if (open) {
        loadBreachData();
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

  // Helper to determine the dynamic badge and category for an article
  const getCategoryInfo = (art: Article) => {
    const tags = art.tags || [];
    const text = (art.title + ' ' + (art.summary || '') + ' ' + tags.join(' ')).toLowerCase();

    if (tags.includes('ransomware') || text.includes('ransomware') || text.includes('lockbit') || text.includes('published a new victim')) {
      return {
        category: 'ransomware',
        label: 'FİDYE YAZILIMI',
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
        label: 'ERİŞİM SATIŞI',
        color: '#c084fc',
        bg: 'rgba(192, 132, 252, 0.2)',
        border: 'rgba(192, 132, 252, 0.35)',
      };
    }
    if (tags.includes('database-dump') || text.includes('sql dump') || text.includes('database dump') || text.includes('db dump')) {
      return {
        category: 'database-dump',
        label: 'VERİTABANI DÖKÜMÜ',
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
      label: 'VERİ SIZINTISI',
      color: '#fca5a5',
      bg: 'rgba(244, 63, 94, 0.2)',
      border: 'rgba(244, 63, 94, 0.35)',
    };
  };

  // Helper to extract darkweb forum or threat actor from tags
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
    // Sadece Telegram breachdetect kanalından gelen haberler filtrelenir
    let items = allArticles.filter((a) => {
      const src = (a.source || '').toLowerCase();
      const link = (a.link || '').toLowerCase();
      const isTelegram = src.startsWith('telegram:') || link.includes('t.me/');
      const isBreach = src.includes('breachdetect') || link.includes('breachdetect');
      return isTelegram && isBreach;
    });

    // Kategori Filtresi
    if (selectedCategory) {
      items = items.filter((a) => {
        const info = getCategoryInfo(a);
        return info.category === selectedCategory;
      });
    }

    // Arama Filtresi (Başlık, Özet, Kaynak, Etiketler, Aktör, Forum)
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
      <div class="modal-card modal-card-wide" style="max-width: 1100px; height: 88vh;">
        <button class="modal-close" onClick={closeModal} aria-label="Kapat">&times;</button>
        <div class="modal-header" style="padding-bottom: 12px;">
          <div class="ioc-modal-header-top" style="margin-bottom: 10px;">
            <div>
              <h2>
                Dark Web &amp; Sızıntı Radarı (BreachDetect)
                <span
                  class="feed-count-badge"
                  style="background: rgba(239, 68, 68, 0.2); color: #fca5a5; font-size: 0.8rem; padding: 2px 8px; border-radius: 6px; margin-left: 8px;"
                >
                  {filteredBreaches.length} istihbarat maddesi
                </span>
              </h2>
              <p class="modal-sub">
                Telegram @breachdetect kanalından anlık toplanan sızıntılar, combolistler, fidye yazılımı kurbanları ve dark web verileri.
              </p>
            </div>
          </div>

          {/* Kategori Filtre Butonları (Pills) */}
          <div
            style="display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 12px; padding: 4px 0;"
          >
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

          <div class="ioc-filters-bar" style="display: flex; gap: 12px; align-items: center; margin-top: 0;">
            <input
              type="text"
              class="ioc-search-input"
              placeholder="Hedef kurum, domain, dark web forumu veya aktör ara..."
              value={search}
              onInput={(e: any) => handleSearchChange(e.target.value)}
              style="flex: 1;"
            />
            <div class="select-wrapper">
              <select
                value={sortOrder}
                onChange={(e: any) => setSortOrder(e.target.value)}
                style="background: #0e1422; border: 1px solid var(--bg-card-border); color: var(--text-secondary); padding: 7px 12px; border-radius: var(--radius-md); font-size: 0.825rem; cursor: pointer;"
              >
                <option value="desc">En Yeni Tarih</option>
                <option value="asc">En Eski Tarih</option>
              </select>
            </div>
          </div>
        </div>

        <div class="ioc-table-container" style="max-height: calc(88vh - 210px);">
          {loading ? (
            <div style="text-align: center; padding: 40px; color: var(--text-muted);">
              Dark Web ve sızıntı kayıtları yükleniyor...
            </div>
          ) : (
            <table class="ioc-table">
              <thead>
                <tr>
                  <th style="width: 155px;">Kategori</th>
                  <th>Hedef Kurum &amp; Dark Web Özeti</th>
                  <th style="width: 200px;">Kaynak &amp; Forum</th>
                  <th style="width: 110px;">Yayınlanma</th>
                  <th style="width: 80px; text-align: center;">Bağlantı</th>
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
                        <div style="font-weight: 600; color: #fff; margin-bottom: 4px; font-size: 0.88rem;">
                          {art.title}
                        </div>
                        <div style="font-size: 0.78rem; color: var(--text-secondary); line-height: 1.4; word-break: break-word;">
                          {art.summary || ''}
                        </div>
                      </td>
                      <td>
                        <div style="display: flex; flex-direction: column; gap: 4px; align-items: flex-start;">
                          <span class="ioc-source-tag" style="font-size: 0.72rem;">{art.source}</span>
                          {forum && (
                            <span
                              style="font-size: 0.7rem; color: #cbd5e1; background: rgba(255,255,255,0.08); padding: 1px 6px; border-radius: 4px; border: 1px solid rgba(255,255,255,0.12);"
                              title="Dark Web Forum Kaynağı"
                            >
                              🌐 {forum}
                            </span>
                          )}
                          {actor && (
                            <span
                              style="font-size: 0.68rem; color: #fdba74; background: rgba(251,146,60,0.12); padding: 1px 6px; border-radius: 4px; border: 1px solid rgba(251,146,60,0.25);"
                              title="Tehdit Aktörü / Yazar"
                            >
                              👤 {actor}
                            </span>
                          )}
                        </div>
                      </td>
                      <td style="color: var(--text-muted); font-size: 0.8rem; white-space: nowrap;">
                        {timeAgo}
                      </td>
                      <td style="text-align: center;">
                        <a
                          href={art.link}
                          target="_blank"
                          rel="noopener noreferrer"
                          class="btn-link"
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
            <div class="ioc-empty-state">
              <p>Seçilen kategori veya arama kriterine uygun dark web / sızıntı kaydı bulunamadı.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
