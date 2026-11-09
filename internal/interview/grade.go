package interview

import (
	"math"

	"ai-interview-coach/internal/domain"
)

func AggregateFeedback(session domain.InterviewSession, answers []domain.InterviewAnswer) domain.Feedback {
	if len(answers) == 0 {
		return domain.Feedback{OverallScore: 0, Grade: "Junior-", TopicScores: map[string]int{}}
	}

	var total float64
	topicSum := map[string]float64{}
	topicCount := map[string]int{}
	coverage := map[string]bool{}
	for _, ans := range answers {
		weight := difficultyWeight(ans.Difficulty)
		if ans.IsFollowUp {
			weight *= 0.9
		}
		score := float64(ans.Evaluation.Correctness*45+ans.Evaluation.Depth*35+ans.Evaluation.Clarity*20) / 10.0
		total += score * weight
		topicSum[ans.Topic] += score
		topicCount[ans.Topic]++
		coverage[ans.Topic] = true
	}

	avg := total / float64(len(answers))
	coverageRatio := float64(len(coverage)) / math.Max(1, float64(len(session.Plan)))
	if coverageRatio > 1 {
		coverageRatio = 1
	}
	overall := int(math.Round(avg*0.85 + coverageRatio*15))
	if overall > 100 {
		overall = 100
	}

	topicScores := make(map[string]int, len(topicSum))
	for topic, sum := range topicSum {
		topicScores[topic] = int(math.Round(sum / float64(topicCount[topic])))
	}

	return domain.Feedback{
		OverallScore: overall,
		Grade:        GradeFromScore(overall),
		TopicScores:  topicScores,
	}
}

func GradeFromScore(score int) string {
	switch {
	case score < 35:
		return "Junior-"
	case score < 45:
		return "Junior"
	case score < 55:
		return "Junior+"
	case score < 62:
		return "Middle-"
	case score < 72:
		return "Middle"
	case score < 82:
		return "Middle+"
	case score < 88:
		return "Senior-"
	case score < 94:
		return "Senior"
	default:
		return "Senior+"
	}
}

func difficultyWeight(d domain.Difficulty) float64 {
	switch d {
	case domain.DifficultyHarder:
		return 1.08
	case domain.DifficultyEasier:
		return 0.92
	default:
		return 1.0
	}
}
