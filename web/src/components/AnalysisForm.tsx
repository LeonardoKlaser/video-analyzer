import { useState } from 'react';
import type { BusinessContext, Metrics, Mode, Platform } from '../types';

interface Props {
  disabled?: boolean;
  savedContext?: BusinessContext;
  onSubmit: (data: {
    file: File;
    mode: Mode;
    businessContext: BusinessContext;
    metrics?: Metrics;
  }) => void;
}

const PLATFORMS: Platform[] = ['tiktok', 'instagram', 'youtube', 'other'];

const MODE_OPTIONS: { value: Mode; label: string; sub: string }[] = [
  { value: 'pre_post', label: 'Pré-postagem', sub: 'vale postar como está?' },
  { value: 'reference', label: 'Referência', sub: 'vídeo viral de terceiro' },
  { value: 'post_mortem', label: 'Post-mortem', sub: 'diagnóstico de já postado' },
];

export function AnalysisForm({ disabled, savedContext, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [mode, setMode] = useState<Mode>('pre_post');
  const [bc, setBC] = useState<BusinessContext>(savedContext ?? {
    brand_name: '',
    description: '',
    target_audience: '',
    platforms: ['tiktok'],
    main_pain: '',
    content_history: '',
  });
  const [metrics, setMetrics] = useState<Metrics>({});

  function togglePlatform(p: Platform) {
    setBC((prev) => ({
      ...prev,
      platforms: prev.platforms.includes(p)
        ? prev.platforms.filter((x) => x !== p)
        : [...prev.platforms, p],
    }));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) {
      alert('Selecione um arquivo de vídeo');
      return;
    }
    const metricsAllowed = mode === 'post_mortem' || mode === 'reference';
    const m = metricsAllowed && Object.keys(metrics).some((k) => metrics[k as keyof Metrics] !== undefined) ? metrics : undefined;
    onSubmit({ file, mode, businessContext: bc, metrics: m });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      <section>
        <SectionTitle n="01" title="Modo" />
        <div className="grid grid-cols-3 gap-2">
          {MODE_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => setMode(opt.value)}
              disabled={disabled}
              className={`rounded-md border p-3 text-left transition ${
                mode === opt.value
                  ? 'border-emerald-500 bg-emerald-500/5'
                  : 'border-zinc-800 hover:border-zinc-600'
              }`}
            >
              <div className="font-display text-sm font-semibold">{opt.label}</div>
              <div className="text-xs text-zinc-500 mt-0.5">{opt.sub}</div>
            </button>
          ))}
        </div>
      </section>

      <section>
        <SectionTitle n="02" title="Vídeo" />
        <label
          htmlFor="video-file"
          className={`flex items-center justify-center gap-3 rounded-md border-2 border-dashed p-6 text-sm cursor-pointer transition ${
            file
              ? 'border-emerald-500 bg-emerald-500/5 text-emerald-300'
              : 'border-zinc-800 hover:border-zinc-600 text-zinc-400'
          }`}
        >
          <span className="text-lg">{file ? '✓' : '↑'}</span>
          <span className="font-mono">
            {file ? `${file.name} · ${(file.size / 1024 / 1024).toFixed(1)} MB` : 'escolha um arquivo .mp4'}
          </span>
        </label>
        <input
          id="video-file"
          type="file"
          accept="video/mp4,video/*"
          onChange={(e) => setFile(e.target.files?.[0] || null)}
          disabled={disabled}
          className="sr-only"
          required
        />
      </section>

      <section>
        <SectionTitle n="03" title="Contexto" />
        <div className="space-y-3">
          <Text label="Marca / Criador" value={bc.brand_name} onChange={(v) => setBC({ ...bc, brand_name: v })} disabled={disabled} placeholder="Ex: Minha Marca, João Silva, @perfil" />
          <Text label="O que faz / vende" value={bc.description} onChange={(v) => setBC({ ...bc, description: v })} disabled={disabled} placeholder="Ex: curso online de design, loja de roupas, consultoria financeira" />
          <Text label="Público-alvo" value={bc.target_audience} onChange={(v) => setBC({ ...bc, target_audience: v })} disabled={disabled} placeholder="Ex: mulheres 25-35 interessadas em moda sustentável" />
          <div>
            <Label>Plataformas</Label>
            <div className="flex gap-2 flex-wrap">
              {PLATFORMS.map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() => togglePlatform(p)}
                  disabled={disabled}
                  className={`px-3 py-1 text-xs rounded-full border transition ${
                    bc.platforms.includes(p)
                      ? 'border-emerald-500 bg-emerald-500/10 text-emerald-300'
                      : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
                  }`}
                >
                  {p}
                </button>
              ))}
            </div>
          </div>
          <Text label="Principal dor do seu público" value={bc.main_pain} onChange={(v) => setBC({ ...bc, main_pain: v })} disabled={disabled} placeholder="Ex: não tem tempo para cozinhar refeições saudáveis no dia a dia" />
          <Text label="O que já funcionou no seu conteúdo" value={bc.content_history} onChange={(v) => setBC({ ...bc, content_history: v })} disabled={disabled} placeholder="Ex: vídeos curtos com antes/depois têm mais retenção que tutoriais longos" />
        </div>
      </section>

      {(mode === 'post_mortem' || mode === 'reference') && (
        <section>
          <SectionTitle n="04" title={mode === 'reference' ? 'Performance do vídeo (opcional)' : 'Métricas (opcional)'} />
          <div className="grid grid-cols-2 gap-3">
            <Num label="Views" value={metrics.views} onChange={(v) => setMetrics({ ...metrics, views: v })} disabled={disabled} />
            <Num label="Likes" value={metrics.likes} onChange={(v) => setMetrics({ ...metrics, likes: v })} disabled={disabled} />
            {mode === 'post_mortem' && (
              <>
                <Num label="Avg watch time (s)" value={metrics.avg_watch_time} onChange={(v) => setMetrics({ ...metrics, avg_watch_time: v })} disabled={disabled} />
                <Num label="Completion rate (0-1)" step={0.01} value={metrics.completion_rate} onChange={(v) => setMetrics({ ...metrics, completion_rate: v })} disabled={disabled} />
                <Num label="Followers ganhos" value={metrics.followers_gained} onChange={(v) => setMetrics({ ...metrics, followers_gained: v })} disabled={disabled} />
              </>
            )}
          </div>
        </section>
      )}

      <button
        type="submit"
        disabled={disabled}
        className="w-full py-3 rounded-md bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 disabled:cursor-not-allowed font-display font-semibold text-zinc-950 tracking-wide transition"
      >
        Analisar →
      </button>
    </form>
  );
}

function SectionTitle({ n, title }: { n: string; title: string }) {
  return (
    <div className="flex items-baseline gap-3 mb-3">
      <span className="font-mono text-xs text-zinc-600">{n}</span>
      <h2 className="font-display text-sm font-semibold uppercase tracking-widest text-zinc-300">{title}</h2>
    </div>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5">{children}</label>;
}

function Text({ label, value, onChange, disabled, placeholder }: { label: string; value: string; onChange: (v: string) => void; disabled?: boolean; placeholder?: string }) {
  return (
    <div>
      <Label>{label}</Label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        placeholder={placeholder}
        className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
        required
      />
    </div>
  );
}

function Num({ label, value, onChange, step = 1, disabled }: { label: string; value?: number; onChange: (v: number | undefined) => void; step?: number; disabled?: boolean }) {
  return (
    <div>
      <Label>{label}</Label>
      <input
        type="number"
        step={step}
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
        disabled={disabled}
        className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm font-mono focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
      />
    </div>
  );
}
