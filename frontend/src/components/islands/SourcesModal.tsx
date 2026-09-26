import { useState, useEffect } from 'preact/hooks';
import { activeModal, closeModal } from '../../services/store';
import { fetchSources, toggleSource, addSource, deleteSource } from '../../services/api';
import type { SourceInfo } from '../../types/cti';

export default function SourcesModal() {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [sources, setSources] = useState<SourceInfo[]>([]);
  const [search, setSearch] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [togglingId, setTogglingId] = useState<number | null>(null);

  const [showAddForm, setShowAddForm] = useState<boolean>(false);
  const [addType, setAddType] = useState<'twitter' | 'rss'>('twitter');
  const [formInput, setFormInput] = useState<string>('');
  const [formName, setFormName] = useState<string>('');
  const [formCategory, setFormCategory] = useState<string>('Twitter Threat Intel');
  const [formSubmitting, setFormSubmitting] = useState<boolean>(false);
  const [formMsg, setFormMsg] = useState<{ type: 'error' | 'success'; text: string } | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  const loadSources = () => {
    setLoading(true);
    fetchSources()
      .then((res) => setSources(res.sources || []))
      .catch((err) => console.error('Kaynaklar yuklenirken hata:', err))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    const unsub = activeModal.subscribe((val) => {
      const open = val === 'sources';
      setIsOpen(open);
      if (open) {
        loadSources();
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

  const handleToggle = async (src: SourceInfo) => {
    if (!src.id || togglingId !== null) return;
    const previousState = src.is_active !== false;
    const nextState = !previousState;

    setSources((prev) =>
      prev.map((item) => (item.id === src.id ? { ...item, is_active: nextState } : item))
    );
    setTogglingId(src.id);

    try {
      const res = await toggleSource(src.id);
      setSources((prev) =>
        prev.map((item) => (item.id === src.id ? { ...item, is_active: res.is_active } : item))
      );
    } catch (err) {
      console.error('Kaynak durumu degistirilemedi:', err);
      setSources((prev) =>
        prev.map((item) => (item.id === src.id ? { ...item, is_active: previousState } : item))
      );
    } finally {
      setTogglingId(null);
    }
  };

  const handleTypeChange = (type: 'twitter' | 'rss') => {
    setAddType(type);
    if (type === 'twitter') {
      setFormCategory('Twitter Threat Intel');
    } else {
      setFormCategory('General Security');
    }
  };

  const handleAddSubmit = async (e: Event) => {
    e.preventDefault();
    if (!formInput.trim()) {
      setFormMsg({ type: 'error', text: 'Lütfen kullanıcı adı veya besleme URL adresi girin.' });
      return;
    }
    setFormSubmitting(true);
    setFormMsg(null);
    try {
      const res = await addSource({
        name: formName.trim() || undefined,
        url: formInput.trim(),
        category: formCategory.trim() || undefined,
      });
      if (res.source) {
        setSources((prev) => [res.source, ...prev]);
        setFormMsg({ type: 'success', text: 'Yeni kaynak başarıyla eklendi!' });
        setFormInput('');
        setFormName('');
        setTimeout(() => {
          setShowAddForm(false);
          setFormMsg(null);
        }, 1200);
      }
    } catch (err: any) {
      setFormMsg({ type: 'error', text: err.message || 'Kaynak eklenemedi.' });
    } finally {
      setFormSubmitting(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Bu kaynağı silmek istediğinize emin misiniz?')) return;
    setDeletingId(id);
    try {
      await deleteSource(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
    } catch (err: any) {
      alert(err.message || 'Kaynak silinemedi');
    } finally {
      setDeletingId(null);
    }
  };

  const getLinkUrl = (rawUrl: string) => {
    if (rawUrl.includes('/api/rss/twitter/')) {
      const idx = rawUrl.indexOf('/api/rss/twitter/');
      return rawUrl.substring(idx);
    }
    return rawUrl;
  };

  if (!isOpen) return null;

  const filteredSources = sources.filter((s) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return s.name.toLowerCase().includes(q) || (s.category && s.category.toLowerCase().includes(q));
  });

  const activeCount = sources.filter((s) => s.is_active !== false).length;

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
          <div style="display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap;">
            <h2 style="margin: 0;">İzlenen CTI Besleme Kaynakları</h2>
            <span style="font-size: 0.85rem; color: var(--accent-primary, #38bdf8); font-weight: 600;">
              {activeCount} / {sources.length} Aktif
            </span>
          </div>
          <p class="modal-sub" style="margin-top: 6px;">
            Siber tehditler, kritik CVE zafiyetleri ve TR-Focus istihbaratı için taranan kaynaklar ve sağlık durumları.
          </p>

          <div style="margin-top: 14px; display: flex; gap: 10px; flex-wrap: wrap; align-items: center; justify-content: space-between;">
            <input
              type="text"
              class="search-input"
              placeholder="Kaynak veya kategori ara..."
              value={search}
              onInput={(e) => setSearch((e.target as HTMLInputElement).value)}
              style="width: 100%; max-width: 320px; font-size: 0.85rem; padding: 6px 12px; background: rgba(14,20,34,0.6); border: 1px solid var(--bg-card-border); border-radius: 6px; color: var(--text-base);"
            />
            <button
              type="button"
              class="btn-primary"
              onClick={() => {
                setShowAddForm(!showAddForm);
                setFormMsg(null);
              }}
              style="padding: 6px 14px; font-size: 0.82rem; font-weight: 600; display: inline-flex; align-items: center; gap: 6px; cursor: pointer;"
            >
              {showAddForm ? '✕ Formu Kapat' : '＋ Yeni X / RSS Kaynağı Ekle'}
            </button>
          </div>

          {showAddForm && (
            <div style="margin-top: 14px; padding: 14px; background: rgba(15, 23, 42, 0.7); border: 1px solid rgba(56, 189, 248, 0.3); border-radius: 8px;">
              <div style="display: flex; gap: 12px; margin-bottom: 12px; align-items: center;">
                <span style="font-size: 0.82rem; font-weight: 600; color: var(--text-muted);">Kaynak Tipi:</span>
                <label style="display: inline-flex; align-items: center; gap: 4px; font-size: 0.82rem; cursor: pointer; color: var(--text-base);">
                  <input
                    type="radio"
                    name="addType"
                    checked={addType === 'twitter'}
                    onChange={() => handleTypeChange('twitter')}
                  />
                  X / Twitter Hesabı
                </label>
                <label style="display: inline-flex; align-items: center; gap: 4px; font-size: 0.82rem; cursor: pointer; color: var(--text-base);">
                  <input
                    type="radio"
                    name="addType"
                    checked={addType === 'rss'}
                    onChange={() => handleTypeChange('rss')}
                  />
                  Standart RSS Beslemesi
                </label>
              </div>

              <form onSubmit={handleAddSubmit} style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 10px;">
                <div>
                  <label style="display: block; font-size: 0.75rem; color: var(--text-muted); margin-bottom: 4px;">
                    {addType === 'twitter' ? 'X Kullanıcı Adı veya Link' : 'RSS Besleme URL'} *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder={addType === 'twitter' ? '@vxunderground veya vxunderground' : 'https://example.com/feed.xml'}
                    value={formInput}
                    onInput={(e) => setFormInput((e.target as HTMLInputElement).value)}
                    style="width: 100%; font-size: 0.82rem; padding: 6px 10px; background: rgba(10, 15, 29, 0.8); border: 1px solid var(--bg-card-border); border-radius: 6px; color: var(--text-base);"
                  />
                </div>

                <div>
                  <label style="display: block; font-size: 0.75rem; color: var(--text-muted); margin-bottom: 4px;">
                    Kaynak Görünen Adı (İsteğe Bağlı)
                  </label>
                  <input
                    type="text"
                    placeholder={addType === 'twitter' ? 'X: vx-underground' : 'Güvenlik Bülteni'}
                    value={formName}
                    onInput={(e) => setFormName((e.target as HTMLInputElement).value)}
                    style="width: 100%; font-size: 0.82rem; padding: 6px 10px; background: rgba(10, 15, 29, 0.8); border: 1px solid var(--bg-card-border); border-radius: 6px; color: var(--text-base);"
                  />
                </div>

                <div>
                  <label style="display: block; font-size: 0.75rem; color: var(--text-muted); margin-bottom: 4px;">
                    Kategori
                  </label>
                  <input
                    type="text"
                    placeholder="Twitter Threat Intel / Zafiyet"
                    value={formCategory}
                    onInput={(e) => setFormCategory((e.target as HTMLInputElement).value)}
                    style="width: 100%; font-size: 0.82rem; padding: 6px 10px; background: rgba(10, 15, 29, 0.8); border: 1px solid var(--bg-card-border); border-radius: 6px; color: var(--text-base);"
                  />
                </div>

                <div style="display: flex; align-items: flex-end; gap: 8px;">
                  <button
                    type="submit"
                    disabled={formSubmitting}
                    class="btn-primary"
                    style="padding: 7px 16px; font-size: 0.82rem; font-weight: 600; cursor: pointer; height: 33px;"
                  >
                    {formSubmitting ? 'Ekleniyor...' : 'Kaydet ve Ekle'}
                  </button>
                  <button
                    type="button"
                    onClick={() => setShowAddForm(false)}
                    class="btn-secondary"
                    style="padding: 7px 12px; font-size: 0.82rem; cursor: pointer; height: 33px;"
                  >
                    İptal
                  </button>
                </div>
              </form>

              {formMsg && (
                <div
                  style={`margin-top: 10px; font-size: 0.8rem; font-weight: 600; color: ${
                    formMsg.type === 'error' ? '#f87171' : '#4ade80'
                  };`}
                >
                  {formMsg.text}
                </div>
              )}
            </div>
          )}
        </div>

        {loading ? (
          <div style="text-align: center; padding: 40px; color: var(--text-muted);">
            Kaynaklar yükleniyor...
          </div>
        ) : (
          <div class="sources-list">
            {filteredSources.map((src) => {
              const isActive = src.is_active !== false;
              const status = src.last_status || 'pending';
              const latency = src.response_time_ms || 0;
              let latencyClass = 'fast';
              if (latency >= 3000) latencyClass = 'slow';
              else if (latency >= 1000) latencyClass = 'medium';

              return (
                <div
                  key={src.id || src.url}
                  class={`source-item-card ${!isActive ? 'is-inactive' : ''}`}
                >
                  <div class="source-info">
                    <div class="source-header-row">
                      <span
                        class={`source-health-dot ${status}`}
                        title={
                          status === 'ok'
                            ? 'Kaynak Sağlıklı'
                            : status === 'error'
                            ? `Hata: ${src.last_error || 'Bağlantı kurulamadı'}`
                            : 'Henüz taranmadı'
                        }
                      />
                      <h5 title={src.name}>{src.name}</h5>
                      {latency > 0 && isActive && (
                        <span
                          class={`source-latency-badge ${latencyClass}`}
                          title={`Son Yanıt Süresi: ${latency}ms`}
                        >
                          {latency}ms
                        </span>
                      )}
                    </div>
                    <div class="source-meta-row">
                      <span class="source-cat">{src.category || 'Genel CTI'}</span>
                      {src.last_error && status === 'error' && (
                        <span
                          style="font-size: 0.7rem; color: #f87171; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;"
                          title={src.last_error}
                        >
                          {src.last_error}
                        </span>
                      )}
                    </div>
                  </div>

                  <div class="source-actions-group">
                    {src.id && (
                      <label
                        class="source-toggle-switch"
                        title={isActive ? 'Beslemeyi Devre Dışı Bırak' : 'Beslemeyi Etkinleştir'}
                      >
                        <input
                          type="checkbox"
                          checked={isActive}
                          disabled={togglingId === src.id}
                          onChange={() => handleToggle(src)}
                        />
                        <span class="source-toggle-slider" />
                      </label>
                    )}
                    <a
                      href={getLinkUrl(src.url)}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="btn-link"
                      title="Besleme Bağlantısı"
                      style="font-size: 0.78rem; text-decoration: none; padding: 4px 8px;"
                    >
                      XML &rarr;
                    </a>
                    {src.id && (
                      <button
                        type="button"
                        class="btn-icon"
                        disabled={deletingId === src.id}
                        onClick={() => handleDelete(src.id!)}
                        title="Kaynağı Sil"
                        style="background: transparent; border: none; color: #94a3b8; cursor: pointer; padding: 4px 6px; font-size: 0.9rem;"
                      >
                        🗑
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
