export type Mode = 'pre_post' | 'reference' | 'post_mortem';

export interface User {
  id: string;
  email: string;
  business_context?: BusinessContext;
  created_at: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export type Platform = 'tiktok' | 'instagram' | 'youtube' | 'other';

export interface BusinessContext {
  brand_name: string;
  description: string;
  target_audience: string;
  platforms: Platform[];
  main_pain: string;
  content_history: string;
}

export interface Metrics {
  views?: number;
  likes?: number;
  avg_watch_time?: number;
  completion_rate?: number;
  followers_gained?: number;
}

export interface SignedURLResponse {
  put_url: string;
  gcs_uri: string;
  expires_at: string;
}

export interface StartAnalyzeRequest {
  gcs_uri: string;
  original_name: string;
  mode: Mode;
  business_context: BusinessContext;
  metrics?: Metrics;
  user_concept?: string;
}

export interface AnalysisStatus {
  id: string;
  status: 'processing' | 'done' | 'error';
  mode: Mode;
  progress_msg: string;
  created_at: string;
  completed_at?: string;
  result?: ClaudeResult;
  error?: string;
}

export interface ClaudeResult {
  hook_analysis: { score: number; why: string; improvement: string };
  structure_analysis: { framework_match: string; retention_issues: string[] };
  visual_analysis: { rhythm: string; first_frame: string; dominant_visual: string };
  key_insights: string[];
  action_items: string[];
  replication_script?: string;
  neuromarketing_refs?: string[];
  viral_elements?: string[];
  verdict:
    | 'vai bombar' | 'ok' | 'vai flopar'
    | 'performou bem' | 'na média' | 'abaixo do esperado'
    | string;
  verdict_reason: string;
}

export interface AnalysisListItem {
  id: string;
  mode: Mode;
  status: 'processing' | 'done' | 'error';
  original_name?: string;
  verdict?: string;
  created_at: string;
}
