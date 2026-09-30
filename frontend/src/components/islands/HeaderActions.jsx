import { useState } from 'preact/hooks';
import { triggerGlobalRefresh } from '../../services/store';
import { triggerScan } from '../../services/api';

export default function HeaderActions() {
  const [isScanning, setIsScanning] = useState(false);
  const [bannerMsg, setBannerMsg] = useState(null);

  const handleScan = async () => {
    if (isScanning) return;
    setIsScanning(true);
    setBannerMsg('CTI kaynaklarına bağlanılıyor, beslemeler ayrıştırılıyor...');

    try {
      const res = await triggerScan();
      setBannerMsg(`Tarama tamamlandı: ${res.new_inserted} yeni tehdit eklendi.`);
      triggerGlobalRefresh();
      setTimeout(() => setBannerMsg(null), 5000);
    } catch (err) {
      console.error('Tarama hatası:', err);
      setBannerMsg(`Tarama hatası: ${err.message || 'Bilinmeyen hata'}`);
      setTimeout(() => setBannerMsg(null), 4000);
    } finally {
      setIsScanning(false);
    }
  };

  return (
    <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">
      <button
        class={`btn-primary ${isScanning ? 'scanning' : ''}`}
        disabled={isScanning}
        onClick={handleScan}
      >
        <span>{isScanning ? 'Kaynaklar Taranıyor...' : 'Beslemeleri Tara'}</span>
      </button>

      {bannerMsg && (
        <div class="scan-banner" style="margin: 0; padding: 6px 12px; font-size: 0.8rem;">
          <span>{bannerMsg}</span>
        </div>
      )}
    </div>
  );
}