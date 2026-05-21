import { useState } from 'react';
import type { Metrics } from '../types';
import { SectionTitle, VideoUpload, NumField, SubmitButton, NoProfileBanner } from './FormPrimitives';

interface Props {
  disabled?: boolean;
  hasProfile: boolean;
  onSubmit: (data: { file: File; metrics?: Metrics }) => void;
}

export function PostMortemForm({ disabled, hasProfile, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [metrics, setMetrics] = useState<Metrics>({});

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) { alert('Selecione o vídeo já postado'); return; }
    const m = Object.values(metrics).some((v) => v !== undefined) ? metrics : undefined;
    onSubmit({ file, metrics: m });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      {!hasProfile && <NoProfileBanner />}

      <section>
        <SectionTitle n="02" title="Vídeo postado" />
        <VideoUpload file={file} onChange={setFile} disabled={disabled} />
      </section>

      <section>
        <SectionTitle n="03" title="Métricas (opcional)" />
        <div className="grid grid-cols-2 gap-3">
          <NumField label="Views" value={metrics.views} onChange={(v) => setMetrics({ ...metrics, views: v })} disabled={disabled} />
          <NumField label="Likes" value={metrics.likes} onChange={(v) => setMetrics({ ...metrics, likes: v })} disabled={disabled} />
          <NumField label="Avg watch time (s)" value={metrics.avg_watch_time} onChange={(v) => setMetrics({ ...metrics, avg_watch_time: v })} disabled={disabled} />
          <NumField label="Completion rate (0–1)" step={0.01} value={metrics.completion_rate} onChange={(v) => setMetrics({ ...metrics, completion_rate: v })} disabled={disabled} />
          <NumField label="Followers ganhos" value={metrics.followers_gained} onChange={(v) => setMetrics({ ...metrics, followers_gained: v })} disabled={disabled} />
        </div>
      </section>

      <SubmitButton disabled={disabled} />
    </form>
  );
}
