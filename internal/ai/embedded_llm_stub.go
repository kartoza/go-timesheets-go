//go:build !llamacpp

package ai

import "fmt"

// EmbeddedLLM is a no-op stub when the "llamacpp" build tag is not set.
// Build with -tags llamacpp to enable embedded LLM inference.
type EmbeddedLLM struct{}

// EmbeddedLLMConfig holds configuration for the embedded LLM
type EmbeddedLLMConfig struct {
	ModelPath   string
	Threads     int
	ContextSize int
	Temperature float64
	TopP        float64
	TopK        int
	MaxTokens   int
}

// DefaultEmbeddedLLMConfig returns default configuration
func DefaultEmbeddedLLMConfig() EmbeddedLLMConfig {
	return EmbeddedLLMConfig{}
}

// NewEmbeddedLLM returns a stub (build with -tags llamacpp for real implementation)
func NewEmbeddedLLM(_ EmbeddedLLMConfig) (*EmbeddedLLM, error) {
	return &EmbeddedLLM{}, nil
}

// LoadModel is a no-op stub
func (e *EmbeddedLLM) LoadModel(_ string) error {
	return fmt.Errorf("embedded LLM not available (build with -tags llamacpp)")
}

// IsLoaded always returns false in the stub
func (e *EmbeddedLLM) IsLoaded() bool {
	return false
}

// Generate is a no-op stub
func (e *EmbeddedLLM) Generate(_ string) (string, error) {
	return "", fmt.Errorf("embedded LLM not available (build with -tags llamacpp)")
}

// Close is a no-op stub
func (e *EmbeddedLLM) Close() {}

// FindDefaultModel always returns empty in the stub
func FindDefaultModel() string {
	return ""
}

// EmbeddedLLMAvailable returns false when the embedded LLM is not compiled in
func EmbeddedLLMAvailable() bool {
	return false
}
