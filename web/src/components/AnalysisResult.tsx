import type { ClaudeResult, Mode } from '../types';

interface Props {
  result: ClaudeResult;
  mode?: Mode;
}

const VERDICT_STYLE: Record<string, { bg: string; ring: string; icon: string }> = {
  'vai bombar':         { bg: 'bg-emerald-500 text-zinc-950', ring: 'ring-emerald-500/40', icon: '↑' },
  ok:                   { bg: 'bg-amber-400 text-zinc-950',   ring: 'ring-amber-400/40',   icon: '→' },
  'vai flopar':         { bg: 'bg-rose-500 text-white',       ring: 'ring-rose-500/40',     icon: '↓' },
  'performou bem':      { bg: 'bg-emerald-500 text-zinc-950', ring: 'ring-emerald-500/40', icon: '↑' },
  'na média':           { bg: 'bg-amber-400 text-zinc-950',   ring: 'ring-amber-400/40',   icon: '→' },
  'abaixo do esperado': { bg: 'bg-rose-500 text-white',       ring: 'ring-rose-500/40',     icon: '↓' },
};

export function AnalysisResult({ result, mode }: Props) {
  const style = VERDICT_STYLE[result.verdict] || {
    bg: 'bg-zinc-600 text-white', ring: 'ring-zinc-600/40', icon: '·',
  };

  const isReference = mode === 'reference';

  return (
    <div className="space-y-6">
      <div className={`rounded-lg p-6 ring-2 ${style.ring} ${style.bg} relative overflow-hidden`}>
        <div className="absolute top-2 right-3 font-mono text-xs opacity-50">VEREDITO</div>
        <div className="flex items-baseline gap-3">
          <span className="font-display text-5xl font-bold leading-none">{style.icon}</span>
          <span className="font-display text-3xl font-bold tracking-tight">{result.verdict}</span>
        </div>
        <p className="text-sm mt-2 opacity-90 leading-relaxed">{result.verdict_reason}</p>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <ScoreBlock label="Hook" value={`${result.hook_analysis.score}`} suffix="/10" />
        <InfoBlock label="Ritmo" value={result.visual_analysis.rhythm} />
        <InfoBlock label="Issues" value={`${result.structure_analysis.retention_issues.length}`} />
      </div>

      <Section title="Hook" accent="emerald">
        <p className="text-sm leading-relaxed mb-3">
          <span className="text-zinc-500 font-mono text-xs uppercase tracking-wider mr-2">por quê</span>
          {result.hook_analysis.why}
        </p>
        <p className="text-sm leading-relaxed">
          <span className="text-zinc-500 font-mono text-xs uppercase tracking-wider mr-2">melhorar</span>
          {result.hook_analysis.improvement}
        </p>
      </Section>

      <Section title="Estrutura" accent="amber">
        <p className="text-sm leading-relaxed mb-3">
          <span className="text-zinc-500 font-mono text-xs uppercase tracking-wider mr-2">framework</span>
          {result.structure_analysis.framework_match}
        </p>
        {result.structure_analysis.retention_issues.length > 0 && (
          <ul className="text-sm space-y-1.5 mt-3">
            {result.structure_analysis.retention_issues.map((x, i) => (
              <li key={i} className="flex gap-2">
                <span className="text-rose-400 font-mono shrink-0">⚠</span>
                <span>{x}</span>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Análise visual" accent="sky">
        <dl className="text-sm space-y-2">
          <Row label="Primeiro frame" value={result.visual_analysis.first_frame} />
          <Row label="Visual dominante" value={result.visual_analysis.dominant_visual} />
        </dl>
      </Section>

      {isReference && result.neuromarketing_refs && result.neuromarketing_refs.length > 0 && (
        <Section title="Por que viralizou" accent="violet">
          <div className="flex flex-wrap gap-2 mb-4">
            {result.neuromarketing_refs.map((ref, i) => (
              <span
                key={i}
                className="px-2.5 py-1 rounded text-xs bg-violet-500/10 border border-violet-500/20 text-violet-300"
              >
                {ref}
              </span>
            ))}
          </div>
          {result.viral_elements && result.viral_elements.length > 0 && (
            <>
              <p className="text-zinc-500 font-mono text-xs uppercase tracking-wider mb-2">elementos replicáveis</p>
              <ol className="space-y-1.5">
                {result.viral_elements.map((el, i) => (
                  <li key={i} className="flex gap-2 text-sm">
                    <span className="text-violet-400 font-mono shrink-0">{String(i + 1).padStart(2, '0')}</span>
                    <span>{el}</span>
                  </li>
                ))}
              </ol>
            </>
          )}
        </Section>
      )}

      <Section title="Insights" accent="violet">
        <ul className="space-y-2">
          {result.key_insights.map((x, i) => (
            <li key={i} className="flex gap-3 text-sm leading-relaxed">
              <span className="text-violet-400 font-mono shrink-0">{String(i + 1).padStart(2, '0')}</span>
              <span>{x}</span>
            </li>
          ))}
        </ul>
      </Section>

      <Section title="Próximas ações" accent="emerald">
        <ol className="space-y-2">
          {result.action_items.map((x, i) => (
            <li key={i} className="flex gap-3 text-sm leading-relaxed">
              <span className="text-emerald-400 font-mono shrink-0">→</span>
              <span>{x}</span>
            </li>
          ))}
        </ol>
      </Section>

      {result.replication_script && (
        <Section title="Roteiro adaptado" accent="rose">
          <pre className="whitespace-pre-wrap text-sm bg-zinc-900/60 border border-zinc-800 p-4 rounded font-mono leading-relaxed">
            {result.replication_script}
          </pre>
        </Section>
      )}
    </div>
  );
}

const ACCENT_BORDER: Record<string, string> = {
  emerald: 'border-l-emerald-500',
  amber: 'border-l-amber-500',
  sky: 'border-l-sky-500',
  violet: 'border-l-violet-500',
  rose: 'border-l-rose-500',
};

function Section({ title, children, accent }: { title: string; children: React.ReactNode; accent: string }) {
  return (
    <section className={`rounded-md bg-zinc-900/30 border border-zinc-800 border-l-2 ${ACCENT_BORDER[accent] || 'border-l-zinc-700'} p-5`}>
      <h3 className="font-display text-xs font-semibold uppercase tracking-widest text-zinc-400 mb-4">{title}</h3>
      {children}
    </section>
  );
}

function ScoreBlock({ label, value, suffix }: { label: string; value: string; suffix?: string }) {
  return (
    <div className="rounded-md bg-zinc-900/30 border border-zinc-800 p-4">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500 mb-1">{label}</div>
      <div className="flex items-baseline gap-1">
        <span className="font-display text-3xl font-bold">{value}</span>
        {suffix && <span className="text-sm text-zinc-500">{suffix}</span>}
      </div>
    </div>
  );
}

function InfoBlock({ label, value, suffix }: { label: string; value: string; suffix?: string }) {
  return (
    <div className="rounded-md bg-zinc-900/30 border border-zinc-800 p-4">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500 mb-1">{label}</div>
      <div className="font-display text-lg font-semibold capitalize">
        {value}
        {suffix && <span className="text-sm text-zinc-500 font-normal ml-1">{suffix}</span>}
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex gap-3">
      <dt className="font-mono text-xs uppercase tracking-wider text-zinc-500 w-32 shrink-0 pt-0.5">{label}</dt>
      <dd className="flex-1">{value}</dd>
    </div>
  );
}
