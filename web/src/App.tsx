import { useEffect, useRef, useState } from 'react';
import {
  clearToken,
  getAnalysis,
  getMe,
  getSignedURL,
  getToken,
  listAnalyses,
  startAnalyze,
  uploadToGCS,
} from './api';
import { AuthPage } from './components/AuthPage';
import { AnalysesSidebar } from './components/AnalysesSidebar';
import { AnalysisForm } from './components/AnalysisForm';
import { AnalysisResult } from './components/AnalysisResult';
import { AnalysisRunning } from './components/AnalysisRunning';
import { ProfilePage } from './components/ProfilePage';
import type { AnalysisListItem, AnalysisStatus, BusinessContext, Metrics, Mode, User } from './types';

type View =
  | { kind: 'form' }
  | { kind: 'uploading'; pct: number }
  | { kind: 'running'; id: string; progressMsg: string }
  | { kind: 'result'; status: AnalysisStatus }
  | { kind: 'profile' }
  | { kind: 'error'; msg: string };

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [authChecked, setAuthChecked] = useState(false);
  const [view, setView] = useState<View>({ kind: 'form' });
  const [list, setList] = useState<AnalysisListItem[]>([]);
  const [currentId, setCurrentId] = useState<string | undefined>();
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    const token = getToken();
    if (!token) { setAuthChecked(true); return; }
    getMe()
      .then((u) => setUser(u))
      .catch(() => { clearToken(); setAuthChecked(true); });
  }, []);

  useEffect(() => {
    if (user) { refreshList(); setAuthChecked(true); }
  }, [user]);

  useEffect(() => {
    return () => { if (pollRef.current) window.clearInterval(pollRef.current); };
  }, []);

  async function refreshList() {
    try { setList(await listAnalyses()); } catch { /* ignore */ }
  }

  function stopPolling() {
    if (pollRef.current) { window.clearInterval(pollRef.current); pollRef.current = null; }
  }

  function startPolling(id: string) {
    stopPolling();
    pollRef.current = window.setInterval(async () => {
      try {
        const s = await getAnalysis(id);
        if (s.status === 'processing') {
          setView({ kind: 'running', id, progressMsg: s.progress_msg });
        } else {
          stopPolling();
          setView({ kind: 'result', status: s });
          refreshList();
        }
      } catch (e) {
        stopPolling();
        setView({ kind: 'error', msg: String(e) });
      }
    }, 3000);
  }

  async function handleSubmit(data: {
    file: File;
    mode: Mode;
    businessContext: BusinessContext;
    metrics?: Metrics;
    userConcept?: string;
  }) {
    try {
      setView({ kind: 'uploading', pct: 0 });
      const signed = await getSignedURL(data.file.name, data.file.type || 'video/mp4');
      await uploadToGCS(signed.put_url, data.file, (pct) => setView({ kind: 'uploading', pct }));
      const { id } = await startAnalyze({
        gcs_uri: signed.gcs_uri,
        original_name: data.file.name,
        mode: data.mode,
        business_context: data.businessContext,
        metrics: data.metrics,
        user_concept: data.userConcept,
      });
      setCurrentId(id);
      setView({ kind: 'running', id, progressMsg: 'Iniciando análise...' });
      startPolling(id);
    } catch (e) {
      setView({ kind: 'error', msg: String(e) });
    }
  }

  async function handleSelect(id: string) {
    stopPolling();
    setCurrentId(id);
    try {
      const s = await getAnalysis(id);
      if (s.status === 'processing') {
        setView({ kind: 'running', id, progressMsg: s.progress_msg });
        startPolling(id);
      } else {
        setView({ kind: 'result', status: s });
      }
    } catch (e) {
      setView({ kind: 'error', msg: String(e) });
    }
  }

  function handleNew() {
    stopPolling();
    setCurrentId(undefined);
    setView({ kind: 'form' });
  }

  function handleLogout() {
    stopPolling();
    clearToken();
    setUser(null);
    setAuthChecked(true);
    setList([]);
    setView({ kind: 'form' });
  }

  function handleProfileSaved(bc: BusinessContext) {
    setUser((u) => u ? { ...u, business_context: bc } : u);
  }

  if (!authChecked) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
        <div className="text-zinc-600 text-sm font-mono">carregando...</div>
      </div>
    );
  }

  if (!user) {
    return <AuthPage onAuth={(u) => setUser(u)} />;
  }

  const hasProfile = !!user.business_context?.brand_name;

  return (
    <div className="h-full flex">
      <AnalysesSidebar
        items={list}
        currentId={currentId}
        onSelect={handleSelect}
        onNew={handleNew}
        onGoToProfile={() => { stopPolling(); setCurrentId(undefined); setView({ kind: 'profile' }); }}
        userEmail={user.email}
        onLogout={handleLogout}
      />
      <main className="flex-1 overflow-y-auto scrollbar-thin">
        <div className="max-w-3xl mx-auto px-8 py-12 w-full">
          {!hasProfile && view.kind === 'form' && (
            <div className="mb-8 rounded-md border border-amber-700/40 bg-amber-950/30 px-4 py-3 text-xs text-amber-300 flex items-center justify-between">
              <span>Configure seu perfil para análises personalizadas</span>
              <button
                onClick={() => setView({ kind: 'profile' })}
                className="ml-4 underline hover:text-amber-200 transition shrink-0"
              >
                Configurar →
              </button>
            </div>
          )}

          {view.kind === 'form' && (
            <AnalysisForm
              user={user}
              onSubmit={handleSubmit}
            />
          )}

          {view.kind === 'uploading' && (
            <AnalysisRunning progressMsg="Subindo vídeo..." uploadPct={view.pct} />
          )}

          {view.kind === 'running' && <AnalysisRunning progressMsg={view.progressMsg} />}

          {view.kind === 'result' && view.status.status === 'done' && view.status.result && (
            <AnalysisResult result={view.status.result} mode={view.status.mode} />
          )}

          {view.kind === 'result' && view.status.status === 'error' && (
            <ErrorBanner msg={view.status.error || 'Erro desconhecido'} onBack={handleNew} />
          )}

          {view.kind === 'error' && <ErrorBanner msg={view.msg} onBack={handleNew} />}

          {view.kind === 'profile' && (
            <ProfilePage user={user} onSaved={handleProfileSaved} />
          )}
        </div>
      </main>
    </div>
  );
}

function ErrorBanner({ msg, onBack }: { msg: string; onBack: () => void }) {
  return (
    <div className="rounded-md border border-rose-700/60 bg-rose-950/40 p-5">
      <div className="flex items-baseline gap-2 mb-2">
        <span className="font-mono text-xs uppercase tracking-widest text-rose-400">erro</span>
      </div>
      <p className="text-sm text-rose-200 leading-relaxed mb-4">{msg}</p>
      <button
        onClick={onBack}
        className="text-xs font-mono uppercase tracking-wider text-rose-300 hover:text-rose-200 underline"
      >
        ← nova análise
      </button>
    </div>
  );
}
