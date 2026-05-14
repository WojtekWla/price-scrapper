package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"price-scrapper/models"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	model       = "gemini-2.5-flash-lite"
	instruction = "Extract all products from the provided data. For each product return its name, price as an integer in the smallest currency unit, and the source link."
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

func (g *GeminiService) ExtractProducts(ctx context.Context, scrapedData string) ([]models.ScrapedProduct, error) {
	prompt := scrapedData + "\n\n" + instruction

	m := g.client.GenerativeModel(model)
	m.ResponseMIMEType = "application/json"
	m.ResponseSchema = &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"name":  {Type: genai.TypeString},
				"price": {Type: genai.TypeInteger},
				"link":  {Type: genai.TypeString},
			},
			Required: []string{"name", "price", "link"},
		},
	}

	resp, err := m.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("Gemini request failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	var raw string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			raw += string(text)
		}
	}

	type geminiProduct struct {
		Name  string `json:"name"`
		Price int64  `json:"price"`
		Link  string `json:"link"`
	}

	var parsed []geminiProduct
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	products := make([]models.ScrapedProduct, len(parsed))
	for i, p := range parsed {
		products[i] = models.ScrapedProduct{
			Name:  p.Name,
			Price: p.Price,
			Link:  p.Link,
		}
	}

	return products, nil
}
