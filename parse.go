package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const maxBody = 64 << 10 // 64 KiB of pasted text is plenty for a sheet

// systemPrompt pins the model to the gm-character/v1 contract (see FORMAT.md).
const systemPrompt = `Ты — парсер листов персонажей D&D 5e. На вход — сырой текст персонажа в любом виде
(экспорт D&D Beyond, OCR со скрина, рукописный список, короткое описание). Верни ТОЛЬКО валидный
JSON объекта "gm-character/v1" — без markdown-заборов, без пояснений, без текста вокруг.

Форма (все ключи латиницей ровно как здесь; значения-подписи по-русски):
{
  "schema": "gm-character/v1",
  "name": string, "player": string|null, "class": string|null, "subclass": string|null,
  "level": int(1..20), "race": string|null, "background": string|null, "alignment": string|null,
  "ac": int, "hp": { "max": int, "current": int, "temp": int },
  "speed": string, "initiative": int, "prof": int,
  "abilities": { "str": int, "dex": int, "con": int, "int": int, "wis": int, "cha": int },
  "saves": [ ключи из str/dex/con/int/wis/cha, где есть владение ],
  "skills": "Навык +N, …" | "—",
  "passives": { "perception": int, ... },
  "senses": string|"—", "languages": string|"—",
  "attacks": [ { "name": string, "hit": int|null, "damage": "NdM+K"|"", "type": string, "note"?: string, "extra"?: "NdM" } ],
  "spellcasting": { "ability": string, "dc": int, "attack": int, "slots": {"1":int,...}, "spells": [ {"name":string,"level":int,"desc":string} ] } | null,
  "features": [string], "gear": [string], "conditions": [], "notes": string
}

Правила:
- Характеристики (abilities) — это ЗНАЧЕНИЯ (8..20+), не модификаторы. initiative/saves/навыки/hit — модификаторы (числа со знаком, но в JSON просто int).
- hit — число (бонус к попаданию) или null для атак-эффектов без броска. damage — строка кубов или "".
- prof по уровню, если не указан явно: 1-4=2, 5-8=3, 9-12=4, 13-16=5, 17-20=6.
- hp.current = hp.max, hp.temp = 0, если не указано иное.
- Чего нет в исходнике — не выдумывай: строки → "—", массивы → [], spellcasting → null, неизвестные числа → 0 и короткая пометка в notes.
- conditions почти всегда []. Никаких лишних ключей (additionalProperties запрещены).`

var (
	clientOnce sync.Once
	client     anthropic.Client
	clientErr  error
)

func getClient() (anthropic.Client, error) {
	clientOnce.Do(func() {
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			clientErr = errors.New("ANTHROPIC_API_KEY is not set")
			return
		}
		client = anthropic.NewClient() // reads ANTHROPIC_API_KEY from env
	})
	return client, clientErr
}

func parseModel() anthropic.Model {
	if v := os.Getenv("PARSE_MODEL"); v != "" {
		return anthropic.Model(v)
	}
	// String id (not an SDK constant) so this doesn't break when the pinned
	// SDK predates a model release. Override with PARSE_MODEL.
	return anthropic.Model("claude-opus-4-8")
}

type parseRequest struct {
	Text string `json:"text"`
}

func handleParseCharacter(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json body: "+err.Error())
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		writeErr(w, http.StatusBadRequest, "empty text")
		return
	}

	cl, err := getClient()
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 80*time.Second)
	defer cancel()

	raw, err := callClaude(ctx, cl, req.Text)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "model call failed: "+err.Error())
		return
	}

	obj, err := extractJSON(raw)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "model did not return valid character json: "+err.Error())
		return
	}
	normalize(obj)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(obj)
}

func callClaude(ctx context.Context, cl anthropic.Client, text string) (string, error) {
	resp, err := cl.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     parseModel(),
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Текст персонажа:\n\n" + text)),
		},
	})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	return sb.String(), nil
}

// extractJSON pulls the first balanced {...} object out of the model text
// (tolerates stray prose or code fences) and unmarshals it.
func extractJSON(s string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return nil, errors.New("no json object found")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s[start:end+1]), &obj); err != nil {
		return nil, err
	}
	if _, ok := obj["name"]; !ok {
		return nil, errors.New("missing required field: name")
	}
	if _, ok := obj["abilities"]; !ok {
		return nil, errors.New("missing required field: abilities")
	}
	return obj, nil
}

// normalize fills a few safe defaults so the client can render without guards.
func normalize(obj map[string]any) {
	obj["schema"] = "gm-character/v1"
	if hp, ok := obj["hp"].(map[string]any); ok {
		if _, has := hp["current"]; !has {
			hp["current"] = hp["max"]
		}
		if _, has := hp["temp"]; !has {
			hp["temp"] = 0
		}
	}
	if _, ok := obj["conditions"]; !ok {
		obj["conditions"] = []any{}
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

var _ = io.Discard // reserved for future streaming
