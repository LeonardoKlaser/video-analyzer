import { useState } from 'react';
import type { Metrics } from '../types';
import { SectionTitle, VideoUpload, NumField, SubmitButton } from './FormPrimitives';

interface Props {
  disabled?: boolean;
  onSubmit: (data: { file: File; metrics?: Metrics; userConcept: string }) => void;
}

export function ReferenceForm({ disabled, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [metrics, setMetrics] = useState<Metrics>({});
  const [userConcept, setUserConcept] = useState('');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) { alert('Selecione o vídeo viral de referência'); return; }
    const m = Object.values(metrics).some((v) => v !== undefined) ? metrics : undefined;
    onSubmit({ file, metrics: m, userConcept });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      <section>
        <SectionTitle n="02" title="Vídeo viral de referência" />
        <VideoUpload file={file} onChange={setFile} disabled={disabled} />
      </section>

      <section>
        <SectionTitle n="03" title="O que você quer gravar?" />
        <p className="text-xs text-zinc-500 mb-3">
          Descreva brevemente seu conteúdo planejado. A análise vai montar um roteiro
          personalizado usando os elementos do viral.
        </p>
        <textarea
          value={userConcept}
          onChange={(e) => setUserConcept(e.target.value)}
          disabled={disabled}
          rows={3}
          placeholder="Ex: quero falar sobre como economizar R$500/mês com compras no mercado, meu público é jovens adultos que querem organizar as finanças"
          required
          className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition resize-none"
        />
      </section>

      <section>
        <SectionTitle n="04" title="Performance do viral (opcional)" />
        <div className="grid grid-cols-2 gap-3">
          <NumField label="Views" value={metrics.views} onChange={(v) => setMetrics({ ...metrics, views: v })} disabled={disabled} />
          <NumField label="Likes" value={metrics.likes} onChange={(v) => setMetrics({ ...metrics, likes: v })} disabled={disabled} />
        </div>
      </section>

      <SubmitButton disabled={disabled} />
    </form>
  );
}
