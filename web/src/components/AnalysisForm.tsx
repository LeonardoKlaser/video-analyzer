import { useState } from 'react';
import type { BusinessContext, Metrics, Mode, User } from '../types';
import { ReferenceForm } from './ReferenceForm';
import { PrePostForm } from './PrePostForm';
import { PostMortemForm } from './PostMortemForm';

interface Props {
  user: User;
  disabled?: boolean;
  onSubmit: (data: {
    file: File;
    mode: Mode;
    businessContext: BusinessContext;
    metrics?: Metrics;
    userConcept?: string;
  }) => void;
}

const MODE_OPTIONS: { value: Mode; label: string; sub: string }[] = [
  { value: 'reference',   label: 'Referência',    sub: 'vídeo viral de terceiro' },
  { value: 'pre_post',    label: 'Pré-postagem',  sub: 'vale postar como está?' },
  { value: 'post_mortem', label: 'Post-mortem',   sub: 'diagnóstico de já postado' },
];

export function AnalysisForm({ user, disabled, onSubmit }: Props) {
  const [mode, setMode] = useState<Mode>('reference');

  const bc: BusinessContext = user.business_context ?? {
    brand_name: '', description: '', target_audience: '',
    platforms: [], main_pain: '', content_history: '',
  };
  const hasProfile = !!user.business_context?.brand_name;

  return (
    <div className="space-y-8">
      <section>
        <div className="flex items-baseline gap-3 mb-3">
          <span className="font-mono text-xs text-zinc-600">01</span>
          <h2 className="font-display text-sm font-semibold uppercase tracking-widest text-zinc-300">Modo</h2>
        </div>
        <div className="grid grid-cols-3 gap-2">
          {MODE_OPTIONS.map((opt) => (
            <button
              key={opt.value} type="button"
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

      {mode === 'reference' && (
        <ReferenceForm
          disabled={disabled}
          onSubmit={({ file, metrics, userConcept }) =>
            onSubmit({ file, mode, businessContext: bc, metrics, userConcept })
          }
        />
      )}
      {mode === 'pre_post' && (
        <PrePostForm
          disabled={disabled}
          hasProfile={hasProfile}
          onSubmit={({ file, userConcept }) =>
            onSubmit({ file, mode, businessContext: bc, userConcept })
          }
        />
      )}
      {mode === 'post_mortem' && (
        <PostMortemForm
          disabled={disabled}
          hasProfile={hasProfile}
          onSubmit={({ file, metrics }) =>
            onSubmit({ file, mode, businessContext: bc, metrics })
          }
        />
      )}
    </div>
  );
}
