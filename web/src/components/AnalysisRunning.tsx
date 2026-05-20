interface Props {
  progressMsg: string;
  uploadPct?: number;
}

const STAGES = [
  'Iniciando análise...',
  'Subindo vídeo...',
  'Analisando estrutura visual...',
  'Identificando padrões de corte...',
  'Gerando insights com IA...',
];

function stageIndex(msg: string): number {
  if (!msg) return 0;
  if (msg.includes('Iniciando')) return 0;
  if (msg.includes('Subindo')) return 1;
  if (msg.includes('estrutura visual')) return 2;
  if (msg.includes('padrões')) return 3;
  if (msg.includes('insights')) return 4;
  return 0;
}

export function AnalysisRunning({ progressMsg, uploadPct }: Props) {
  const current = stageIndex(progressMsg);

  return (
    <div className="space-y-8">
      <div className="text-center space-y-2">
        <div className="inline-flex items-center gap-2 text-emerald-400">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
          </span>
          <span className="font-mono text-xs uppercase tracking-widest">analisando</span>
        </div>
        <h2 className="font-display text-2xl">{progressMsg || 'Trabalhando...'}</h2>
        <p className="text-sm text-zinc-500">
          Pode levar de 1 a 3 minutos. Pode atualizar a página — o resultado fica no histórico.
        </p>
      </div>

      {typeof uploadPct === 'number' && (
        <div className="space-y-1">
          <div className="flex items-baseline justify-between text-xs font-mono">
            <span className="text-zinc-500">UPLOAD</span>
            <span className="text-emerald-400">{Math.round(uploadPct * 100)}%</span>
          </div>
          <div className="w-full bg-zinc-900 rounded-full h-1 overflow-hidden">
            <div
              className="bg-emerald-500 h-full transition-all duration-200"
              style={{ width: `${Math.round(uploadPct * 100)}%` }}
            />
          </div>
        </div>
      )}

      <ol className="space-y-2">
        {STAGES.map((stage, i) => {
          const state = i < current ? 'done' : i === current ? 'active' : 'pending';
          return (
            <li
              key={stage}
              className={`flex items-center gap-3 text-sm transition ${
                state === 'done'
                  ? 'text-zinc-600'
                  : state === 'active'
                    ? 'text-zinc-100'
                    : 'text-zinc-700'
              }`}
            >
              <span className="font-mono text-xs w-6 text-right">
                {state === 'done' ? '✓' : state === 'active' ? '·' : '○'}
              </span>
              <span className={state === 'active' ? 'font-medium' : ''}>{stage}</span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
