export function SectionTitle({ n, title }: { n: string; title: string }) {
  return (
    <div className="flex items-baseline gap-3 mb-3">
      <span className="font-mono text-xs text-zinc-600">{n}</span>
      <h2 className="font-display text-sm font-semibold uppercase tracking-widest text-zinc-300">{title}</h2>
    </div>
  );
}

export function FieldLabel({ children }: { children: React.ReactNode }) {
  return <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5">{children}</label>;
}

export function TextField({
  label, value, onChange, disabled, placeholder, required,
}: {
  label: string; value: string; onChange: (v: string) => void;
  disabled?: boolean; placeholder?: string; required?: boolean;
}) {
  return (
    <div>
      <FieldLabel>{label}</FieldLabel>
      <input
        type="text" value={value} onChange={(e) => onChange(e.target.value)}
        disabled={disabled} placeholder={placeholder} required={required}
        className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
      />
    </div>
  );
}

export function NumField({
  label, value, onChange, step = 1, disabled,
}: {
  label: string; value?: number; onChange: (v: number | undefined) => void;
  step?: number; disabled?: boolean;
}) {
  return (
    <div>
      <FieldLabel>{label}</FieldLabel>
      <input
        type="number" step={step} value={value ?? ''}
        onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
        disabled={disabled}
        className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm font-mono focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
      />
    </div>
  );
}

export function VideoUpload({
  file, onChange, disabled,
}: {
  file: File | null; onChange: (f: File | null) => void; disabled?: boolean;
}) {
  return (
    <>
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
        id="video-file" type="file" accept="video/mp4,video/*"
        onChange={(e) => onChange(e.target.files?.[0] || null)}
        disabled={disabled} className="sr-only"
      />
    </>
  );
}

export function SubmitButton({ disabled, label = 'Analisar →' }: { disabled?: boolean; label?: string }) {
  return (
    <button
      type="submit" disabled={disabled}
      className="w-full py-3 rounded-md bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 disabled:cursor-not-allowed font-display font-semibold text-zinc-950 tracking-wide transition"
    >
      {label}
    </button>
  );
}

export function NoProfileBanner() {
  return (
    <div className="rounded-md border border-amber-700/40 bg-amber-950/30 px-4 py-3 text-xs text-amber-300">
      Perfil não configurado — análise será genérica. Configure em <strong>Configurações</strong> no sidebar.
    </div>
  );
}
