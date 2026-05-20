# Video Analysis Tools

Ferramentas de análise de vídeo para a estratégia de conteúdo do ScrapJobs.

## Setup

```bash
cd tools/
npm install
```

Certifique-se de que a variável de ambiente aponta para as credentials:
```bash
export GOOGLE_APPLICATION_CREDENTIALS="/Users/I768258/Desktop/ScrapJobs_project/gen-lang-client-0123498892-3f1abe7b0b6c.json"
```

## Uso

```bash
# Analisar vídeo de referência
node analyze-video.js ./viral-itau.mp4 --name itau-pov

# Analisar seu próprio vídeo antes de postar
node analyze-video.js ./minha-rotina.mp4 --name rotina-sap

# Output será salvo em: tools/outputs/YYYY-MM-DD-<nome>.json
```

## Features Ativas (Free Tier)

- **LABEL_DETECTION**: Identifica objetos, cenas, atividades (1000 min/mês grátis)
- **SHOT_CHANGE_DETECTION**: Timing exato de cada corte (1000 min/mês grátis)

## Features Desabilitadas (Requer Créditos GCP)

- **SPEECH_TRANSCRIPTION**: Transcrição de fala ($0.048/min)
- **TEXT_DETECTION**: OCR de texto na tela ($0.075/min)

Para habilitar, descomente as linhas marcadas em `analyze-video.js`.

## Integração com /content-strategy

Após rodar a análise, use a skill:
```
/content-strategy Analisa esse vídeo viral: tools/outputs/2026-05-16-itau-pov.json
```

A skill vai interpretar o ritmo de cortes, composição visual, e sugerir como replicar.
