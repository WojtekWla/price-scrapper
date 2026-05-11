package llm

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	model      = "gemini-2.5-flash-lite"
	instruction = "Extract name of the product, price of the product, and link to which this product belongs"
)

type GeminiService struct {
	client *genai.Client
}

func NewGeminiService(ctx context.Context, apiKey string) (*GeminiService, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	return &GeminiService{client: client}, nil
}

func (g *GeminiService) Close() {
	g.client.Close()
}

func (g *GeminiService) ExtractProducts(ctx context.Context, scrapedData string) (string, error) {
	prompt := scrapedData + "\n\n" + instruction

	m := g.client.GenerativeModel(model)
	resp, err := m.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("Gemini request failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("empty response from Gemini")
	}

	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	return result, nil
}
