// Merkezi Reaktif Store & Event Bus - Astro Adaları Arası İletişim
import type { Article } from '../types/cti';

export type ModalType = 'article' | 'cve' | 'ioc' | 'sources' | 'breach' | null;

type Listener<T> = (val: T) => void;

function createSignal<T>(initialValue: T) {
  let value = initialValue;
  const listeners = new Set<Listener<T>>();

  return {
    get: () => value,
    set: (newValue: T) => {
      value = newValue;
      listeners.forEach(fn => fn(value));
    },
    subscribe: (fn: Listener<T>) => {
      listeners.add(fn);
      fn(value);
      return () => listeners.delete(fn);
    }
  };
}

export const activeModal = createSignal<ModalType>(null);
export const selectedArticle = createSignal<Article | null>(null);
export const activeTagFilter = createSignal<string>('');
export const refreshTrigger = createSignal<number>(0);

export function openArticleModal(article: Article) {
  selectedArticle.set(article);
  activeModal.set('article');
}

export function openModal(modal: ModalType) {
  activeModal.set(modal);
}

export function closeModal() {
  activeModal.set(null);
  selectedArticle.set(null);
}

export function triggerGlobalRefresh() {
  refreshTrigger.set(Date.now());
}

export function setTagFilter(tag: string) {
  activeTagFilter.set(tag);
}

// Yardımcı Formatlayıcılar
export function formatTimeAgo(dateString?: string): string {
  if (!dateString) return '-';
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  if (diffMin < 1) return 'Şimdi';
  if (diffMin < 60) return `${diffMin} dk önce`;
  if (diffHour < 24) return `${diffHour} saat önce`;
  if (diffDay === 1) return 'Dün';
  return `${diffDay} gün önce`;
}

export function getScoreCategory(score: number): { label: string; className: string } {
  if (score >= 80) return { label: `${score} KRİTİK`, className: 'score-critical' };
  if (score >= 50) return { label: `${score} YÜKSEK`, className: 'score-high' };
  if (score >= 30) return { label: `${score} ORTA`, className: 'score-medium' };
  return { label: `${score} DÜŞÜK`, className: 'score-low' };
}
