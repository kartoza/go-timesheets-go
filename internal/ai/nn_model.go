package ai

import (
	"math"
)

// SimilarityModel provides TF-IDF cosine similarity matching.
// This is more suitable for matching timesheet queries to project/task/activity
// names than the Seq2Seq model used in kartoza-pg-ai (which was designed for SQL generation).
// It follows the same pattern: tokenizer + model + trainer.
type SimilarityModel struct {
	tokenizer  *Tokenizer
	documents  []documentVector // Known project/task/activity combinations
	idfWeights map[int]float64  // Inverse document frequency weights
	trained    bool
}

// documentVector holds a TF-IDF vector for a known combination
type documentVector struct {
	Match  TimesheetMatch  // The project/task/activity this represents
	Text   string          // Original text (project + task + activity names)
	Vector map[int]float64 // TF-IDF vector
}

// NewSimilarityModel creates a new similarity model
func NewSimilarityModel(tokenizer *Tokenizer) *SimilarityModel {
	return &SimilarityModel{
		tokenizer:  tokenizer,
		idfWeights: make(map[int]float64),
	}
}

// Train builds TF-IDF vectors from training pairs
func (m *SimilarityModel) Train(pairs []TrainingPair) {
	if len(pairs) == 0 {
		return
	}

	// Build vocabulary from all texts
	var allTexts []string
	for _, p := range pairs {
		allTexts = append(allTexts, p.Query)
		allTexts = append(allTexts, p.Label)
	}
	m.tokenizer.BuildVocabulary(allTexts)

	// Compute document frequencies for IDF
	docFreq := make(map[int]int)
	totalDocs := len(pairs)

	for _, p := range pairs {
		seen := make(map[int]bool)
		tokens := m.tokenizer.Tokenize(p.Label)
		for _, token := range tokens {
			if idx, ok := m.tokenizer.wordToIdx[token]; ok && !seen[idx] {
				docFreq[idx]++
				seen[idx] = true
			}
		}
	}

	// Compute IDF weights: log(N / (1 + df))
	m.idfWeights = make(map[int]float64)
	for idx, df := range docFreq {
		m.idfWeights[idx] = math.Log(float64(totalDocs) / float64(1+df))
	}

	// Build TF-IDF vectors for each document
	m.documents = nil
	for _, p := range pairs {
		tf := m.tokenizer.Encode(p.Label)
		tfidf := make(map[int]float64)
		for idx, freq := range tf {
			idf := m.idfWeights[idx]
			if idf == 0 {
				idf = 1.0 // Default IDF for unknown terms
			}
			tfidf[idx] = freq * idf
		}

		m.documents = append(m.documents, documentVector{
			Match:  p.Match,
			Text:   p.Label,
			Vector: tfidf,
		})
	}

	m.trained = true
}

// Predict finds the closest matches for a query
func (m *SimilarityModel) Predict(query string, topK int) []TimesheetMatch {
	if !m.trained || len(m.documents) == 0 {
		return nil
	}

	// Encode query as TF-IDF vector
	tf := m.tokenizer.Encode(query)
	queryVec := make(map[int]float64)
	for idx, freq := range tf {
		idf := m.idfWeights[idx]
		if idf == 0 {
			idf = 1.0
		}
		queryVec[idx] = freq * idf
	}

	// Compute cosine similarity with each document
	type scored struct {
		match TimesheetMatch
		score float64
	}
	var results []scored

	for _, doc := range m.documents {
		sim := cosineSimilarity(queryVec, doc.Vector)
		if sim > 0.1 { // Minimum threshold
			match := doc.Match
			match.Confidence = sim
			match.Reason = "Neural network match"
			results = append(results, scored{match, sim})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return top K
	if len(results) > topK {
		results = results[:topK]
	}

	var matches []TimesheetMatch
	for _, r := range results {
		matches = append(matches, r.match)
	}
	return matches
}

// IsTrained returns whether the model has been trained
func (m *SimilarityModel) IsTrained() bool {
	return m.trained
}

// TrainingPair represents a query-to-match pair for training
type TrainingPair struct {
	Query string         // Natural language description
	Label string         // Combined project/task/activity text
	Match TimesheetMatch // The resolved match
}

// --- Math utilities ---

func cosineSimilarity(a, b map[int]float64) float64 {
	dotProduct := 0.0
	for idx, va := range a {
		if vb, ok := b[idx]; ok {
			dotProduct += va * vb
		}
	}

	magA := vectorMagnitude(a)
	magB := vectorMagnitude(b)

	if magA == 0 || magB == 0 {
		return 0
	}

	return dotProduct / (magA * magB)
}

func vectorMagnitude(v map[int]float64) float64 {
	sum := 0.0
	for _, val := range v {
		sum += val * val
	}
	return math.Sqrt(sum)
}
