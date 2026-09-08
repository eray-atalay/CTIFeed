// TypeScript tanımlamaları - CTIFeed Siber Tehdit İstihbarat Platformu

export interface Article {
  id: number;
  title: string;
  link: string;
  summary: string;
  published_at: string;
  source: string;
  score: number;
  tags: string[];
  content?: string;
  created_at?: string;
}

export interface ArticlesResponse {
  total: number;
  page?: number;
  articles: Article[];
}

export interface IoCItem {
  id?: number;
  article_id?: number;
  type: 'ip' | 'domain' | 'sha256' | 'md5' | string;
  value: string;
  threat_context?: string;
  context?: string;
  source?: string;
  first_seen?: string;
  created_at?: string;
  url?: string;
}

export interface IoCResponse {
  total: number;
  iocs: IoCItem[];
}

export interface IoCCounts {
  all: number;
  ip: number;
  domain: number;
  sha256: number;
  md5: number;
}

export interface ThreatStats {
  total_articles: number;
  high_priority_count: number;
  critical_vulnerabilities: number;
  tr_focus_count: number;
  total_iocs: number;
}

export interface TagMetric {
  label: string;
  count: number;
}

export interface VendorMetric {
  vendor: string;
  count: number;
}

export interface TimelineMetric {
  date: string;
  total: number;
  critical: number;
}

export interface AnalyticsData {
  top_tags: TagMetric[];
  top_vendors: VendorMetric[];
  timeline: TimelineMetric[];
}

export interface SourceInfo {
  name: string;
  category?: string;
  url: string;
  count?: number;
  last_fetched?: string;
}

export interface SourcesResponse {
  sources: SourceInfo[];
}

export interface ScanResponse {
  status: string;
  new_inserted: number;
  duplicates_skipped: number;
  duration_ms: number;
}

export interface FilterParams {
  search?: string;
  tag?: string;
  source?: string;
  min_score?: number;
  sort?: string;
  time_range?: string;
  limit?: number;
  offset?: number;
}
