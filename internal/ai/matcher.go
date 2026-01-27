package ai

import (
	"regexp"
	"sort"
	"strings"
)

// FuzzyMatcher provides rule-based fuzzy matching to find project/task/activity combinations
// from natural language queries. Adapted from the SemanticMatcher in kartoza-pg-ai.
type FuzzyMatcher struct{}

// NewFuzzyMatcher creates a new fuzzy matcher
func NewFuzzyMatcher() *FuzzyMatcher {
	return &FuzzyMatcher{}
}

// Match finds project/task/activity combinations that match the query
func (m *FuzzyMatcher) Match(query string, ctx *TimesheetContext) []TimesheetMatch {
	if ctx == nil || len(ctx.Projects) == 0 {
		return nil
	}

	query = strings.ToLower(strings.TrimSpace(query))

	// Extract meaningful keywords from the query
	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		return nil
	}

	// Score each project/task/activity combination
	var matches []TimesheetMatch

	for _, project := range ctx.Projects {
		projectScore := bestKeywordScore(keywords, project.Name)

		// If no tasks, score just by project + best activity
		if len(project.Tasks) == 0 {
			if projectScore >= 0.3 {
				// Find best activity match
				bestAct, actScore := bestActivityMatch(keywords, ctx.Activities)
				combinedScore := projectScore
				if actScore > 0 {
					combinedScore = (projectScore + actScore) / 2
				}
				match := TimesheetMatch{
					ProjectID:   project.ID,
					ProjectName: project.Name,
					Confidence:  combinedScore,
					Reason:      "Matched project name",
				}
				if bestAct != nil {
					match.ActivityID = bestAct.ID
					match.ActivityName = bestAct.Name
					match.Reason = "Matched project and activity"
				}
				matches = append(matches, match)
			}
			continue
		}

		for _, task := range project.Tasks {
			taskScore := bestKeywordScore(keywords, task.Name)

			// Combine project and task scores
			combinedScore := 0.0
			reason := ""
			if projectScore > 0 && taskScore > 0 {
				combinedScore = (projectScore*0.6 + taskScore*0.4)
				reason = "Matched project and task"
			} else if projectScore > 0 {
				combinedScore = projectScore * 0.7
				reason = "Matched project name"
			} else if taskScore > 0 {
				combinedScore = taskScore * 0.7
				reason = "Matched task name"
			}

			if combinedScore < 0.25 {
				continue
			}

			// Find best activity match
			bestAct, actScore := bestActivityMatch(keywords, ctx.Activities)
			if actScore > 0 {
				combinedScore = (combinedScore + actScore*0.3) / 1.3
				reason += " and activity"
			}

			match := TimesheetMatch{
				ProjectID:   project.ID,
				ProjectName: project.Name,
				TaskID:      task.ID,
				TaskName:    task.Name,
				Confidence:  combinedScore,
				Reason:      reason,
			}
			if bestAct != nil {
				match.ActivityID = bestAct.ID
				match.ActivityName = bestAct.Name
			}
			matches = append(matches, match)
		}
	}

	// Sort by confidence descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Confidence > matches[j].Confidence
	})

	// Limit to top 10
	if len(matches) > 10 {
		matches = matches[:10]
	}

	return matches
}

func bestActivityMatch(keywords []string, activities []ActivityInfo) (*ActivityInfo, float64) {
	var best *ActivityInfo
	bestScore := 0.0
	for i, act := range activities {
		score := bestKeywordScore(keywords, act.Name)
		if score > bestScore {
			bestScore = score
			best = &activities[i]
		}
	}
	return best, bestScore
}

// bestKeywordScore returns the best match score between any keyword and the target name
func bestKeywordScore(keywords []string, name string) float64 {
	nameLower := strings.ToLower(name)
	nameWords := splitEntityName(nameLower)
	bestScore := 0.0

	for _, kw := range keywords {
		// Exact match with full name
		if kw == nameLower {
			return 1.0
		}

		// Exact match with any word in the name
		for _, word := range nameWords {
			if kw == word {
				score := 0.95
				if score > bestScore {
					bestScore = score
				}
			}
		}

		// Contains match
		if strings.Contains(nameLower, kw) {
			score := 0.85 * (float64(len(kw)) / float64(len(nameLower)))
			if score+0.1 > bestScore {
				bestScore = score + 0.1
			}
		}
		if strings.Contains(kw, nameLower) && len(nameLower) >= 3 {
			score := 0.8 * (float64(len(nameLower)) / float64(len(kw)))
			if score+0.1 > bestScore {
				bestScore = score + 0.1
			}
		}

		// Prefix match
		if strings.HasPrefix(nameLower, kw) || strings.HasPrefix(kw, nameLower) {
			minLen := len(kw)
			if len(nameLower) < minLen {
				minLen = len(nameLower)
			}
			score := 0.75 * (float64(minLen) / float64(len(nameLower)+len(kw)-minLen))
			if score+0.15 > bestScore {
				bestScore = score + 0.15
			}
		}

		// Stem matching
		kwStem := stemWord(kw)
		for _, word := range nameWords {
			wordStem := stemWord(word)
			if kwStem == wordStem && len(kwStem) >= 3 {
				if 0.85 > bestScore {
					bestScore = 0.85
				}
			}
			if strings.HasPrefix(wordStem, kwStem) || strings.HasPrefix(kwStem, wordStem) {
				minLen := len(kwStem)
				if len(wordStem) < minLen {
					minLen = len(wordStem)
				}
				if minLen >= 3 {
					score := 0.7 * (float64(minLen) / float64(len(kwStem)))
					if score > bestScore {
						bestScore = score
					}
				}
			}
		}

		// N-gram similarity (trigrams)
		ngramScore := ngramSimilarity(kw, nameLower, 3)
		if ngramScore > 0.3 && ngramScore*0.8 > bestScore {
			bestScore = ngramScore * 0.8
		}

		// Levenshtein distance for fuzzy match
		lenDiff := len(kw) - len(nameLower)
		if lenDiff < 0 {
			lenDiff = -lenDiff
		}
		if lenDiff <= 3 && len(kw) >= 4 {
			distance := levenshteinDistance(kw, nameLower)
			maxLen := len(kw)
			if len(nameLower) > maxLen {
				maxLen = len(nameLower)
			}
			if distance <= 2 {
				score := (1.0 - float64(distance)/float64(maxLen)) * 0.7
				if score > bestScore {
					bestScore = score
				}
			}
		}
	}

	return bestScore
}

// extractKeywords extracts meaningful words from a query, filtering out stop words
func extractKeywords(query string) []string {
	// Try to extract the meaningful part with regex patterns
	extractPatterns := []string{
		`(?:which|what) project (?:should i|do i|can i) (?:log|use|pick|choose) (.+?)(?:\s+(?:activity|task))?\s*(?:to|for|on|$)`,
		`(?:log|start|track|record) (.+?)(?:\s+to|\s+for|\s+on|\s*$)`,
		`(?:find|search|look for) (.+?)(?:\s+project|\s+task|\s+activity|\s*$)`,
		`(.+?)(?:\s+project|\s+task|\s+activity)`,
	}

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true,
		"which": true, "what": true, "who": true, "whom": true, "whose": true,
		"this": true, "that": true, "these": true, "those": true,
		"i": true, "me": true, "my": true, "we": true, "our": true,
		"to": true, "for": true, "on": true, "in": true, "at": true,
		"of": true, "with": true, "by": true, "from": true, "about": true,
		"log": true, "use": true, "pick": true, "choose": true, "find": true,
		"search": true, "look": true, "start": true, "track": true, "record": true,
		"project": true, "task": true, "activity": true,
	}

	// Try extraction patterns first
	for _, pat := range extractPatterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(query); len(m) > 1 {
			words := strings.Fields(m[1])
			var keywords []string
			for _, w := range words {
				w = strings.ToLower(strings.Trim(w, ".,?!"))
				if len(w) > 1 && !stopWords[w] {
					keywords = append(keywords, w)
				}
			}
			if len(keywords) > 0 {
				return keywords
			}
		}
	}

	// Fall back to extracting all non-stop words
	words := strings.Fields(query)
	var keywords []string
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,?!"))
		if len(w) > 2 && !stopWords[w] {
			keywords = append(keywords, w)
		}
	}

	return keywords
}

// --- String matching utilities (adapted from kartoza-pg-ai) ---

// stemWord returns a simplified stem of a word
func stemWord(word string) string {
	word = strings.ToLower(word)

	suffixes := []string{
		"ology", "ological", "ation", "ations", "ment", "ments",
		"ness", "ity", "ies", "ing", "ed", "er", "ers", "est",
		"ly", "al", "ial", "ous", "ive", "able", "ible", "ful",
		"less", "ship", "ward", "wards", "wise",
	}

	// Handle plurals first
	if strings.HasSuffix(word, "ies") && len(word) > 4 {
		word = word[:len(word)-3] + "y"
	} else if strings.HasSuffix(word, "es") && len(word) > 3 {
		word = word[:len(word)-2]
	} else if strings.HasSuffix(word, "s") && len(word) > 2 && !strings.HasSuffix(word, "ss") {
		word = word[:len(word)-1]
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(word, suffix) && len(word)-len(suffix) >= 3 {
			word = word[:len(word)-len(suffix)]
			break
		}
	}

	return word
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = minInt(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func minInt(nums ...int) int {
	m := nums[0]
	for _, n := range nums[1:] {
		if n < m {
			m = n
		}
	}
	return m
}

// generateNgrams generates character n-grams from a string
func generateNgrams(s string, n int) map[string]bool {
	ngrams := make(map[string]bool)
	s = strings.ToLower(s)
	if len(s) < n {
		ngrams[s] = true
		return ngrams
	}
	for i := 0; i <= len(s)-n; i++ {
		ngrams[s[i:i+n]] = true
	}
	return ngrams
}

// ngramSimilarity calculates Jaccard similarity between n-gram sets
func ngramSimilarity(s1, s2 string, n int) float64 {
	ngrams1 := generateNgrams(s1, n)
	ngrams2 := generateNgrams(s2, n)

	if len(ngrams1) == 0 || len(ngrams2) == 0 {
		return 0
	}

	intersection := 0
	for ng := range ngrams1 {
		if ngrams2[ng] {
			intersection++
		}
	}

	union := len(ngrams1) + len(ngrams2) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// splitEntityName splits camelCase and snake_case into words
func splitEntityName(name string) []string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	var result []string
	var current strings.Builder
	for i, r := range name {
		if r == ' ' {
			if current.Len() > 0 {
				result = append(result, strings.ToLower(current.String()))
				current.Reset()
			}
			continue
		}
		if i > 0 && r >= 'A' && r <= 'Z' {
			if current.Len() > 0 {
				result = append(result, strings.ToLower(current.String()))
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		result = append(result, strings.ToLower(current.String()))
	}

	return result
}
