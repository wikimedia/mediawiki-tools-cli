package liftwing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

const indexFixture = `openapi: "3.0.0"
paths:
  /service/lw/inference/v1/models/outlink-topic-model:predict:
    $ref: ./outlink-topic-model.yaml#/paths/~1service~1lw~1inference~1v1~1models~1outlink-topic-model:predict
  /service/lw/inference/v1/models/{wiki}-damaging:predict:
    $ref: ./ores/damaging.yaml#/paths/~1service~1lw~1inference~1v1~1models~1{wiki}-damaging:predict
  /service/lw/inference/v1/models/{model}/openai/v1/chat/completions:
    $ref: ./llm-chat-completions.yaml
  /service/lw/recommendation/api/v1/translation:
    $ref: ./translation.yaml
`

const topicFixture = `paths:
  /service/lw/inference/v1/models/outlink-topic-model:predict:
    post:
      summary: Predict article topics
      description: See [model card](https://meta.wikimedia.org/wiki/Machine_learning_models).
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                page_title:
                  type: string
            examples:
              by-title:
                value:
                  page_title: Albert Einstein
                  lang: en
      responses:
        "200":
          description: Prediction
`

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func mock(t *testing.T, handler func(*http.Request) (*http.Response, error)) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = transport(func(req *http.Request) (*http.Response, error) {
		if req.UserAgent() != cli.HTTPUserAgent() || req.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected request headers: %v", req.Header)
		}
		return handler(req)
	})
	t.Cleanup(func() { http.DefaultTransport = old })
}

func reply(req *http.Request, body string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func execute(input string, args ...string) (string, error) {
	cmd := NewCmd()
	cmd.SilenceUsage = true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestDiscoverAndInfo(t *testing.T) {
	requests := 0
	mock(t, func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.String() {
		case specURL:
			return reply(req, indexFixture)
		case baseURL + "/service/lw/specs/outlink-topic-model.yaml":
			return reply(req, topicFixture)
		default:
			t.Fatalf("unexpected URL %s", req.URL)
			return nil, nil
		}
	})
	out, err := execute("", "models", "list", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var models []model
	if err := json.Unmarshal([]byte(out), &models); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(models) != 2 || models[0].Name != "outlink-topic-model" || models[1].Name != "{wiki}-damaging" {
		t.Fatalf("bad model discovery: %s (%d requests)", out, requests)
	}
	out, err = execute("", "models", "info", "outlink-topic-model")
	if err != nil {
		t.Fatal(err)
	}
	var info model
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatal(err)
	}
	if requests != 3 || info.Operation["summary"] != "Predict article topics" || !strings.Contains(out, "Albert Einstein") || !strings.Contains(out, "model card") {
		t.Fatalf("missing model details: %s", out)
	}
	if strings.Contains(out, `"path"`) || strings.Contains(out, `"route"`) {
		t.Fatal("internal fields exposed", out)
	}
}

func TestPredictionFileAndStdinPreserveNumbers(t *testing.T) {
	const input = `{"rev_id":9007199254740993}`
	file := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(file, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	predictions := 0
	mock(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == specURL {
			return reply(req, indexFixture)
		}
		if req.URL.String() != baseURL+modelPrefix+"enwiki-damaging:predict" || req.Method != http.MethodPost || req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("bad prediction request %s %s", req.Method, req.URL)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil || string(body) != input {
			t.Fatalf("changed input: %s, %v", body, err)
		}
		predictions++
		return reply(req, `{"revision":9007199254740993,"probability":0.125}`)
	})
	for _, args := range [][]string{{"predict", "enwiki-damaging"}, {"predict", "enwiki-damaging", "--file", file}, {"predict", "enwiki-damaging", "--output", "jq", "--format", ".revision"}} {
		out, err := execute(input, args...)
		if err != nil || !strings.Contains(out, "9007199254740993") {
			t.Fatalf("prediction failed: %s, %v", out, err)
		}
	}
	if predictions != 3 {
		t.Fatal("wrong prediction count", predictions)
	}
}

func TestValidationBeforeNetwork(t *testing.T) {
	mock(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request %s", req.URL)
		return nil, nil
	})
	for _, tc := range []struct {
		input string
		args  []string
	}{
		{`[]`, []string{"predict", "outlink-topic-model"}},
		{`null`, []string{"predict", "outlink-topic-model"}},
		{`{} {}`, []string{"predict", "outlink-topic-model"}},
		{`{`, []string{"predict", "outlink-topic-model"}},
		{`{}`, []string{"predict", "../bad"}},
		{`{}`, []string{"predict", "outlink-topic-model", "--output", "table"}},
		{`{}`, []string{"predict", "outlink-topic-model", "--format", ".["}},
		{strings.Repeat(" ", maxInput+1), []string{"predict", "outlink-topic-model"}},
	} {
		if _, err := execute(tc.input, tc.args...); err == nil {
			t.Fatalf("accepted invalid input: %v", tc.args)
		}
	}
}

func TestUnknownModelAndUnsafeReference(t *testing.T) {
	mock(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != specURL {
			t.Fatalf("followed unsafe reference %s", req.URL)
		}
		return reply(req, strings.ReplaceAll(indexFixture, "./outlink-topic-model.yaml#", "https://example.org/model.yaml#"))
	})
	if _, err := execute(`{}`, "predict", "unknown-model"); err == nil || !strings.Contains(err.Error(), "no documented") {
		t.Fatal("unknown model accepted", err)
	}
	if _, err := execute("", "models", "info", "outlink-topic-model"); err == nil || !strings.Contains(err.Error(), "outside Lift Wing") {
		t.Fatal("external reference accepted", err)
	}
}

func TestHTTPFailuresAndCancellation(t *testing.T) {
	mock(t, func(req *http.Request) (*http.Response, error) {
		resp, _ := reply(req, `{"detail":"rate limited"}`)
		resp.StatusCode = http.StatusTooManyRequests
		resp.Header.Set("Retry-After", "60")
		return resp, nil
	})
	if _, err := execute("", "models", "list"); err == nil || !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "Retry-After: 60") {
		t.Fatal("rate limit details lost", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mock(t, func(req *http.Request) (*http.Response, error) {
		if req.Context().Err() != context.Canceled {
			t.Fatal("request lost command context")
		}
		return nil, req.Context().Err()
	})
	if _, err := discover(ctx); err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatal("cancellation lost", err)
	}
}

func TestPredictionRejectsMalformedResponse(t *testing.T) {
	for _, body := range []string{"not JSON", `{} {}`, `{} trailing`} {
		t.Run(body, func(t *testing.T) {
			mock(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == specURL {
					return reply(req, indexFixture)
				}
				return reply(req, body)
			})
			out, err := execute(`{}`, "predict", "outlink-topic-model")
			if err == nil || out != "" {
				t.Fatalf("invalid prediction produced output: %q, %v", out, err)
			}
		})
	}
}
