import { useState, useEffect } from 'preact/hooks';
import { activeModal, selectedArticle, closeModal, getScoreCategory } from '../../services/store';
import { fetchIoCs } from '../../services/api';
import type { Article, IoCItem } from '../../types/cti';

const PROD_KEYWORDS = [
  'fortinet', 'cisco', 'wordpress', 'vmware', 'palo-alto', 'microsoft-exchange',
  'active-directory', 'ivanti', 'citrix', 'sonicwall', 'check-point', 'f5',
  'juniper', 'veeam', 'moveit', 'goanywhere', 'atlassian', 'sharepoint',
  'outlook', 'entra-id', 'openssh', 'kubernetes', 'linux'
];

const EXPLOIT_KEYWORDS = ['in-the-wild', 'poc', 'active-exploitation'];

const THREAT_VECS = [
  'zero-day', 'rce', 'ransomware', 'data-breach', 'leak', 'apt',
  'auth-bypass', 'privilege-escalation', 'pre-auth', 'infostealer',
  'wiper', 'spyware', 'c2', 'supply-chain', 'ssrf', 'sqli'
];

export default function ArticleModal() {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [article, setArticle] = useState<Article | null>(null);
  const [iocs, setIocs] = useState<IoCItem[]>([]);
  const [loadingIocs, setLoadingIocs] = useState<boolean>(false);
  const [copiedVal, setCopiedVal] = useState<string | null>(null);

  useEffect(() => {
    const unsubModal = activeModal.subscribe((val) => {
      setIsOpen(val === 'article');
    });
    const unsubArticle = selectedArticle.subscribe((art) => {
      setArticle(art);
      if (art) {
        loadArticleIocs(art.id);
      } else {
        setIocs([]);
      }
    });

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeModal();
    };
    window.addEventListener('keydown', handleKeyDown);

    return () => {
      unsubModal();
      unsubArticle();
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const loadArticleIocs = async (articleId: number) => {
    setLoadingIocs(true);
    try {
      const res = await fetchIoCs({ article_id: articleId });
      setIocs(res.iocs || []);
    } catch (err) {
      console.error('Makale IoC yüklenirken hata:', err);
      setIocs([]);
    } finally {
      setLoadingIocs(false);
    }
  };

  const handleCopy = (val: string) => {
    navigator.clipboard.writeText(val).then(() => {
      setCopiedVal(val);
      setTimeout(() => setCopiedVal(null), 1500);
    });
  };

  if (!isOpen || !article) return null;

  const scoreBadge = getScoreCategory(article.score);
  const tags = article.tags || [];

  // Puan Kırılımı Hesabı
  const hasTr = tags.includes('TR-Focus');
  const matchedCVEs = tags.filter(t => t.toUpperCase().startsWith('CVE-'));
  const matchedProds = tags.filter(t => PROD_KEYWORDS.includes(t));
  const matchedExploits = tags.filter(t => EXPLOIT_KEYWORDS.includes(t));
  const matchedThreats = tags.filter(t => THREAT_VECS.includes(t));

  const hasAnyBreakdown = hasTr || matchedCVEs.length > 0 || matchedProds.length > 0 || matchedExploits.length > 0 || matchedThreats.length > 0;

  return (
    <div class="modal-overlay" role="dialog" aria-modal="true" onClick={(e) => {
      if (e.target === e.currentTarget) closeModal();
    }}>
      <div class="modal-card">
        <button class="modal-close" onClick={closeModal} aria-label="Modalı Kapat">&times;</button>

        <div class="modal-header">
          <div class="modal-meta-row">
            <span class="modal-source">{article.source}</span>
            <span class="modal-date">
              Yayınlanma: {new Date(article.published_at).toLocaleString('tr-TR')}
            </span>
            <span class={`score-badge ${scoreBadge.className}`}>{scoreBadge.label}</span>
          </div>
          <h2 class="modal-title">{article.title}</h2>
        </div>

        <div class="modal-body">
          {/* Öncelik Puanı Dağılımı */}
          <div class="breakdown-box">
            <h4>Öncelik Puanı Dağılımı</h4>
            <div class="breakdown-list">
              {hasTr && (
                <div class="breakdown-item">
                  <span>Türkiye Odağı Tespit Edildi (USOM / TR Kurumları)</span>
                  <span class="breakdown-badge" style="background: rgba(239, 68, 68, 0.2); color: #fca5a5;">+50 PUAN</span>
                </div>
              )}
              {matchedCVEs.length > 0 && (
                <div class="breakdown-item">
                  <span>CVE Güvenlik Zafiyeti Tespit Edildi ({matchedCVEs.join(', ')})</span>
                  <span class="breakdown-badge" style="background: rgba(244, 63, 94, 0.2); color: #fda4af;">+35 PUAN</span>
                </div>
              )}
              {matchedProds.length > 0 && (
                <div class="breakdown-item">
                  <span>Kritik Kurumsal Sistemler ({matchedProds.join(', ')})</span>
                  <span class="breakdown-badge" style="background: rgba(245, 158, 11, 0.2); color: #fde047;">+30 PUAN</span>
                </div>
              )}
              {matchedExploits.length > 0 && (
                <div class="breakdown-item">
                  <span>Aktif Sömürü / PoC Tespit Edildi ({matchedExploits.join(', ')})</span>
                  <span class="breakdown-badge" style="background: rgba(234, 88, 12, 0.2); color: #fb923c;">+25 PUAN</span>
                </div>
              )}
              {matchedThreats.length > 0 && (
                <div class="breakdown-item">
                  <span>Kritik Tehdit Vektörleri ({matchedThreats.join(', ')})</span>
                  <span class="breakdown-badge" style="background: rgba(99, 102, 241, 0.2); color: #c7d2fe;">+20 PUAN</span>
                </div>
              )}
              {!hasAnyBreakdown && (
                <div class="breakdown-item">
                  <span>Standart Tehdit Bülteni</span>
                  <span class="breakdown-badge">+0 PUAN</span>
                </div>
              )}
            </div>
          </div>

          {/* Etiketler */}
          <div class="modal-tags-wrap">
            <h4>Tespit Edilen Etiketler ve CVE Kodları</h4>
            <div class="tags-container">
              {tags.length > 0 ? (
                tags.map((t) => {
                  if (t.toUpperCase().startsWith('CVE-')) {
                    return (
                      <a
                        key={t}
                        href={`https://nvd.nist.gov/vuln/detail/${encodeURIComponent(t)}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="tag-item tag-cve"
                        title="NIST NVD üzerinde incele"
                      >
                        [CVE] {t}
                      </a>
                    );
                  }
                  if (t === 'TR-Focus') {
                    return <span key={t} class="tag-item tag-tr">{t}</span>;
                  }
                  if (t === 'in-the-wild' || t === 'poc' || t === 'active-exploitation') {
                    return <span key={t} class="tag-item tag-exploit">{t}</span>;
                  }
                  return <span key={t} class="tag-item">{t}</span>;
                })
              ) : (
                <span style="color: var(--text-muted); font-size: 0.85rem;">Etiket atanmadı</span>
              )}
            </div>
          </div>

          {/* Özet */}
          <div class="modal-summary-box">
            <h4>İstihbarat Özeti</h4>
            <p>{article.summary || 'Bu kayıt için özet metin bulunmuyor.'}</p>
          </div>

          {/* İlişkili IoC Listesi */}
          {loadingIocs && (
            <div class="modal-iocs-wrap">
              <h4>Tespit Edilen Tehdit Göstergeleri (IoC)</h4>
              <p style="color: var(--text-muted); font-size: 0.85rem;">Tehdit göstergeleri taranıyor...</p>
            </div>
          )}

          {!loadingIocs && iocs.length > 0 && (
            <div class="modal-iocs-wrap">
              <h4>Tespit Edilen Tehdit Göstergeleri (IoC) ({iocs.length})</h4>
              <div class="modal-iocs-list">
                {iocs.map((ioc, idx) => (
                  <div key={idx} class="modal-ioc-item">
                    <span class={`ioc-type-badge ioc-badge-${ioc.type}`}>{ioc.type.toUpperCase()}</span>
                    <span class="ioc-val">{ioc.value}</span>
                    <button
                      class={`btn-copy-ioc ${copiedVal === ioc.value ? 'copied' : ''}`}
                      onClick={() => handleCopy(ioc.value)}
                      title="Panoya Kopyala"
                    >
                      <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                      </svg>
                      <span>{copiedVal === ioc.value ? 'Kopyalandı!' : 'Kopyala'}</span>
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div class="modal-footer">
          <button class="btn-secondary" onClick={closeModal}>Kapat</button>
          <a href={article.link} target="_blank" rel="noopener noreferrer" class="btn-primary">
            <span>Tam Raporu İncele</span>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path>
              <polyline points="15 3 21 3 21 9"></polyline>
              <line x1="10" y1="14" x2="21" y2="3"></line>
            </svg>
          </a>
        </div>
      </div>
    </div>
  );
}
