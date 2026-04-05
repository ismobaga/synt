import asyncio
import io
import os
import subprocess
import tempfile
import wave
from pathlib import Path
from typing import Any

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse, Response
from pydantic import BaseModel

try:
    import edge_tts
except Exception as exc:  # pragma: no cover - import best effort
    edge_tts = None
    EDGE_TTS_IMPORT_ERROR = str(exc)
else:
    EDGE_TTS_IMPORT_ERROR = ""

app = FastAPI(title="synt-local-tts", version="0.1.0")


def _load_kitten_runtime():
    errors: list[str] = []
    for module_name in ("kittentts", "kitten_tts", "KittenTTS"):
        try:
            module = __import__(module_name, fromlist=["*"])
        except Exception as exc:  # pragma: no cover - best effort import
            errors.append(f"{module_name}: {exc}")
            continue
        for attr in ("KittenTTS", "TTS", "Kitten"):
            candidate = getattr(module, attr, None)
            if candidate is not None:
                return candidate, ""
        return module, ""
    return None, "; ".join(errors)


KITTEN_RUNTIME, KITTEN_IMPORT_ERROR = _load_kitten_runtime()
KITTEN_MODELS: dict[str, Any] = {}
KITTEN_VOICES = ["Bella", "Jasper", "Luna", "Bruno", "Rosie", "Hugo", "Kiki", "Leo"]

EDGE_TTS_VOICES = [
    "en-US-JennyNeural",
    "en-US-GuyNeural",
    "en-GB-SoniaNeural",
    "fr-FR-DeniseNeural",
    "de-DE-KatjaNeural",
    "es-ES-ElviraNeural",
    "pt-BR-FranciscaNeural",
]

MODEL_ALIASES = {
    "kitten": "KittenML/kitten-tts-mini-0.8",
    "kitten-mini": "KittenML/kitten-tts-mini-0.8",
    "kitten-tts-mini": "KittenML/kitten-tts-mini-0.8",
    "kittenml/kitten-tts-mini-0.8": "KittenML/kitten-tts-mini-0.8",
    "edge": "microsoft/edge-tts",
    "edge-tts": "microsoft/edge-tts",
    "microsoft/edge-tts": "microsoft/edge-tts",
    "resemble-ai/chatterbox": "resemble-ai/chatterbox",
    "chatterbox": "resemble-ai/chatterbox",
    "fishaudio/vibevoice-realtime-0.5b": "fishaudio/VibeVoice-Realtime-0.5B",
    "vibevoice": "fishaudio/VibeVoice-Realtime-0.5B",
    "vibevoice-realtime-0.5b": "fishaudio/VibeVoice-Realtime-0.5B",
    "microsoft/speecht5_tts": "microsoft/speecht5_tts",
}
SUPPORTED_MODELS = [
    "KittenML/kitten-tts-mini-0.8",
    "microsoft/edge-tts",
    "resemble-ai/chatterbox",
    "fishaudio/VibeVoice-Realtime-0.5B",
    "microsoft/speecht5_tts",
]


class OpenAITTSRequest(BaseModel):
    input: str | None = None
    text: str | None = None
    voice: str | None = None
    speed: float | None = 1.0
    response_format: str | None = "wav"
    format: str | None = None
    model: str | None = None
    language: str | None = "en"


def first_non_empty(*values: Any) -> str:
    for value in values:
        if value is None:
            continue
        text = str(value).strip()
        if text:
            return text
    return ""


def normalize_model_name(model: str) -> str:
    model = first_non_empty(model)
    if not model:
        return os.getenv("LOCAL_TTS_DEFAULT_MODEL", "KittenML/kitten-tts-mini-0.8")
    lowered = model.strip().lower()
    return MODEL_ALIASES.get(lowered, model.strip())


def select_kitten_voice(voice: str) -> str:
    chosen = first_non_empty(voice, os.getenv("KITTEN_TTS_VOICE"), os.getenv("LOCAL_TTS_DEFAULT_VOICE"), "Jasper")
    lowered = chosen.lower()
    for candidate in KITTEN_VOICES:
        if candidate.lower() == lowered:
            return candidate
    return "Jasper"


def select_edge_voice(language: str, voice: str) -> str:
    chosen = first_non_empty(voice, os.getenv("EDGE_TTS_VOICE"))
    if chosen:
        return chosen
    language = first_non_empty(language, "en").lower()
    if language.startswith("es"):
        return "es-ES-ElviraNeural"
    if language.startswith("fr"):
        return "fr-FR-DeniseNeural"
    if language.startswith("de"):
        return "de-DE-KatjaNeural"
    if language.startswith("pt"):
        return "pt-BR-FranciscaNeural"
    return "en-US-JennyNeural"


def select_voice(language: str, voice: str) -> str:
    if first_non_empty(voice):
        return voice.strip()
    language = first_non_empty(language, "en").lower()
    if language.startswith("es"):
        return "es"
    if language.startswith("fr"):
        return "fr"
    if language.startswith("de"):
        return "de"
    if language.startswith("pt"):
        return "pt"
    return os.getenv("LOCAL_TTS_DEFAULT_VOICE", "en-us")


def should_use_kitten_runtime(model_name: str) -> bool:
    backend = first_non_empty(os.getenv("LOCAL_TTS_BACKEND"), "auto").lower()
    return model_name.startswith("KittenML/") or backend in {"kitten", "kittentts"}


def should_use_edge_runtime(model_name: str) -> bool:
    backend = first_non_empty(os.getenv("LOCAL_TTS_BACKEND"), "auto").lower()
    lowered = model_name.lower()
    return lowered == "microsoft/edge-tts" or backend in {"edge", "edge-tts"}


def get_kitten_model(model_name: str):
    if KITTEN_RUNTIME is None:
        raise RuntimeError(f"KittenTTS runtime is unavailable: {KITTEN_IMPORT_ERROR or 'not installed'}")
    if model_name in KITTEN_MODELS:
        return KITTEN_MODELS[model_name]

    constructors = [
        lambda: KITTEN_RUNTIME(model_name),
        lambda: KITTEN_RUNTIME(model=model_name),
        lambda: KITTEN_RUNTIME(model_name=model_name),
        lambda: KITTEN_RUNTIME(),
    ]
    last_error: Exception | None = None
    for builder in constructors:
        try:
            instance = builder()
            KITTEN_MODELS[model_name] = instance
            return instance
        except TypeError as exc:
            last_error = exc
            continue
    if last_error is not None:
        raise last_error
    raise RuntimeError("unable to initialize KittenTTS runtime")


def write_wave_bytes(samples: Any, sample_rate: int = 24000) -> bytes:
    if isinstance(samples, (bytes, bytearray)):
        return bytes(samples)

    if hasattr(samples, "tolist"):
        samples = samples.tolist()
    if not isinstance(samples, (list, tuple)):
        raise RuntimeError(f"unsupported audio sample type: {type(samples)!r}")

    if samples and isinstance(samples[0], (list, tuple)):
        samples = samples[0]

    clipped = []
    for sample in samples:
        try:
            value = float(sample)
        except Exception:
            value = 0.0
        value = max(-1.0, min(1.0, value))
        clipped.append(int(value * 32767.0))

    buffer = io.BytesIO()
    with wave.open(buffer, "wb") as wav_file:
        wav_file.setnchannels(1)
        wav_file.setsampwidth(2)
        wav_file.setframerate(int(sample_rate or 24000))
        wav_file.writeframes(b"".join(int(sample).to_bytes(2, byteorder="little", signed=True) for sample in clipped))
    return buffer.getvalue()


def synthesize_with_kitten(text: str, language: str, voice: str, speed: float, model_name: str) -> bytes:
    instance = get_kitten_model(model_name)
    voice_name = select_kitten_voice(voice)

    calls = []
    if hasattr(instance, "generate"):
        calls.extend([
            lambda: instance.generate(text, voice=voice_name, speed=speed),
            lambda: instance.generate(text=text, voice=voice_name, speed=speed),
            lambda: instance.generate(text, speaker=voice_name, speed=speed),
            lambda: instance.generate(text=text, speaker=voice_name, speed=speed),
            lambda: instance.generate(text, voice=voice_name),
            lambda: instance.generate(text=text, voice=voice_name),
        ])
    if hasattr(instance, "synthesize"):
        calls.extend([
            lambda: instance.synthesize(text, voice=voice_name, speed=speed),
            lambda: instance.synthesize(text=text, voice=voice_name, speed=speed),
            lambda: instance.synthesize(text, voice=voice_name),
            lambda: instance.synthesize(text=text, voice=voice_name),
        ])
    if callable(instance):
        calls.extend([
            lambda: instance(text, voice=voice_name, speed=speed),
            lambda: instance(text, voice=voice_name),
            lambda: instance(text),
        ])

    last_error: Exception | None = None
    for call in calls:
        try:
            result = call()
            break
        except TypeError as exc:
            last_error = exc
            continue
    else:
        raise RuntimeError(f"KittenTTS runtime did not expose a supported synth API: {last_error}")

    sample_rate = 24000
    audio = result
    if isinstance(result, tuple) and len(result) == 2:
        first, second = result
        if isinstance(first, int):
            sample_rate, audio = first, second
        elif isinstance(second, int):
            audio, sample_rate = first, second
    elif isinstance(result, dict):
        audio = result.get("audio") or result.get("samples") or result.get("wav") or result.get("data")
        sample_rate = int(result.get("sample_rate", sample_rate))

    return write_wave_bytes(audio, sample_rate=sample_rate)


async def _save_edge_audio(text: str, voice_name: str, rate: str, output_path: str) -> None:
    communicator = edge_tts.Communicate(text, voice_name, rate=rate)
    await communicator.save(output_path)


def edge_rate_value(speed: float) -> str:
    percent = int(round((speed - 1.0) * 100))
    return f"{percent:+d}%"


def synthesize_with_edge_tts(text: str, language: str, voice: str, speed: float) -> bytes:
    if edge_tts is None:
        raise RuntimeError(f"edge-tts runtime is unavailable: {EDGE_TTS_IMPORT_ERROR or 'not installed'}")

    output_dir = Path(os.getenv("LOCAL_TTS_OUTPUT_DIR", "/tmp/tts-local"))
    output_dir.mkdir(parents=True, exist_ok=True)

    with tempfile.NamedTemporaryFile(suffix=".mp3", dir=output_dir, delete=False) as mp3_tmp:
        mp3_path = Path(mp3_tmp.name)
    with tempfile.NamedTemporaryFile(suffix=".wav", dir=output_dir, delete=False) as wav_tmp:
        wav_path = Path(wav_tmp.name)

    try:
        asyncio.run(_save_edge_audio(text, select_edge_voice(language, voice), edge_rate_value(speed), str(mp3_path)))
        subprocess.run(
            ["ffmpeg", "-y", "-i", str(mp3_path), "-ac", "1", "-ar", "24000", str(wav_path)],
            check=True,
            capture_output=True,
            text=True,
        )
        data = wav_path.read_bytes()
    finally:
        mp3_path.unlink(missing_ok=True)
        wav_path.unlink(missing_ok=True)

    if not data:
        raise RuntimeError("edge-tts returned empty audio")
    return data


def synthesize_with_espeak(text: str, language: str, voice: str, speed: float) -> bytes:
    output_dir = Path(os.getenv("LOCAL_TTS_OUTPUT_DIR", "/tmp/tts-local"))
    output_dir.mkdir(parents=True, exist_ok=True)

    with tempfile.NamedTemporaryFile(suffix=".wav", dir=output_dir, delete=False) as tmp:
        output_path = Path(tmp.name)

    rate = max(120, min(260, round(175 * speed)))
    command = [
        os.getenv("LOCAL_TTS_COMMAND", "espeak"),
        "-w",
        str(output_path),
        "-s",
        str(rate),
        "-v",
        select_voice(language, voice),
        text,
    ]

    try:
        subprocess.run(command, check=True, capture_output=True, text=True)
    except subprocess.CalledProcessError as exc:
        stderr = (exc.stderr or "").strip()
        raise HTTPException(status_code=500, detail=f"local tts failed: {stderr or exc}") from exc

    data = output_path.read_bytes()
    output_path.unlink(missing_ok=True)
    if not data:
        raise HTTPException(status_code=500, detail="local tts returned empty audio")
    return data


def synthesize_to_wav(text: str, language: str, voice: str, speed: float, model_name: str = "") -> tuple[bytes, str]:
    text = first_non_empty(text)
    if not text:
        raise HTTPException(status_code=400, detail="text is required")

    try:
        speed = float(speed or 1.0)
    except Exception:
        speed = 1.0
    if speed <= 0:
        speed = 1.0

    model_name = normalize_model_name(model_name or os.getenv("LOCAL_TTS_DEFAULT_MODEL", "KittenML/kitten-tts-mini-0.8"))
    if should_use_edge_runtime(model_name):
        try:
            return synthesize_with_edge_tts(text, language, voice, speed), "edge-tts"
        except Exception:
            pass
    if should_use_kitten_runtime(model_name):
        try:
            return synthesize_with_kitten(text, language, voice, speed, model_name), "kitten"
        except Exception:
            pass

    return synthesize_with_espeak(text, language, voice, speed), "espeak"


@app.get("/")
def root() -> dict[str, Any]:
    return {
        "service": "synt-local-tts",
        "backend": os.getenv("LOCAL_TTS_BACKEND", "espeak"),
        "default_model": normalize_model_name(os.getenv("LOCAL_TTS_DEFAULT_MODEL", "KittenML/kitten-tts-mini-0.8")),
        "kitten_available_voices": KITTEN_VOICES,
        "edge_available_voices": EDGE_TTS_VOICES,
        "supported_models": SUPPORTED_MODELS,
        "endpoints": ["/health", "/v1/audio/speech", "/models/{model:path}"],
    }


@app.get("/health")
def health() -> dict[str, Any]:
    return {
        "status": "ok",
        "backend": os.getenv("LOCAL_TTS_BACKEND", "espeak"),
        "command": os.getenv("LOCAL_TTS_COMMAND", "espeak"),
        "default_model": normalize_model_name(os.getenv("LOCAL_TTS_DEFAULT_MODEL", "KittenML/kitten-tts-mini-0.8")),
        "kitten_available_voices": KITTEN_VOICES,
        "edge_available_voices": EDGE_TTS_VOICES,
        "supported_models": SUPPORTED_MODELS,
        "kitten_runtime_available": KITTEN_RUNTIME is not None,
        "kitten_runtime_error": KITTEN_IMPORT_ERROR,
        "edge_runtime_available": edge_tts is not None,
        "edge_runtime_error": EDGE_TTS_IMPORT_ERROR,
    }


@app.post("/v1/audio/speech")
def openai_compatible_tts(payload: OpenAITTSRequest) -> Response:
    model_name = normalize_model_name(payload.model or os.getenv("LOCAL_TTS_DEFAULT_MODEL", "KittenML/kitten-tts-mini-0.8"))
    audio, engine = synthesize_to_wav(
        text=first_non_empty(payload.input, payload.text),
        language=first_non_empty(payload.language, "en"),
        voice=first_non_empty(payload.voice),
        speed=payload.speed or 1.0,
        model_name=model_name,
    )
    return Response(content=audio, media_type="audio/wav", headers={"X-TTS-Model": model_name, "X-TTS-Engine": engine})


@app.post("/models/{model:path}")
def hf_compatible_tts(model: str, payload: dict[str, Any]):
    model_name = normalize_model_name(model)
    text = payload.get("inputs") or payload.get("input") or payload.get("text")
    if isinstance(text, list):
        text = " ".join(str(item) for item in text)

    parameters = payload.get("parameters") or {}
    voice = first_non_empty(parameters.get("voice"), parameters.get("speaker"), payload.get("voice"))
    language = first_non_empty(parameters.get("language"), payload.get("language"), "en")
    speed = parameters.get("speed", payload.get("speed", 1.0))

    audio, engine = synthesize_to_wav(
        text=str(text or ""),
        language=language,
        voice=voice,
        speed=float(speed or 1.0),
        model_name=model_name,
    )

    if payload.get("return_base64") is True:
        import base64

        return JSONResponse({
            "model": model_name,
            "engine": engine,
            "audio_base64": base64.b64encode(audio).decode("ascii"),
            "format": "wav",
        })

    return Response(content=audio, media_type="audio/wav", headers={"X-TTS-Model": model_name, "X-TTS-Engine": engine})
