import type { AnalysisListItem } from '../types';

interface Props {
  items: AnalysisListItem[];
  currentId?: string;
  onSelect: (id: string) => void;
  onNew: () => void;
  userEmail?: string;
  onLogout?: () => void;
}

const STATUS_STYLE: Record<string, string> = {
  processing: 'bg-amber-400 animate-pulse',
  done: 'bg-emerald-500',
  error: 'bg-rose-500',
};

const MODE_LABEL: Record<string, string> = {
  pre_post: 'PRÉ',
  reference: 'REF',
  post_mortem: 'POST',
};

const VERDICT_COLOR: Record<string, string> = {
  'vai bombar': 'text-emerald-400',
  ok: 'text-amber-400',
  'vai flopar': 'text-rose-400',
};

export function AnalysesSidebar({ items, currentId, onSelect, onNew, userEmail, onLogout }: Props) {
  return (
    <aside className="w-64 h-full border-r border-zinc-900 flex flex-col bg-zinc-950/50">
      <div className="p-4 border-b border-zinc-900">
        <div className="flex items-baseline justify-between mb-3">
          <h1 className="font-display text-base font-bold tracking-tight">
            video<span className="text-emerald-500">.</span>analyzer
          </h1>
          <span className="font-mono text-[10px] text-zinc-600">v0.1</span>
        </div>
        <button
          onClick={onNew}
          className="w-full py-2 rounded-md bg-zinc-800 hover:bg-zinc-700 text-xs font-medium tracking-wide transition"
        >
          + nova análise
        </button>
      </div>
      <div className="flex-1 overflow-y-auto scrollbar-thin">
        {items.length === 0 && (
          <div className="p-4 text-xs text-zinc-600">Nenhuma análise ainda.</div>
        )}
        {items.map((it) => (
          <button
            key={it.id}
            onClick={() => onSelect(it.id)}
            className={`w-full text-left p-3 border-b border-zinc-900/60 hover:bg-zinc-900/40 text-sm flex items-center gap-2 transition ${
              currentId === it.id ? 'bg-zinc-900/60' : ''
            }`}
          >
            <span className={`inline-block w-1.5 h-1.5 rounded-full ${STATUS_STYLE[it.status] || 'bg-zinc-600'}`} />
            <span className="font-mono text-[10px] text-zinc-500 w-10 shrink-0">{MODE_LABEL[it.mode]}</span>
            <span className="flex-1 truncate text-zinc-300">{it.original_name || it.id.slice(0, 8)}</span>
            {it.verdict && (
              <span className={`text-[10px] font-mono shrink-0 ${VERDICT_COLOR[it.verdict] || 'text-zinc-500'}`}>
                {it.verdict === 'vai bombar' ? '↑' : it.verdict === 'vai flopar' ? '↓' : '→'}
              </span>
            )}
          </button>
        ))}
      </div>
      {userEmail && onLogout && (
        <div className="p-3 border-t border-zinc-900 flex items-center gap-2">
          <span className="flex-1 truncate text-[10px] text-zinc-600 font-mono">{userEmail}</span>
          <button
            onClick={onLogout}
            className="text-[10px] text-zinc-600 hover:text-zinc-400 transition shrink-0"
          >
            sair
          </button>
        </div>
      )}
    </aside>
  );
}
