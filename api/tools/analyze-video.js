// analyze-video.js — Google Video Intelligence + Google Speech-to-Text (pt-BR)
// Usage:
//   CLI (legacy):    node analyze-video.js ./video.mp4 [--name NAME]
//   Backend invoke:  node analyze-video.js --uri gs://bucket/key.mp4 --json-stdout

const videoIntelligence = require('@google-cloud/video-intelligence');
const { Storage } = require('@google-cloud/storage');
const speech = require('@google-cloud/speech');
const { execFileSync } = require('child_process');
const ffmpegPath = require('ffmpeg-static');
const fs = require('fs');
const os = require('os');
const path = require('path');

const viClient = new videoIntelligence.VideoIntelligenceServiceClient();
const speechClient = new speech.SpeechClient();
const storage = new Storage();

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag) => { const i = args.indexOf(flag); return i !== -1 ? args[i + 1] : null; };
  const has = (flag) => args.includes(flag);
  const flagsWithValues = ['--uri', '--name'];
  let filePath = null;
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a.startsWith('--')) { if (flagsWithValues.includes(a)) i++; continue; }
    filePath = a;
    break;
  }
  return { filePath, uri: get('--uri'), name: get('--name'), jsonStdout: has('--json-stdout') };
}

function parseTimestamp(timeOffset) {
  if (!timeOffset) return 0;
  return parseInt(timeOffset.seconds || 0) + parseInt(timeOffset.nanos || 0) / 1e9;
}

function classifyRhythm(avgDuration) {
  if (avgDuration < 2) return 'fast';
  if (avgDuration < 4) return 'medium';
  return 'slow';
}

function processLabels(annotationResults) {
  const videoLabels = (annotationResults.segmentLabelAnnotations || [])
    .map((l) => l.entity.description).slice(0, 15);

  const shotLabels = (annotationResults.shotLabelAnnotations || []).map((label) => ({
    label: label.entity.description,
    category: label.categoryEntities?.map((c) => c.description) || [],
    segments: (label.segments || []).map((seg) => ({
      start: `${parseTimestamp(seg.segment.startTimeOffset).toFixed(1)}s`,
      end: `${parseTimestamp(seg.segment.endTimeOffset).toFixed(1)}s`,
      confidence: Math.round(seg.confidence * 100) / 100,
    })),
  }));

  const frameLabels = {};
  (annotationResults.frameLabelAnnotations || []).forEach((label) => {
    (label.frames || []).forEach((frame) => {
      const time = Math.round(parseTimestamp(frame.timeOffset));
      const key = `${time}s`;
      if (!frameLabels[key]) frameLabels[key] = [];
      if (!frameLabels[key].includes(label.entity.description))
        frameLabels[key].push(label.entity.description);
    });
  });

  return { videoLabels, shotLabels, frameLabels };
}

function processShotChanges(annotationResults) {
  const shots = annotationResults.shotAnnotations || [];
  if (shots.length === 0) return { total: 0, averageDuration: 0, timestamps: [], rhythm: 'unknown' };
  const durations = shots.map((s) => parseTimestamp(s.endTimeOffset) - parseTimestamp(s.startTimeOffset));
  const avg = durations.reduce((a, b) => a + b, 0) / durations.length;
  return {
    total: shots.length,
    averageDuration: Math.round(avg * 100) / 100,
    timestamps: shots.map((s) => `${parseTimestamp(s.startTimeOffset).toFixed(1)}s`),
    durations: durations.map((d) => `${d.toFixed(2)}s`),
    rhythm: classifyRhythm(avg),
  };
}

function processTextDetection(annotationResults) {
  const textAnnotations = annotationResults.textAnnotations || [];
  if (textAnnotations.length === 0) return null;

  const seen = new Set();
  const texts = [];

  for (const annotation of textAnnotations) {
    const text = (annotation.text || '').trim();
    if (!text || text.length < 2 || seen.has(text)) continue;
    seen.add(text);

    const firstSeg = (annotation.segments || [])[0];
    const start = firstSeg ? parseTimestamp(firstSeg.segment?.startTimeOffset) : 0;
    const end = firstSeg ? parseTimestamp(firstSeg.segment?.endTimeOffset) : 0;

    texts.push({ text, start: `${start.toFixed(1)}s`, end: `${end.toFixed(1)}s` });
  }

  const hookTexts = texts.filter((t) => parseFloat(t.start) < 5).map((t) => t.text);
  return { hookTexts, allTexts: texts.slice(0, 60) };
}

function buildContentInsights(shotChanges, labels, duration) {
  const cutsPerSecond = duration > 0 ? shotChanges.total / duration : 0;
  const firstShotLabels = labels.shotLabels
    .filter((l) => l.segments.some((s) => s.start === '0.0s'))
    .map((l) => l.label).slice(0, 5);

  const firstFrameLabels = [];
  for (const [time, labs] of Object.entries(labels.frameLabels)) {
    if (parseFloat(time) <= 3)
      labs.forEach((l) => { if (!firstFrameLabels.includes(l)) firstFrameLabels.push(l); });
  }

  return {
    cutsPerSecond: Math.round(cutsPerSecond * 100) / 100,
    rhythm: shotChanges.rhythm,
    totalShots: shotChanges.total,
    averageShotDuration: `${shotChanges.averageDuration}s`,
    firstShotLabels,
    firstFrameLabels: firstFrameLabels.slice(0, 10),
    dominantLabels: labels.videoLabels.slice(0, 5),
  };
}

async function transcribeWithSpeechToText(uri) {
  const tmpMp4 = path.join(os.tmpdir(), `va-${Date.now()}.mp4`);
  const tmpFlac = tmpMp4.replace('.mp4', '.flac');

  try {
    // 1. Download video from GCS
    const withoutScheme = uri.replace('gs://', '');
    const slashIdx = withoutScheme.indexOf('/');
    const bucketName = withoutScheme.slice(0, slashIdx);
    const objectName = withoutScheme.slice(slashIdx + 1);
    await storage.bucket(bucketName).file(objectName).download({ destination: tmpMp4 });

    // 2. Extract audio to FLAC (16kHz mono — ideal para Speech-to-Text)
    execFileSync(ffmpegPath, [
      '-i', tmpMp4,
      '-ar', '16000',
      '-ac', '1',
      '-f', 'flac',
      '-y',
      tmpFlac,
    ], { stdio: 'pipe' });

    // 3. Send to Google Speech-to-Text pt-BR
    const audioBytes = fs.readFileSync(tmpFlac).toString('base64');

    const [operation] = await speechClient.longRunningRecognize({
      audio: { content: audioBytes },
      config: {
        encoding: 'FLAC',
        sampleRateHertz: 16000,
        languageCode: 'pt-BR',
        enableWordTimeOffsets: true,
        enableAutomaticPunctuation: true,
      },
    });

    const [response] = await operation.promise();

    // 4. Process results
    const allWords = [];
    for (const result of response.results || []) {
      const best = (result.alternatives || [])[0];
      if (!best || !best.words) continue;
      for (const w of best.words) {
        allWords.push({
          word: w.word,
          start: parseInt(w.startTime?.seconds || 0) + parseInt(w.startTime?.nanos || 0) / 1e9,
          end: parseInt(w.endTime?.seconds || 0) + parseInt(w.endTime?.nanos || 0) / 1e9,
        });
      }
    }

    if (allWords.length === 0) return null;

    // Group into ~5s segments
    const segments = [];
    let seg = { start: allWords[0].start, words: [] };
    for (const w of allWords) {
      seg.words.push(w.word);
      if (w.end - seg.start >= 5) {
        segments.push({ start: `${seg.start.toFixed(1)}s`, end: `${w.end.toFixed(1)}s`, text: seg.words.join(' ') });
        seg = { start: w.end, words: [] };
      }
    }
    if (seg.words.length > 0) {
      const last = allWords[allWords.length - 1];
      segments.push({ start: `${seg.start.toFixed(1)}s`, end: `${last.end.toFixed(1)}s`, text: seg.words.join(' ') });
    }

    const hookText = allWords.filter((w) => w.start < 5).map((w) => w.word).join(' ') || null;

    return {
      fullTranscript: allWords.map((w) => w.word).join(' '),
      hookText,
      timedSegments: segments,
      language: 'pt-BR',
    };
  } catch (err) {
    return { error: err.message, fullTranscript: null, hookText: null, timedSegments: [] };
  } finally {
    try { fs.unlinkSync(tmpMp4); } catch {}
    try { fs.unlinkSync(tmpFlac); } catch {}
  }
}

function buildRequest({ uri, filePath }) {
  const features = ['LABEL_DETECTION', 'SHOT_CHANGE_DETECTION', 'TEXT_DETECTION'];
  const videoContext = {
    labelDetectionConfig: { labelDetectionMode: 'SHOT_AND_FRAME_MODE', model: 'builtin/latest' },
    shotChangeDetectionConfig: { model: 'builtin/latest' },
  };
  if (uri) return { inputUri: uri, features, videoContext };
  const inputContent = fs.readFileSync(filePath).toString('base64');
  return { inputContent, features, videoContext };
}

async function analyze() {
  const { filePath, uri, name, jsonStdout } = parseArgs();

  if (!uri && !filePath) {
    console.error('Usage: node analyze-video.js (<path> [--name NAME] | --uri gs://... [--json-stdout])');
    process.exit(1);
  }
  if (filePath && !fs.existsSync(filePath)) {
    console.error(`File not found: ${filePath}`);
    process.exit(1);
  }

  if (!jsonStdout) console.log(`Analyzing ${uri || filePath} ...`);

  const request = buildRequest({ uri, filePath });

  try {
    // GVI e Speech-to-Text em paralelo
    const [gviResult, speech] = await Promise.all([
      (async () => {
        const [operation] = await viClient.annotateVideo(request);
        const [result] = await operation.promise();
        return result.annotationResults[0];
      })(),
      uri ? transcribeWithSpeechToText(uri) : Promise.resolve(null),
    ]);

    const shotChanges = processShotChanges(gviResult);
    const labels = processLabels(gviResult);
    const textDetection = processTextDetection(gviResult);

    const lastShot = (gviResult.shotAnnotations || []).slice(-1)[0];
    const totalDuration = lastShot ? parseTimestamp(lastShot.endTimeOffset) : 0;
    const insights = buildContentInsights(shotChanges, labels, totalDuration);

    const output = {
      metadata: {
        source: uri || path.basename(filePath || ''),
        duration: `${totalDuration.toFixed(1)}s`,
        analyzedAt: new Date().toISOString(),
        features: ['LABEL_DETECTION', 'SHOT_CHANGE_DETECTION', 'TEXT_DETECTION', 'SPEECH_TRANSCRIPTION_PT_BR'],
      },
      shotChanges,
      labels: {
        videoLevel: labels.videoLabels,
        byShot: labels.shotLabels.slice(0, 30),
        byFrame: labels.frameLabels,
      },
      textDetection,
      contentInsights: insights,
      speech,
    };

    if (jsonStdout) {
      process.stdout.write(JSON.stringify(output));
      return;
    }

    const outputName = name || path.basename(filePath || '', path.extname(filePath || ''));
    const date = new Date().toISOString().split('T')[0];
    const outputPath = path.join(__dirname, 'outputs', `${date}-${outputName}.json`);
    fs.mkdirSync(path.dirname(outputPath), { recursive: true });
    fs.writeFileSync(outputPath, JSON.stringify(output, null, 2));
    console.log(`\nSaved: ${outputPath}`);
  } catch (error) {
    if (error.code === 7) console.error('ERROR: Billing not enabled or quota exceeded.');
    else if (error.code === 3) console.error('ERROR: Unsupported video format or file too large.');
    else console.error('API error:', error.message, '(code', error.code, ')');
    process.exit(1);
  }
}

analyze();
