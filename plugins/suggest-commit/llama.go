//go:build llama && cgo

package main

/*
#cgo pkg-config: llama
#include <llama.h>
#include <stdlib.h>

static void ignore_log(enum ggml_log_level level, const char *text, void *data) {}

static void setup_backend(void) {
	llama_log_set(ignore_log, NULL);
	llama_backend_init();
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

const (
	maxContextTokens       = 4096
	outputTokensPerSubject = 48
)

func generate(modelPath, prompt string, maxOutputTokens int) (string, error) {
	C.setup_backend()
	defer C.llama_backend_free()

	path := C.CString(modelPath)
	defer C.free(unsafe.Pointer(path))
	modelParams := C.llama_model_default_params()
	modelParams.n_gpu_layers = 0
	modelParams.use_mmap = true
	model := C.llama_model_load_from_file(path, modelParams)
	if model == nil {
		return "", fmt.Errorf("could not load GGUF model %q", modelPath)
	}
	defer C.llama_model_free(model)
	vocab := C.llama_model_get_vocab(model)

	input := C.CString(prompt)
	defer C.free(unsafe.Pointer(input))
	tokens := make([]C.llama_token, maxContextTokens-maxOutputTokens)
	count := C.llama_tokenize(vocab, input, C.int32_t(len(prompt)), &tokens[0], C.int32_t(len(tokens)), true, true)
	if count <= 0 {
		return "", fmt.Errorf("could not fit the diff in the model context")
	}

	contextParams := C.llama_context_default_params()
	contextParams.n_ctx = C.uint32_t(int(count) + maxOutputTokens)
	contextParams.n_batch = C.uint32_t(count)
	contextParams.n_threads = C.int32_t(min(4, runtime.GOMAXPROCS(0)))
	contextParams.n_threads_batch = contextParams.n_threads
	ctx := C.llama_init_from_model(model, contextParams)
	if ctx == nil {
		return "", fmt.Errorf("could not initialize the model context")
	}
	defer C.llama_free(ctx)
	sampler := C.llama_sampler_init_greedy()
	defer C.llama_sampler_free(sampler)

	if rc := C.llama_decode(ctx, C.llama_batch_get_one(&tokens[0], count)); rc != 0 {
		return "", fmt.Errorf("model could not read the diff (code %d)", rc)
	}
	var draft strings.Builder
	for i := range maxOutputTokens {
		token := C.llama_sampler_sample(sampler, ctx, -1)
		if C.llama_vocab_is_eog(vocab, token) {
			break
		}

		var piece [256]C.char
		size := C.llama_token_to_piece(vocab, token, &piece[0], C.int32_t(len(piece)), 0, false)
		if size < 0 {
			return "", fmt.Errorf("model returned an oversized token")
		}
		draft.WriteString(C.GoStringN(&piece[0], C.int(size)))
		if i == maxOutputTokens-1 {
			break
		}
		if rc := C.llama_decode(ctx, C.llama_batch_get_one(&token, 1)); rc != 0 {
			return "", fmt.Errorf("model could not generate the subject (code %d)", rc)
		}
	}
	return draft.String(), nil
}
