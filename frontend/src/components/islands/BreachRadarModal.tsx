import { useState, useEffect, useMemo, useRef } from 'preact/hooks';
import { activeModal, closeModal, formatTimeAgo } from '../../services/store';
import { fetchArticles } from '../../services/api';
import type { Article } from '../../types/cti';

export default function BreachRadarModal() {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [allArticles, setAllArticles] = useState<Article[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [search, setSearch] = useState<string>('');
  const [debouncedSearch, setDebouncedSearch] = useState<string>('');
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

  const filteredBreaches = useMemo(() => {
    // Sadece Telegram kanalından gelen haberler filtrelenir (Bloglar ve genel RSS hariç tutulur)
    let items = allArticles.filter((a) => {
      const src = (a.source || '').toLowerCase();
      const link = (a.link || '').toLowerCase();
      const isTelegram = src.startsWith('telegram:') || link.includes('t.me/');
      const isBreach = src.includes('breachdetect') || link.includes('breachdetect');
      return isTelegram && isBreach;
    });

    if (debouncedSearch) {
      items = items.filter(
        (a) =>
          (a.title && a.title.toLowerCase().includes(debouncedSearch)) ||
          (a.summary && a.summary.toLowerCase().includes(debouncedSearch)) ||
          (a.source && a.source.toLowerCase().includes(debouncedSearch)) ||
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
      <div class="modal-card modal-card-wide" style="max-width: 1050px; height: 85vh;">
        <button class="modal-close" onClick={closeModal} aria-label="Kapat">&times;</button>
        <div class="modal-header">
          <div class="ioc-modal-header-top">
            <div>
              <h2>
                Veri Sızıntısı Radarı (BreachDetect)
                <span
                  class="feed-count-badge"
                  style="background: rgba(239, 68, 68, 0.2); color: #fca5a5; font-size: 0.8rem; padding: 2px 8px; border-radius: 6px; margin-left: 8px;"
                >
                  {filteredBreaches.length} sızıntı
                </span>
              </h2>
              <p class="modal-sub">
                Telegram @breachdetect kanalından anlık çekilen kurumsal veri sızıntıları ve siber vakalar.
              </p>
            </div>
          </div>
          <div class="ioc-filters-bar" style="display: flex; gap: 12px; align-items: center;">
            <input
              type="text"
              class="ioc-search-input"
              placeholder="Kurum, ülke veya sızıntı ara..."
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

        <div class="ioc-table-container" style="max-height: calc(85vh - 170px);">
          {loading ? (
            <div style="text-align: center; padding: 40px; color: var(--text-muted);">
              Sızıntı kayıtları yükleniyor...
            </div>
          ) : (
            <table class="ioc-table">
              <thead>
                <tr>
                  <th style="width: 140px;">Kategori</th>
                  <th>Sızıntı & Hedef Kurum Özeti</th>
                  <th style="width: 170px;">Kaynak</th>
                  <th style="width: 120px;">Yayınlanma</th>
                  <th style="width: 80px; text-align: center;">Bağlantı</th>
                </tr>
              </thead>
              <tbody>
                {filteredBreaches.map((art) => {
                  const timeAgo = formatTimeAgo(art.published_at);

                  return (
                    <tr key={art.id || art.link}>
                      <td>
                        <span class="tag-item tag-tr" style="font-size: 0.75rem;">
                          VERİ SIZINTISI
                        </span>
                      </td>
                      <td>
                        <div style="font-weight: 600; color: #fff; margin-bottom: 4px;">
                          {art.title}
                        </div>
                        <div style="font-size: 0.78rem; color: var(--text-secondary); line-height: 1.4;">
                          {art.summary || ''}
                        </div>
                      </td>
                      <td>
                        <span class="ioc-source-tag">{art.source}</span>
                      </td>
                      <td style="color: var(--text-muted); font-size: 0.8rem;">
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
              <p>Arama kriterine uygun sızıntı kaydı bulunamadı.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
