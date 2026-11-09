package interview

import "ai-interview-coach/internal/domain"

type SpecDefinition struct {
	Slug         string
	SystemPrompt string
	Plans        map[domain.Level][]domain.PlanArea
}

type Catalog map[string]SpecDefinition

func DefaultCatalog() Catalog {
	return Catalog{
		"go-backend": {
			Slug:         "go-backend",
			SystemPrompt: "Interview a Go backend developer. Focus on Go internals, concurrency, HTTP services, PostgreSQL, architecture, testing, and production tradeoffs.",
			Plans: map[domain.Level][]domain.PlanArea{
				domain.LevelJunior: {{Topic: "Go Core", Weight: 35}, {Topic: "HTTP/REST", Weight: 20}, {Topic: "Databases", Weight: 15}, {Topic: "Testing", Weight: 10}, {Topic: "Concurrency", Weight: 10}, {Topic: "DevOps", Weight: 5}, {Topic: "Architecture", Weight: 5}},
				domain.LevelMiddle: {{Topic: "Go Core", Weight: 25}, {Topic: "Concurrency", Weight: 20}, {Topic: "Databases", Weight: 15}, {Topic: "Backend/HTTP", Weight: 15}, {Topic: "Architecture", Weight: 10}, {Topic: "Testing", Weight: 5}, {Topic: "DevOps", Weight: 5}, {Topic: "System Design", Weight: 5}},
				domain.LevelSenior: {{Topic: "System Design", Weight: 20}, {Topic: "Architecture", Weight: 20}, {Topic: "Concurrency", Weight: 15}, {Topic: "Databases", Weight: 15}, {Topic: "Production", Weight: 10}, {Topic: "Go Internals", Weight: 10}, {Topic: "Testing", Weight: 5}, {Topic: "DevOps", Weight: 5}},
			},
		},
		"java-backend": {
			Slug:         "java-backend",
			SystemPrompt: "Interview a Java backend developer. Focus on JVM, collections, concurrency, Spring-style services, databases, testing, architecture, and production diagnostics.",
			Plans: map[domain.Level][]domain.PlanArea{
				domain.LevelJunior: {{Topic: "Java Core", Weight: 35}, {Topic: "OOP", Weight: 20}, {Topic: "HTTP/REST", Weight: 15}, {Topic: "Databases", Weight: 15}, {Topic: "Testing", Weight: 10}, {Topic: "DevOps", Weight: 5}},
				domain.LevelMiddle: {{Topic: "Java Core", Weight: 20}, {Topic: "JVM", Weight: 15}, {Topic: "Concurrency", Weight: 15}, {Topic: "Spring/Backend", Weight: 15}, {Topic: "Databases", Weight: 15}, {Topic: "Architecture", Weight: 10}, {Topic: "Testing", Weight: 5}, {Topic: "DevOps", Weight: 5}},
				domain.LevelSenior: {{Topic: "Architecture", Weight: 20}, {Topic: "System Design", Weight: 20}, {Topic: "JVM Performance", Weight: 15}, {Topic: "Concurrency", Weight: 15}, {Topic: "Databases", Weight: 15}, {Topic: "Production", Weight: 10}, {Topic: "Testing", Weight: 5}},
			},
		},
		"ts-frontend": {
			Slug:         "ts-frontend",
			SystemPrompt: "Interview a JavaScript/TypeScript frontend developer. Focus on JS/TS fundamentals, browser APIs, React-style UI architecture, performance, testing, accessibility, and frontend system design.",
			Plans: map[domain.Level][]domain.PlanArea{
				domain.LevelJunior: {{Topic: "JavaScript Core", Weight: 30}, {Topic: "TypeScript", Weight: 20}, {Topic: "HTML/CSS", Weight: 15}, {Topic: "Browser", Weight: 15}, {Topic: "Testing", Weight: 10}, {Topic: "Accessibility", Weight: 10}},
				domain.LevelMiddle: {{Topic: "JavaScript Core", Weight: 20}, {Topic: "TypeScript", Weight: 15}, {Topic: "UI Architecture", Weight: 20}, {Topic: "Performance", Weight: 15}, {Topic: "Testing", Weight: 10}, {Topic: "Browser", Weight: 10}, {Topic: "Accessibility", Weight: 10}},
				domain.LevelSenior: {{Topic: "Frontend Architecture", Weight: 25}, {Topic: "System Design", Weight: 20}, {Topic: "Performance", Weight: 15}, {Topic: "TypeScript", Weight: 15}, {Topic: "Testing Strategy", Weight: 10}, {Topic: "Accessibility", Weight: 10}, {Topic: "Build Tooling", Weight: 5}},
			},
		},
	}
}

func (c Catalog) Plan(slug string, level domain.Level) []domain.PlanArea {
	if spec, ok := c[slug]; ok {
		if plan, ok := spec.Plans[level]; ok {
			return clonePlan(plan)
		}
	}
	return []domain.PlanArea{{Topic: "Core", Weight: 50}, {Topic: "Practical Experience", Weight: 25}, {Topic: "Architecture", Weight: 25}}
}

func (c Catalog) Prompt(slug string) string {
	if spec, ok := c[slug]; ok {
		return spec.SystemPrompt
	}
	return "Interview a software engineer for the selected specialization. Adapt questions to the requested level."
}

func clonePlan(in []domain.PlanArea) []domain.PlanArea {
	out := make([]domain.PlanArea, len(in))
	copy(out, in)
	return out
}
