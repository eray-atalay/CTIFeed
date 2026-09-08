import { useState, useEffect } from 'preact/hooks';
import { openModal, triggerGlobalRefresh, refreshTrigger } from '../../services/store';
import { fetchSources, triggerScan } from '../../services/api';

export default function HeaderActions() {
  const [sourceCount, setSourceCount] = useState<number>(0);
  const [isScanning, setIsScanning] = useState<boolean>(false);
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);

  const loadSources = async () => {
    try {
      const data = await fetchSources();
      setSourceCount(data.sources?.length || 0);
    } catch (e) {
      console.error('Kaynak sayısı alınamadı:', e);
    }
  };

  useEffect(() => {
    loadSources();
    return refreshTrigger.subscribe(() => {
      loadSources();
    });
  }, []);

  const handleScan = async () => {
    if (isScanning) return;
    setIsScanning(true);
    setBannerMsg(`${sourceCount || 24} CTI ve Telegram kaynağına bağlanılıyor, beslemeler ayrıştırılıyor...`);

    try {
      const res = await triggerScan();
      setBannerMsg(`Tarama tamamlandı: ${res.new_inserted} yeni tehdit maddesi eklendi, ${res.duplicates_skipped} mükerrer kayıt atlandı.`);
      triggerGlobalRefresh();
      setTimeout(() => setBannerMsg(null), 5000);
    } catch (err: any) {
      console.error('Tarama hatası:', err);
      setBannerMsg(`Tarama hatası: ${err.message || 'Bilinmeyen hata'}`);
      setTimeout(() => setBannerMsg(null), 4000);
    } finally {
      setIsScanning(false);
    }
  };

  return (
    <>
      <div class="header-actions">
        {/* CVE Özel Radarı Butonu */}
        <button
          class="btn-secondary"
          title="Telegram ve CTI Kaynaklı CVE Zafiyetleri"
          onClick={() => openModal('cve')}
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
          </svg>
          <span>CVE Radarı</span>
        </button>

        {/* Telegram Bot Yönlendirme Butonu */}
        <a
          href="https://t.me/ctifeed_radar_bot"
          target="_blank"
          rel="noopener noreferrer"
          class="btn-secondary btn-telegram"
          title="Telegram Bildirim Botunu Aç"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69a.2.2 0 0 0-.05-.18c-.06-.05-.14-.03-.21-.02-.09.02-1.49.95-4.22 2.79-.4.27-.76.41-1.08.4-.36-.01-1.04-.2-1.55-.37-.63-.2-1.12-.31-1.08-.66.02-.18.27-.36.74-.55 2.92-1.27 4.86-2.11 5.83-2.51 2.78-1.16 3.35-1.36 3.73-1.36.08 0 .27.02.39.12.1.08.13.19.14.27-.01.06.01.24 0 .38z"/>
          </svg>
          <span>@ctifeed_radar_bot</span>
        </a>

        {/* IoC Havuzu Butonu */}
        <button
          class="btn-secondary"
          title="Tehdit Göstergeleri (IoC) Havuzu & Dışa Aktarım"
          onClick={() => openModal('ioc')}
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
          </svg>
          <span>IoC Havuzu</span>
        </button>

        {/* Aktif Kaynaklar Butonu */}
        <button
          class="btn-secondary"
          title="Aktif Tehdit İstihbarat Kaynakları"
          onClick={() => openModal('sources')}
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M4 11a9 9 0 0 1 9 9"></path>
            <path d="M4 4a16 16 0 0 1 16 16"></path>
            <circle cx="5" cy="19" r="1"></circle>
          </svg>
          <span>Kaynaklar</span>
          <span class="badge">{sourceCount || '24'}</span>
        </button>

        {/* Canlı Tarama Tetikleme Butonu */}
        <button
          class={`btn-primary ${isScanning ? 'scanning' : ''}`}
          title="Tüm Kaynakları Yeniden Tara"
          disabled={isScanning}
          onClick={handleScan}
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" class={isScanning ? 'icon-spin' : ''}>
            <polyline points="23 4 23 10 17 10"></polyline>
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path>
          </svg>
          <span>{isScanning ? `${sourceCount || 24} Kaynak Taranıyor...` : 'Beslemeleri Tara'}</span>
        </button>
      </div>

      {/* Tarama Durum Bildirim Afişi */}
      {bannerMsg && (
        <div class="scan-banner" style="display: flex; align-items: center; justify-content: center; gap: 8px;">
          <div class="spinner-sm"></div>
          <span>{bannerMsg}</span>
        </div>
      )}
    </>
  );
}
