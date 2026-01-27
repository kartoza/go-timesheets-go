package ai

import (
	"strings"
	"unicode"
)

// Tokenizer handles text tokenization for the neural network.
// Adapted from kartoza-pg-ai nn.Tokenizer but simplified for timesheet queries.
type Tokenizer struct {
	wordToIdx     map[string]int
	idxToWord     map[int]string
	vocabSize     int
	specialTokens map[string]int
}

// Special tokens
const (
	PadToken = "<PAD>"
	UnkToken = "<UNK>"
)

// NewTokenizer creates a new tokenizer
func NewTokenizer() *Tokenizer {
	t := &Tokenizer{
		wordToIdx: make(map[string]int),
		idxToWord: make(map[int]string),
		specialTokens: map[string]int{
			PadToken: 0,
			UnkToken: 1,
		},
	}

	for token, idx := range t.specialTokens {
		t.wordToIdx[token] = idx
		t.idxToWord[idx] = token
	}
	t.vocabSize = len(t.specialTokens)

	return t
}

// Tokenize splits text into lowercase word tokens
func (t *Tokenizer) Tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// BuildVocabulary builds vocabulary from a set of texts
func (t *Tokenizer) BuildVocabulary(texts []string) {
	wordFreq := make(map[string]int)

	for _, text := range texts {
		tokens := t.Tokenize(text)
		for _, token := range tokens {
			wordFreq[token]++
		}
	}

	for word := range wordFreq {
		if _, exists := t.wordToIdx[word]; !exists {
			idx := t.vocabSize
			t.wordToIdx[word] = idx
			t.idxToWord[idx] = word
			t.vocabSize++
		}
	}
}

// Encode converts text to a bag-of-words vector (term frequencies)
func (t *Tokenizer) Encode(text string) map[int]float64 {
	tokens := t.Tokenize(text)
	vec := make(map[int]float64)

	for _, token := range tokens {
		if idx, ok := t.wordToIdx[token]; ok {
			vec[idx]++
		} else {
			vec[t.specialTokens[UnkToken]]++
		}
	}

	return vec
}

// VocabSize returns the vocabulary size
func (t *Tokenizer) VocabSize() int {
	return t.vocabSize
}

// Export exports the tokenizer vocabulary for persistence
func (t *Tokenizer) Export() map[string]interface{} {
	return map[string]interface{}{
		"word_to_idx": t.wordToIdx,
		"vocab_size":  t.vocabSize,
	}
}

// Import imports a tokenizer vocabulary from saved data
func (t *Tokenizer) Import(data map[string]interface{}) error {
	if wordToIdx, ok := data["word_to_idx"].(map[string]interface{}); ok {
		t.wordToIdx = make(map[string]int)
		t.idxToWord = make(map[int]string)
		for word, idxVal := range wordToIdx {
			if idx, ok := idxVal.(float64); ok {
				t.wordToIdx[word] = int(idx)
				t.idxToWord[int(idx)] = word
			}
		}
	}

	if vocabSize, ok := data["vocab_size"].(float64); ok {
		t.vocabSize = int(vocabSize)
	}

	return nil
}
