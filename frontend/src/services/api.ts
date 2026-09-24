// Tip güvenli API servis istemcisi - CTIFeed REST Uç Noktaları
import type {
  ThreatStats,
  AnalyticsData,
  SourcesResponse,
  ArticlesResponse,
  IoCResponse,
  ScanResponse,
  FilterParams,
  Article,
  IoCItem
} from '../types/cti';

export async function fetchStats(): Promise<ThreatStats> {
  const res = await fetch('/api/stats');
  if (!res.ok) throw new Error('İstatistik isteği başarısız oldu');
  return res.json();
}

export async function fetchAnalytics(): Promise<AnalyticsData> {
  const res = await fetch('/api/analytics');
  if (!res.ok) throw new Error('Analitik verileri alınamadı');
  return res.json();
}

export async function fetchSources(): Promise<SourcesResponse> {
  const res = await fetch('/api/sources');
  if (!res.ok) throw new Error('Kaynaklar listesi alınamadı');
  return res.json();
}

export async function fetchArticles(params: FilterParams = {}): Promise<ArticlesResponse> {
  const urlParams = new URLSearchParams();
  if (params.search) urlParams.set('search', params.search);
  if (params.tag) urlParams.set('tag', params.tag);
  if (params.source) urlParams.set('source', params.source);
  if (params.min_score && params.min_score > 0) urlParams.set('min_score', params.min_score.toString());
  if (params.sort) urlParams.set('sort', params.sort);
  if (params.time_range) urlParams.set('time_range', params.time_range);
  urlParams.set('limit', (params.limit || 100).toString());
  if (params.offset) urlParams.set('offset', params.offset.toString());

  const res = await fetch(`/api/articles?${urlParams.toString()}`);
  if (!res.ok) throw new Error('Haberler getirilemedi');
  return res.json();
}

export async function fetchIoCs(params: { type?: string; search?: string; limit?: number; offset?: number; article_id?: number } = {}): Promise<IoCResponse> {
  const urlParams = new URLSearchParams();
  if (params.type) urlParams.set('type', params.type);
  if (params.search) urlParams.set('search', params.search);
  if (params.article_id) urlParams.set('article_id', params.article_id.toString());
  urlParams.set('limit', (params.limit || 250).toString());
  if (params.offset) urlParams.set('offset', params.offset.toString());

  const res = await fetch(`/api/iocs?${urlParams.toString()}`);
  if (!res.ok) throw new Error('IoC havuzu verisi alınamadı');
  return res.json();
}

export async function triggerScan(): Promise<ScanResponse> {
  const res = await fetch('/api/scan', { method: 'POST' });
  if (!res.ok) throw new Error('Tarama döngüsü başlatılamadı');
  return res.json();
}

export async function toggleSource(id: number): Promise<{ success: boolean; id: number; is_active: boolean }> {
  const res = await fetch('/api/sources/toggle', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id }),
  });
  if (!res.ok) throw new Error('Kaynak durumu güncellenemedi');
  return res.json();
}
