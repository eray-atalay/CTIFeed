import { useState, useEffect, useMemo, useRef } from 'preact/hooks';
import { fetchArticles, fetchSources } from '../../services/api';
import {
  openArticleModal,
  refreshTrigger,
  activeTagFilter,
  formatTimeAgo,
  getScoreCategory,
} from '../../services/store';
import type { Article, SourceInfo } from '../../types/cti';

const TAG_PILLS = [
  { label: 'Tüm Beslemeler', tag: '' },
  { label: 'TR-Focus', tag: 'TR-Focus', className: 'pill-tr' },
  { label: 'Zero-Day', tag: 'zero-day', className: 'pill-alert' },
  { label: 'In-The-Wild', tag: 'in-the-wild' },
  { label: 'Ransomware', tag: 'ransomware', className: 'pill-ransom' },
  { label: 'RCE', tag: 'rce', className: 'pill-rce' },
  { label: 'Auth Bypass', tag: 'auth-bypass' },
  { label: 'Infostealer', tag: 'infostealer' },
  { label: 'WordPress', tag: 'wordpress' },
  { label: 'Fortinet', tag: 'fortinet' },
  { label: 'Cisco', tag: 'cisco' },
  { label: 'Ivanti', tag: 'ivanti' },
  { label: 'Palo Alto', tag: 'palo-alto' },
];

export default function ThreatFeed() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [sources, setSources] = useState<SourceInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [lastUpdated, setLastUpdated] = useState<string>('');

  // Filtreler
  const [search, setSearch] = useState<string>('');
  const [debouncedSearch, setDebouncedSearch] = useState<string>('');
  const [selectedTag, setSelectedTag] = useState<string>('');
  const [selectedSource, setSelectedSource] = useState<string>('');
  const [minScore, setMinScore] = useState<number>(0);
  const [sortBy, setSortBy] = useState<string>('score');
  const [timeRange, setTimeRange] = useState<string>('');

  const searchTimerRef = useRef<any>(null);

  // Arama gecikmesi (Debounce)
  const handleSearchChange = (val: string) => {
    setSearch(val);
    clearTimeout(searchTimerRef.current);
    searchTimerRef.current = setTimeout(() => {
      setDebouncedSearch(val.trim());
    }, 300);
  };

  const handleClearSearch = () => {
    setSearch('');
    setDebouncedSearch('');
  };

  // Harici etiket seçimini dinleme
  useEffect(() => {
    return activeTagFilter.subscribe((tag) => {
      if (tag !== undefined && tag !== selectedTag) {
        setSelectedTag(tag);
      }
    });
  }, []);

  // Kaynakları yükleme
  useEffect(() => {
    fetchSources()
      .then(res => setSources(res.sources || []))
      .catch(err => console.error('Kaynaklar yüklenemedi:', err));
  }, []);

  // Haberleri çekme fonksiyonu
  const loadArticles = async () => {
    setLoading(true);
    try {
      const data = await fetchArticles({
        search: debouncedSearch,
        tag: selectedTag,
        source: selectedSource,
        min_score: minScore,
        sort: sortBy,
        time_range: timeRange,
        limit: 150,
      });

      // Telegram kaynaklı maddeleri ana dashboard'dan filtreliyoruz (CVE modalında listelenir)
      const feedItems = (data.articles || []).filter(a => !a.source.startsWith('Telegram:'));
      setArticles(feedItems);
      setLastUpdated(new Date().toLocaleTimeString('tr-TR'));
    } catch (err) {
      console.error('Haberler alınırken hata:', err);
      setArticles([]);
    } finally {
      setLoading(false);
    }
  };

  // Filtreler değiştikçe veya yenileme tetiklendikçe haberleri çek
  useEffect(() => {
    loadArticles();
  }, [debouncedSearch, selectedTag, selectedSource, minScore, sortBy, timeRange]);

  useEffect(() => {
    return refreshTrigger.subscribe(() => {
      loadArticles();
    });
  }, [debouncedSearch, selectedTag, selectedSource, minScore, sortBy, timeRange]);

  // Filtreleri sıfırlama
  const handleResetFilters = () => {
    setSearch('');
    setDebouncedSearch('');
    setSelectedTag('');
    setSelectedSource('');
    setMinScore(0);
    setSortBy('score');
    setTimeRange('');
  };

  // Aktif filtre özeti
  const activeFiltersSummary = useMemo(() => {
    const list: string[] = [];
    if (debouncedSearch) list.push(`Arama: "${debouncedSearch}"`);
    if (selectedTag) list.push(`Etiket: [${selectedTag}]`);
    if (selectedSource) list.push(`Kaynak: ${selectedSource}`);
    if (minScore > 0) list.push(`Min Skor: ${minScore}+`);
    if (timeRange) {
      const timeNames: Record<string, string> = { today: 'Bugün', '1w': 'Son 1 Hafta', '2w': 'Son 2 Hafta', '1m': 'Son 1 Ay' };
      list.push(`Zaman: ${timeNames[timeRange] || timeRange}`);
    }
    return list;
  }, [debouncedSearch, selectedTag, selectedSource, minScore, timeRange]);

  return (
    <>
      {/* Araç Çubuğu (Toolbar) */}
      <section class="toolbar-section" aria-label="Arama ve Filtreleme Kontrolleri">
        <div class="search-box">
          <svg class="search-icon" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
          <input
            type="text"
            id="search-input"
            class="search-input"
            placeholder="Tehdit ara... (CVE kodu, zararlı yazılım, zafiyet, APT grubu, kurban kuruluş)"
            value={search}
            onInput={(e: any) => handleSearchChange(e.target.value)}
          />
          {search && (
            <button
              id="search-clear-btn"
              class="search-clear-btn"
              title="Aramayı Temizle"
              onClick={handleClearSearch}
            >
              &times;
            </button>
          )}
        </div>

        {/* Hızlı Kategori ve Tehdit Hapları */}
        <div class="tag-pills" id="tag-pills" role="tablist">
          {TAG_PILLS.map((pill) => (
            <button
              key={pill.tag}
              class={`pill ${pill.className || ''} ${selectedTag === pill.tag ? 'active' : ''}`}
              data-tag={pill.tag}
              onClick={() => setSelectedTag(pill.tag)}
            >
              {pill.label}
            </button>
          ))}
        </div>

        {/* Açılır Kutu Filtreleri */}
        <div class="filter-dropdowns">
          {/* Zaman Filtresi */}
          <div class="select-wrapper">
            <select
              id="time-select"
              aria-label="Zaman Aralığına Göre Filtrele"
              value={timeRange}
              onChange={(e: any) => setTimeRange(e.target.value)}
            >
              <option value="">Tüm Zamanlar</option>
              <option value="today">Bugün (Son 24 Saat)</option>
              <option value="1w">Son 1 Hafta</option>
              <option value="2w">Son 2 Hafta</option>
              <option value="1m">Son 1 Ay</option>
            </select>
          </div>

          {/* Kaynak Seçimi */}
          <div class="select-wrapper">
            <select
              id="source-select"
              aria-label="Kaynağa Göre Filtrele"
              value={selectedSource}
              onChange={(e: any) => setSelectedSource(e.target.value)}
            >
              <option value="">Tüm Kaynaklar</option>
              {sources.map((s) => (
                <option key={s.name} value={s.name}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>

          {/* Skor Filtresi */}
          <div class="select-wrapper">
            <select
              id="score-select"
              aria-label="Tehdit Skoruna Göre Filtrele"
              value={minScore}
              onChange={(e: any) => setMinScore(Number(e.target.value))}
            >
              <option value="0">Tüm Skorlar</option>
              <option value="30">30+ (Orta ve Üzeri)</option>
              <option value="50">50+ (Yüksek ve Kritik)</option>
              <option value="80">80+ (Sadece Kritik)</option>
            </select>
          </div>

          {/* Sıralama */}
          <div class="select-wrapper">
            <select
              id="sort-select"
              aria-label="Sıralama Ölçütü"
              value={sortBy}
              onChange={(e: any) => setSortBy(e.target.value)}
            >
              <option value="score">En Yüksek Tehdit Skoru</option>
              <option value="date">En Yeni Tarih</option>
            </select>
          </div>
        </div>

        {/* Aktif Filtre Bilgilendirme Çubuğu */}
        {activeFiltersSummary.length > 0 && (
          <div class="active-filters-bar" id="active-filter-bar">
            <span>Aktif Filtre: </span>
            <strong id="filter-summary-text">{activeFiltersSummary.join(' - ')}</strong>
            <button class="btn-clear-filters" id="btn-reset-filters" onClick={handleResetFilters}>
              Filtreleri Temizle &times;
            </button>
          </div>
        )}
      </section>

      {/* Tehdit Akışı Izgarası */}
      <section class="feed-section">
        <div class="feed-header">
          <h2 class="feed-title">
            Tehdit İstihbaratı Akışı
            <span class="feed-count-badge" id="feed-count">
              {articles.length} öge
            </span>
          </h2>
          <div class="feed-meta" id="last-updated-text">
            {lastUpdated ? `Son güncelleme: ${lastUpdated}` : 'Güncelleniyor...'}
          </div>
        </div>

        {/* Yükleme İskeleti (Skeleton) */}
        {loading && (
          <div class="loading-grid">
            <div class="card-skeleton"></div>
            <div class="card-skeleton"></div>
            <div class="card-skeleton"></div>
            <div class="card-skeleton"></div>
          </div>
        )}

        {/* Kartlar Izgarası */}
        {!loading && articles.length > 0 && (
          <div class="articles-grid" id="articles-grid">
            {articles.map((article) => {
              const isCritical = article.score >= 50;
              const badge = getScoreCategory(article.score);
              const timeAgo = formatTimeAgo(article.published_at);

              return (
                <article
                  key={article.id}
                  class={`article-card ${isCritical ? 'is-critical' : ''}`}
                  onClick={() => openArticleModal(article)}
                >
                  <div>
                    <div class="card-top-row">
                      <span class="source-badge">
                        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                          <circle cx="12" cy="12" r="10"></circle>
                          <line x1="2" y1="12" x2="22" y2="12"></line>
                        </svg>
                        {article.source}
                      </span>
                      <div style="display: flex; align-items: center; gap: 8px;">
                        <span class="card-time">{timeAgo}</span>
                        <span class={`score-badge ${badge.className}`}>{badge.label}</span>
                      </div>
                    </div>

                    <h3 class="card-title">{article.title}</h3>
                    <p class="card-summary">
                      {article.summary || 'Bu kayıt için özet metin bulunmuyor.'}
                    </p>
                  </div>

                  <div>
                    {article.tags && article.tags.length > 0 && (
                      <div class="card-tags">
                        {article.tags.map((t) => {
                          const isCVE = t.toUpperCase().startsWith('CVE-');
                          const isTR = t === 'TR-Focus';
                          let tagClass = 'tag-item';
                          if (isCVE) tagClass += ' tag-cve';
                          if (isTR) tagClass += ' tag-tr';

                          return (
                            <button
                              key={t}
                              class={tagClass}
                              onClick={(e) => {
                                e.stopPropagation();
                                setSelectedTag(t);
                              }}
                            >
                              {t}
                            </button>
                          );
                        })}
                      </div>
                    )}

                    <div class="card-footer">
                      <span class="card-action-inspect">
                        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                          <circle cx="12" cy="12" r="10"></circle>
                          <line x1="12" y1="16" x2="12" y2="12"></line>
                          <line x1="12" y1="8" x2="12.01" y2="8"></line>
                        </svg>
                        Detayları İncele
                      </span>
                      <a
                        href={article.link}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="card-action-link"
                        title="Kaynak Haberi Görüntüle"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <span>Kaynak</span>
                        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path>
                          <polyline points="15 3 21 3 21 9"></polyline>
                          <line x1="10" y1="14" x2="21" y2="3"></line>
                        </svg>
                      </a>
                    </div>
                  </div>
                </article>
              );
            })}
          </div>
        )}

        {/* Boş Durum (Empty State) */}
        {!loading && articles.length === 0 && (
          <div class="empty-state">
            <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="currentColor" stroke-width="1.5">
              <circle cx="11" cy="11" r="8"></circle>
              <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
              <line x1="8" y1="11" x2="14" y2="11"></line>
            </svg>
            <h3>Eşleşen tehdit istihbaratı bulunamadı</h3>
            <p>Arama sorgunuzu değiştirmeyi veya aktif filtreleri kaldırmayı deneyebilirsiniz.</p>
            <button class="btn-secondary" onClick={handleResetFilters}>
              Filtreleri Temizle
            </button>
          </div>
        )}
      </section>
    </>
  );
}
