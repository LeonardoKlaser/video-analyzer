import { useState } from 'react';
import { updateMe } from '../api';
import type { BusinessContext, Platform, User } from '../types';
import { TextField, FieldLabel } from './FormPrimitives';

const PLATFORMS: Platform[] = ['tiktok', 'instagram', 'youtube', 'other'];

interface Props {
  user: User;
  onSaved: (bc: BusinessContext) => void;
}

export function ProfilePage({ user, onSaved }: Props) {
  const [bc, setBC] = useState<BusinessContext>(
    user.business_context ?? {
      brand_name: '', description: '', target_audience: '',
      platforms: ['tiktok'], main_pain: '', content_history: '',
    }
  );
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  function togglePlatform(p: Platform) {
    setBC((prev) => ({
      ...prev,
      platforms: prev.platforms.includes(p)
        ? prev.platforms.filter((x) => x !== p)
        : [...prev.platforms, p],
    }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      await updateMe(bc);
      setSaved(true);
      onSaved(bc);
      setTimeout(() => setSaved(false), 3000);
    } catch {
      // ignore — user can retry
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="max-w-xl">
      <h1 className="font-display text-xl font-bold mb-1">Perfil</h1>
      <p className="text-sm text-zinc-500 mb-8">
        Preenchido uma vez, usado em todas as análises para personalizar os insights.
      </p>
      <form onSubmit={handleSubmit} className="space-y-4">
        <TextField
          label="Marca / Criador" value={bc.brand_name}
          onChange={(v) => setBC({ ...bc, brand_name: v })}
          placeholder="Ex: @perfil, Minha Marca"
        />
        <TextField
          label="O que faz / vende" value={bc.description}
          onChange={(v) => setBC({ ...bc, description: v })}
          placeholder="Ex: curso de design, loja de roupas"
        />
        <TextField
          label="Público-alvo" value={bc.target_audience}
          onChange={(v) => setBC({ ...bc, target_audience: v })}
          placeholder="Ex: mulheres 25–35 interessadas em moda"
        />
        <div>
          <FieldLabel>Plataformas</FieldLabel>
          <div className="flex gap-2 flex-wrap">
            {PLATFORMS.map((p) => (
              <button
                key={p} type="button" onClick={() => togglePlatform(p)}
                className={`px-3 py-1 text-xs rounded-full border transition ${
                  bc.platforms.includes(p)
                    ? 'border-emerald-500 bg-emerald-500/10 text-emerald-300'
                    : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
                }`}
              >
                {p}
              </button>
            ))}
          </div>
        </div>
        <TextField
          label="Principal dor do seu público" value={bc.main_pain}
          onChange={(v) => setBC({ ...bc, main_pain: v })}
          placeholder="Ex: não tem tempo para cozinhar saudável"
        />
        <TextField
          label="O que já funcionou no seu conteúdo" value={bc.content_history}
          onChange={(v) => setBC({ ...bc, content_history: v })}
          placeholder="Ex: vídeos com antes/depois têm mais retenção"
        />
        <button
          type="submit" disabled={saving}
          className="w-full py-3 rounded-md bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 font-display font-semibold text-zinc-950 tracking-wide transition"
        >
          {saved ? '✓ Salvo' : saving ? 'Salvando...' : 'Salvar perfil'}
        </button>
      </form>
    </div>
  );
}
