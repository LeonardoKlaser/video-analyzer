import { useState } from 'react';
import { SectionTitle, VideoUpload, SubmitButton, NoProfileBanner } from './FormPrimitives';

interface Props {
  disabled?: boolean;
  hasProfile: boolean;
  onSubmit: (data: { file: File; userConcept?: string }) => void;
}

export function PrePostForm({ disabled, hasProfile, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [userConcept, setUserConcept] = useState('');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) { alert('Selecione um arquivo de vídeo'); return; }
    onSubmit({ file, userConcept: userConcept.trim() || undefined });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      {!hasProfile && <NoProfileBanner />}

      <section>
        <SectionTitle n="02" title="Vídeo" />
        <VideoUpload file={file} onChange={setFile} disabled={disabled} />
      </section>

      <section>
        <SectionTitle n="03" title="Conceito planejado (opcional)" />
        <p className="text-xs text-zinc-500 mb-3">
          O que você tentou fazer? Claude avalia se o hook executado bate com a intenção.
        </p>
        <textarea
          value={userConcept}
          onChange={(e) => setUserConcept(e.target.value)}
          disabled={disabled}
          rows={2}
          placeholder="Ex: queria criar urgência falando que só funciona nessa época do ano"
          className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition resize-none"
        />
      </section>

      <SubmitButton disabled={disabled} />
    </form>
  );
}
