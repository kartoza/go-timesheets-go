package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// NNTrainer manages training the similarity model from timesheet history.
// Adapted from kartoza-pg-ai nn.QueryTrainer.
type NNTrainer struct {
	model     *SimilarityModel
	tokenizer *Tokenizer
	modelPath string
	tokPath   string
	mu        sync.RWMutex
	minPairs  int // Minimum pairs required for training
}

// NewNNTrainer creates a new trainer
func NewNNTrainer() (*NNTrainer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	modelDir := filepath.Join(home, ".local", "share", "kartoza-timesheet", "nn_model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return nil, err
	}

	tokPath := filepath.Join(modelDir, "tokenizer.json")
	modelPath := filepath.Join(modelDir, "model.json")

	tokenizer := NewTokenizer()
	model := NewSimilarityModel(tokenizer)

	trainer := &NNTrainer{
		model:     model,
		tokenizer: tokenizer,
		modelPath: modelPath,
		tokPath:   tokPath,
		minPairs:  5,
	}

	// Try to load existing model
	trainer.loadIfExists()

	return trainer, nil
}

// loadIfExists attempts to load an existing saved model
func (t *NNTrainer) loadIfExists() {
	// Load tokenizer
	if tokData, err := os.ReadFile(t.tokPath); err == nil {
		var tokExport map[string]interface{}
		if err := json.Unmarshal(tokData, &tokExport); err == nil {
			t.tokenizer.Import(tokExport)
		}
	}

	// Load model data (document vectors)
	if modelData, err := os.ReadFile(t.modelPath); err == nil {
		var saved savedModel
		if err := json.Unmarshal(modelData, &saved); err == nil {
			t.importModel(saved)
		}
	}
}

// savedModel is the JSON-serializable model state
type savedModel struct {
	Documents  []savedDocument    `json:"documents"`
	IDFWeights map[string]float64 `json:"idf_weights"` // string keys for JSON
	Trained    bool               `json:"trained"`
}

type savedDocument struct {
	Match  TimesheetMatch     `json:"match"`
	Text   string             `json:"text"`
	Vector map[string]float64 `json:"vector"` // string keys for JSON
}

func (t *NNTrainer) importModel(saved savedModel) {
	t.model.trained = saved.Trained
	t.model.idfWeights = make(map[int]float64)
	for k, v := range saved.IDFWeights {
		if idx, ok := t.tokenizer.wordToIdx[k]; ok {
			t.model.idfWeights[idx] = v
		}
	}

	t.model.documents = nil
	for _, sd := range saved.Documents {
		vec := make(map[int]float64)
		for k, v := range sd.Vector {
			if idx, ok := t.tokenizer.wordToIdx[k]; ok {
				vec[idx] = v
			}
		}
		t.model.documents = append(t.model.documents, documentVector{
			Match:  sd.Match,
			Text:   sd.Text,
			Vector: vec,
		})
	}
}

// Train trains the model from timesheet entries
func (t *NNTrainer) Train(entries []EntryInfo) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Build training pairs from entries
	pairs := t.buildTrainingPairs(entries)
	if len(pairs) < t.minPairs {
		return fmt.Errorf("insufficient training data: have %d pairs, need at least %d", len(pairs), t.minPairs)
	}

	// Train the model
	t.model.Train(pairs)

	// Save
	return t.save()
}

// TrainAsync trains the model asynchronously
func (t *NNTrainer) TrainAsync(entries []EntryInfo, callback func(error)) {
	go func() {
		err := t.Train(entries)
		if callback != nil {
			callback(err)
		}
	}()
}

// buildTrainingPairs converts timesheet entries into training pairs.
// Each unique project/task/activity combination becomes a training document,
// with the description and combination name as text features.
func (t *NNTrainer) buildTrainingPairs(entries []EntryInfo) []TrainingPair {
	// Deduplicate by project+task+activity
	type key struct {
		project, task, activity string
	}
	seen := make(map[key]bool)
	var pairs []TrainingPair

	for _, e := range entries {
		k := key{e.ProjectName, e.TaskName, e.ActivityName}
		if seen[k] {
			continue
		}
		seen[k] = true

		label := e.ProjectName
		if e.TaskName != "" {
			label += " " + e.TaskName
		}
		if e.ActivityName != "" {
			label += " " + e.ActivityName
		}

		// Use description as the query if available
		query := label
		if e.Description != "" {
			query = e.Description
		}

		pairs = append(pairs, TrainingPair{
			Query: query,
			Label: label,
			Match: TimesheetMatch{
				ProjectName:  e.ProjectName,
				TaskName:     e.TaskName,
				ActivityName: e.ActivityName,
			},
		})
	}

	return pairs
}

// Predict finds matches using the trained model
func (t *NNTrainer) Predict(query string, topK int) ([]TimesheetMatch, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.model.IsTrained() {
		return nil, fmt.Errorf("model not trained")
	}

	matches := t.model.Predict(query, topK)
	return matches, nil
}

// IsTrained returns whether the model has been trained
func (t *NNTrainer) IsTrained() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.model.IsTrained()
}

// save persists the model and tokenizer to disk
func (t *NNTrainer) save() error {
	// Save tokenizer
	tokExport := t.tokenizer.Export()
	tokData, err := json.MarshalIndent(tokExport, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokenizer: %w", err)
	}
	if err := os.WriteFile(t.tokPath, tokData, 0644); err != nil {
		return fmt.Errorf("failed to write tokenizer: %w", err)
	}

	// Save model
	saved := t.exportModel()
	modelData, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal model: %w", err)
	}
	if err := os.WriteFile(t.modelPath, modelData, 0644); err != nil {
		return fmt.Errorf("failed to write model: %w", err)
	}

	return nil
}

func (t *NNTrainer) exportModel() savedModel {
	// Convert int keys to string keys for JSON serialization
	idfWeights := make(map[string]float64)
	for idx, w := range t.model.idfWeights {
		if word, ok := t.tokenizer.idxToWord[idx]; ok {
			idfWeights[word] = w
		}
	}

	var docs []savedDocument
	for _, d := range t.model.documents {
		vec := make(map[string]float64)
		for idx, v := range d.Vector {
			if word, ok := t.tokenizer.idxToWord[idx]; ok {
				vec[word] = v
			}
		}
		docs = append(docs, savedDocument{
			Match:  d.Match,
			Text:   d.Text,
			Vector: vec,
		})
	}

	return savedModel{
		Documents:  docs,
		IDFWeights: idfWeights,
		Trained:    t.model.trained,
	}
}

// GetStats returns statistics about the trainer
func (t *NNTrainer) GetStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"trained":    t.model.IsTrained(),
		"vocab_size": t.tokenizer.VocabSize(),
		"documents":  len(t.model.documents),
		"min_pairs":  t.minPairs,
	}
}
