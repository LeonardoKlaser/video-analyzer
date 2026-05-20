import { useState } from 'react';
import { login, register, setToken } from '../api';
import type { User } from '../types';

interface Props {
  onAuth: (user: User, token: string) => void;
}

export function AuthPage({ onAuth }: Props) {
  const [tab, setTab] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const fn = tab === 'login' ? login : register;
      const res = await fn(email, password);
      setToken(res.token);
      onAuth(res.user, res.token);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('401') || msg.includes('invalid')) {
        setError('Email ou senha incorretos');
      } else if (msg.includes('409') || msg.includes('already')) {
        setError('Email já cadastrado');
      } else {
        setError('Erro ao conectar. Tente novamente.');
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <h1 className="font-display text-2xl font-bold tracking-tight text-zinc-100">Video Analyzer</h1>
          <p className="text-sm text-zinc-500 mt-1">Análise de vídeos com IA</p>
        </div>

        <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-6">
          <div className="flex rounded-md border border-zinc-800 mb-6 overflow-hidden">
            {(['login', 'register'] as const).map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => { setTab(t); setError(''); }}
                className={`flex-1 py-2 text-sm font-medium transition ${
                  tab === t
                    ? 'bg-emerald-500/10 text-emerald-400'
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                {t === 'login' ? 'Entrar' : 'Criar conta'}
              </button>
            ))}
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5">Email</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoFocus
                placeholder="seu@email.com"
                className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
              />
            </div>
            <div>
              <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5">Senha</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={6}
                placeholder="mínimo 6 caracteres"
                className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
              />
            </div>

            {error && (
              <p className="text-xs text-rose-400 rounded-md bg-rose-950/40 border border-rose-800/40 px-3 py-2">{error}</p>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 rounded-md bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 disabled:cursor-not-allowed font-display font-semibold text-zinc-950 tracking-wide transition"
            >
              {loading ? '...' : tab === 'login' ? 'Entrar →' : 'Criar conta →'}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
