import { useState, useEffect } from 'preact/hooks';
import { fetchSources, toggleSource, addSource, deleteSource, isAdminLoggedIn } from '../../services/api';
import type { SourceInfo } from '../../types/cti';

export default function SourcesModal() {
  const [sources, setSources] = useState<SourceInfo[]>([]);
  const [search, setSearch] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(true);
  const [togglingId, setTogglingId] = useState<number | null>(null);

  const [showAddForm, setShowAddForm] = useState<boolean>(false);
  const [addType, setAddType] = useState<'twitter' | 'rss'>('twitter');
  const [formInput, setFormInput] = useState<string>('');
  const [formName, setFormName] = useState<string>('');
  const [formCategory, setFormCategory] = useState<string>('Twitter Threat Intel');
  const [formSubmitting, setFormSubmitting] = useState<boolean>(false);
  const [formMsg, setFormMsg] = useState<{ type: 'error' | 'success'; text: string } | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [isAdmin, setIsAdmin] = useState<boolean>(false);

  useEffect(() => {
    loadSources();
    isAdminLoggedIn().then(setIsAdmin);
  }, []);

  const loadSources = () => {
    setLoading(true);
    fetchSources()
      .then((res) => setSources(res.sources || []))
      .catch((err) => console.error('Kaynaklar yüklenirken hata:', err))
      .finally(() => setLoading(false));
  };

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
      console.error('Kaynak durumu değiştirilemedi:', err);
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

  const filteredSources = sources.filter((s) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return s.name.toLowerCase().includes(q) || (s.category && s.category.toLowerCase().includes(q));
  });

  const activeCount = sources.filter((s) => s.is_active !== false).length;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', width: '100%', gap: '20px', paddingBottom: '40px' }}>
      {/* Üst Bilgi ve Filtreleme Kartı */}
      <div style={{ background: 'linear-gradient(135deg, rgba(14, 20, 34, 0.9) 0%, rgba(10, 15, 26, 0.95) 100%)', border: '1px solid rgba(56, 189, 248, 0.15)', borderRadius: 'var(--radius-lg)', padding: '24px', boxShadow: '0 4px 20px rgba(0, 0, 0, 0.4)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px', marginBottom: '18px' }}>
          <div>
            <h1 style={{ fontSize: '1.5rem', fontWeight: 700, color: '#fff', margin: '0 0 6px 0', letterSpacing: '-0.02em' }}>
              CTI Besleme Kaynakları
            </h1>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', margin: 0 }}>
              Siber tehditler ve kritik CVE zafiyetleri için anlık taranan kaynaklar ve sağlık durumları.
            </p>
          </div>
          <div style={{ background: 'rgba(56, 189, 248, 0.08)', padding: '8px 14px', borderRadius: 'var(--radius-md)', border: '1px solid rgba(56, 189, 248, 0.15)' }}>
            <span style={{ fontSize: '0.825rem', color: 'var(--text-muted)' }}>İzlenen Kaynaklar: </span>
            <strong style={{ color: '#38bdf8', fontSize: '0.9rem', marginLeft: '4px' }}>{activeCount} / {sources.length} Aktif</strong>
          </div>
        </div>

        {/* Arama ve Yeni Kaynak Ekle Butonu */}
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center', flexWrap: 'wrap', paddingTop: '14px', borderTop: '1px solid rgba(56, 189, 248, 0.1)' }}>
          <div style={{ position: 'relative', flex: 1, minWidth: '280px' }}>
            <input
              type="text"
              className="search-input"
              placeholder="Kaynak veya kategori ara..."
              value={search}
              onInput={(e) => setSearch((e.target as HTMLInputElement).value)}
              style={{ width: '100%', padding: '11px 16px', background: '#070b14', border: '1px solid rgba(56, 189, 248, 0.2)', borderRadius: 'var(--radius-md)', color: '#fff', fontSize: '0.9rem', outline: 'none' }}
            />
          </div>
          {isAdmin && (
            <button
              type="button"
              className="btn-primary"
              onClick={() => {
                setShowAddForm(!showAddForm);
                setFormMsg(null);
              }}
              style={{ padding: '10px 18px', fontSize: '0.85rem', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: '8px', cursor: 'pointer', borderRadius: 'var(--radius-md)' }}
            >
              {showAddForm ? '✕ Formu Kapat' : '＋ Yeni X / RSS Kaynağı Ekle'}
            </button>
          )}
        </div>

        {isAdmin && showAddForm && (
          <div style={{ marginTop: '20px', padding: '20px', background: 'rgba(15, 23, 42, 0.85)', border: '1px solid rgba(56, 189, 248, 0.3)', borderRadius: 'var(--radius-md)' }}>
            <div style={{ display: 'flex', gap: '20px', marginBottom: '16px', alignItems: 'center' }}>
              <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-muted)' }}>Kaynak Tipi:</span>
              <label style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', fontSize: '0.85rem', cursor: 'pointer', color: 'var(--text-base)' }}>
                <input
                  type="radio"
                  name="addType"
                  checked={addType === 'twitter'}
                  onChange={() => handleTypeChange('twitter')}
                />
                X / Twitter Hesabı
              </label>
              <label style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', fontSize: '0.85rem', cursor: 'pointer', color: 'var(--text-base)' }}>
                <input
                  type="radio"
                  name="addType"
                  checked={addType === 'rss'}
                  onChange={() => handleTypeChange('rss')}
                />
                Standart RSS Beslemesi
              </label>
            </div>

            <form onSubmit={handleAddSubmit} style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px' }}>
              <div>
                <label style={{ display: 'block', fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '4px' }}>
                  {addType === 'twitter' ? 'X Kullanıcı Adı veya Link' : 'RSS Besleme URL'} *
                </label>
                <input
                  type="text"
                  required
                  placeholder={addType === 'twitter' ? '@vxunderground veya vxunderground' : 'https://example.com/feed.xml'}
                  value={formInput}
                  onInput={(e) => setFormInput((e.target as HTMLInputElement).value)}
                  style={{ width: '100%', fontSize: '0.85rem', padding: '8px 12px', background: '#070b14', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-md)', color: '#fff', outline: 'none' }}
                />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '4px' }}>
                  Kaynak Görünen Adı (İsteğe Bağlı)
                </label>
                <input
                  type="text"
                  placeholder={addType === 'twitter' ? 'X: vx-underground' : 'Güvenlik Bülteni'}
                  value={formName}
                  onInput={(e) => setFormName((e.target as HTMLInputElement).value)}
                  style={{ width: '100%', fontSize: '0.85rem', padding: '8px 12px', background: '#070b14', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-md)', color: '#fff', outline: 'none' }}
                />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '4px' }}>
                  Kategori
                </label>
                <input
                  type="text"
                  placeholder="Twitter Threat Intel / Zafiyet"
                  value={formCategory}
                  onInput={(e) => setFormCategory((e.target as HTMLInputElement).value)}
                  style={{ width: '100%', fontSize: '0.85rem', padding: '8px 12px', background: '#070b14', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-md)', color: '#fff', outline: 'none' }}
                />
              </div>

              <div style={{ display: 'flex', alignItems: 'flex-end', gap: '10px' }}>
                <button
                  type="submit"
                  disabled={formSubmitting}
                  className="btn-primary"
                  style={{ padding: '9px 18px', fontSize: '0.85rem', fontWeight: 600, cursor: 'pointer', borderRadius: 'var(--radius-md)' }}
                >
                  {formSubmitting ? 'Ekleniyor...' : 'Kaydet ve Ekle'}
                </button>
                <button
                  type="button"
                  onClick={() => setShowAddForm(false)}
                  className="btn-secondary"
                  style={{ padding: '9px 14px', fontSize: '0.85rem', cursor: 'pointer', borderRadius: 'var(--radius-md)' }}
                >
                  İptal
                </button>
              </div>
            </form>

            {formMsg && (
              <div
                style={{
                  marginTop: '12px',
                  fontSize: '0.82rem',
                  fontWeight: 600,
                  color: formMsg.type === 'error' ? '#f87171' : '#4ade80',
                }}
              >
                {formMsg.text}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Kaynaklar Listesi Kartı */}
      {loading ? (
        <div style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', textAlign: 'center', padding: '60px', color: 'var(--text-muted)', fontSize: '0.95rem' }}>
          Kaynaklar güvenli belleğe yükleniyor...
        </div>
      ) : (
        <div className="sources-list" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '16px' }}>
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
                className={`source-item-card ${!isActive ? 'is-inactive' : ''}`}
                style={{ background: 'var(--bg-card)', border: '1px solid var(--bg-card-border)', borderRadius: 'var(--radius-lg)', padding: '18px', display: 'flex', flexDirection: 'column', justifyContent: 'space-between', gap: '12px', boxShadow: '0 4px 15px rgba(0, 0, 0, 0.2)' }}
              >
                <div className="source-info" style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  <div className="source-header-row" style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%' }}>
                    <span
                      className={`source-health-dot ${status}`}
                      style={{ flexShrink: 0 }}
                      title={
                        status === 'ok'
                          ? 'Kaynak Sağlıklı'
                          : status === 'error'
                          ? `Hata: ${src.last_error || 'Bağlantı kurulamadı'}`
                          : 'Henüz taranmadı'
                      }
                    />
                    <h5 title={src.name} style={{ margin: 0, fontSize: '0.92rem', fontWeight: 600, color: '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, minWidth: 0 }}>{src.name}</h5>
                    {latency > 0 && isActive && (
                      <span
                        className={`source-latency-badge ${latencyClass}`}
                        title={`Son Yanıt Süresi: ${latency}ms`}
                        style={{ fontSize: '0.72rem', padding: '2px 6px', borderRadius: '4px', flexShrink: 0 }}
                      >
                        {latency}ms
                      </span>
                    )}
                  </div>
                  <div className="source-meta-row" style={{ display: 'flex', flexDirection: 'column', gap: '4px', paddingLeft: '16px' }}>
                    <span className="source-cat" style={{ fontSize: '0.78rem', color: 'var(--text-secondary)' }}>{src.category || 'Genel CTI'}</span>
                    {src.last_error && status === 'error' && (
                      <span
                        style={{ fontSize: '0.72rem', color: '#f87171', wordBreak: 'break-word', lineHeight: '1.3' }}
                        title={src.last_error}
                      >
                        {src.last_error}
                      </span>
                    )}
                  </div>
                </div>

                <div className="source-actions-group" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingTop: '10px', borderTop: '1px solid var(--bg-card-border)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    {isAdmin && src.id && (
                      <label
                        className="source-toggle-switch"
                        title={isActive ? 'Beslemeyi Devre Dışı Bırak' : 'Beslemeyi Etkinleştir'}
                      >
                        <input
                          type="checkbox"
                          checked={isActive}
                          disabled={togglingId === src.id}
                          onChange={() => handleToggle(src)}
                        />
                        <span className="source-toggle-slider" />
                      </label>
                    )}
                    <a
                      href={getLinkUrl(src.url)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="btn-link"
                      title="Besleme Bağlantısı"
                      style={{ fontSize: '0.78rem', textDecoration: 'none', padding: '4px 8px', color: '#38bdf8', fontWeight: 500 }}
                    >
                      XML &rarr;
                    </a>
                  </div>
                  {isAdmin && src.id && (
                    <button
                      type="button"
                      className="btn-icon"
                      disabled={deletingId === src.id}
                      onClick={() => handleDelete(src.id!)}
                      title="Kaynağı Sil"
                      style={{ background: 'transparent', border: 'none', color: '#94a3b8', cursor: 'pointer', padding: '4px 6px', fontSize: '0.9rem' }}
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
  );
}