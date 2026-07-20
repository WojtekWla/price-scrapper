package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"price-scrapper/models"
	"time"

	"github.com/google/generative-ai-go/genai"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"
)

const (
	model       = "gemini-2.5-flash-lite"
	instruction = "The data is the cleaned HTML of a shop's search results page. Extract all product offers from it. " +
		"For each product return its name, its price as an integer in the smallest currency unit, and its link. " +
		"The link MUST be the value of the href attribute of the <a> tag that wraps or belongs to that specific product's offer (its own product page). " +
		"Take the href verbatim from the HTML — do not invent or modify URLs, and never use a search, listing or category page URL. " +
		"If a product has no own offer href in the HTML, skip it."
)

type GeminiService struct {
	client  *genai.Client
	limiter *rate.Limiter
}

func NewGeminiService(ctx context.Context, apiKey string, requestsPerMinute int) (*GeminiService, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	limiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), 1)
	return &GeminiService{client: client, limiter: limiter}, nil
}

func (g *GeminiService) Close() {
	g.client.Close()
}

func (g *GeminiService) ExtractProducts(ctx context.Context, scrapedData string) ([]models.ScrapedProduct, error) {
	if err := g.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter interrupted: %w", err)
	}

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
