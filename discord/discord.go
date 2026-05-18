package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"price-scrapper/models"
	"strings"
)

const batchSize = 10

type Notifier struct {
	webhookURL string
	client     *http.Client
}

func New(webhookURL string) *Notifier {
	return &Notifier{
		webhookURL: webhookURL,
		client:     &http.Client{},
	}
}

type embed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
}

type webhookPayload struct {
	Embeds []embed `json:"embeds"`
}

func (n *Notifier) NotifyProducts(searchTerm string, products []models.ScrapedProduct) error {
	total := len(products)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := products[i:end]
		batchNum := i/batchSize + 1
		totalBatches := (total + batchSize - 1) / batchSize

		var sb strings.Builder
		for _, p := range batch {
			sb.WriteString(fmt.Sprintf("**%s**\n%.2f PLN — [Link](%s)\n\n", p.Name, float64(p.Price)/100, p.Link))
		}

		title := fmt.Sprintf("Price update: %s (%d/%d)", searchTerm, batchNum, totalBatches)
		if err := n.sendEmbed(title, sb.String()); err != nil {
			return fmt.Errorf("batch %d/%d failed: %w", batchNum, totalBatches, err)
		}
	}
	return nil
}

func (n *Notifier) sendEmbed(title, description string) error {
	payload := webhookPayload{
		Embeds: []embed{
			{
				Title:       title,
				Description: description,
				Color:       0x5865F2,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := n.client.Post(n.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
