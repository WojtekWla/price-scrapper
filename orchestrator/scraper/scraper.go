package scraper

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"net/url"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

const (
	searchURL       = "https://duckduckgo.com"
	maxLinks        = 5
	pageTimeout     = 15 * time.Second
	idleTimeout     = 500 * time.Millisecond
	idleWaitTimeout = 5 * time.Second
)

type Scraper struct {
	browser *rod.Browser
}

func New() (*Scraper, error) {
	u, err := launcher.New().
		Headless(true).
		Set("disable-blink-features", "AutomationControlled").
		Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36").
		Set("lang", "en-US,en").
		NoSandbox(true).
		Launch()

	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().
		ControlURL(u).
		MustConnect()

	return &Scraper{browser: browser}, nil
}

// newStealthPage creates a page with all anti-detection scripts injected before
// any page JavaScript runs, using CDP's addScriptToEvaluateOnNewDocument.
func (s *Scraper) newStealthPage() (*rod.Page, error) {
	return stealth.Page(s.browser)
}

func (s *Scraper) Close() {
	s.browser.MustClose()
}

// SearchAndScrapeProduct searches DuckDuckGo for the product, crawls the first 5
// result pages, and returns compact extracted data ready to send to an LLM.
func (s *Scraper) SearchAndScrapeProduct(ctx context.Context, productName string) (string, error) {
	searchResultURLs, err := s.collectSearchResultURLs(ctx, productName)
	if err != nil {
		return "", fmt.Errorf("failed to collect search results: %w", err)
	}

	log.Printf("found %d URLs to scrape", len(searchResultURLs))

	combinedHTML, err := s.scrapePages(ctx, searchResultURLs)
	if err != nil {
		return "", fmt.Errorf("failed to scrape pages: %w", err)
	}

	return combinedHTML, nil
}

func (s *Scraper) collectSearchResultURLs(ctx context.Context, productName string) ([]string, error) {
	page, err := s.newStealthPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create stealth page: %w", err)
	}
	defer page.Close()

	page.MustSetViewport(1366, 768, 1, false)

	params := url.Values{}
	params.Set("q", productName)
	params.Set("kl", "pl-pl")
	target := searchURL + "?" + params.Encode()

	if err := page.Navigate(target); err != nil {
		return nil, fmt.Errorf("failed to navigate to DuckDuckGo: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("DuckDuckGo failed to load: %w", err)
	}

	_ = acceptCookieConsent(page)

	// Wait for organic results to be rendered
	if _, err := page.Timeout(pageTimeout).Element(`article[data-testid="result"]`); err != nil {
		return nil, fmt.Errorf("search results did not appear: %w", err)
	}

	allURLs, err := extractResultURLs(page)
	if err != nil {
		return nil, fmt.Errorf("failed to extract URLs: %w", err)
	}

	if len(allURLs) > maxLinks {
		allURLs = allURLs[:maxLinks]
	}

	log.Printf("found %d URLs to scrape", len(allURLs))

	return allURLs, nil
}

// extractResultURLs pulls href values from DuckDuckGo's organic result links.
func extractResultURLs(page *rod.Page) ([]string, error) {
	elements, err := page.Elements(`article[data-testid="result"] h2 a`)
	if err != nil {
		return nil, fmt.Errorf("failed to find result links: %w", err)
	}

	seen := make(map[string]bool)
	var urls []string

	for _, el := range elements {
		href, err := el.Attribute("href")
		if err != nil || href == nil || *href == "" {
			continue
		}
		if !seen[*href] {
			seen[*href] = true
			urls = append(urls, *href)
		}
	}

	return urls, nil
}

// scrapePages visits each URL concurrently and returns all extracted data joined
// by a separator that the LLM can use to distinguish sources.
func (s *Scraper) scrapePages(ctx context.Context, urls []string) (string, error) {
	type result struct {
		index int
		url   string
		data  string
		err   error
	}

	results := make([]result, len(urls))
	ch := make(chan result, len(urls))
	sem := make(chan struct{}, 3)

	for i, u := range urls {
		go func(idx int, u string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			data, err := s.fetchPageData(ctx, u)
			ch <- result{index: idx, url: u, data: data, err: err}
		}(i, u)
	}

	for range urls {
		r := <-ch
		results[r.index] = r
	}

	var sb strings.Builder

	for _, r := range results {
		if r.err != nil {
			log.Printf("skipping %s: %v", r.url, r.err)
			continue
		}
		sb.WriteString(fmt.Sprintf("\n\n--- SOURCE: %s ---\n\n", r.url))
		sb.WriteString(r.data)
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("no pages could be scraped")
	}

	return sb.String(), nil
}

// acceptCookieConsent tries to dismiss cookie/consent banners silently.
// Covers the most common GDPR CMP providers found on Polish e-commerce sites.
// Failure is not fatal — scraping continues regardless.
func acceptCookieConsent(page *rod.Page) error {
	// Single combined selector so we pay at most one 3s timeout instead of N×timeout.
	const combined = `` +
		`#onetrust-accept-btn-handler,` + // OneTrust
		`button.onetrust-close-btn-handler,` +
		`#CybotCookiebotDialogBodyButtonAccept,` + // Cookiebot
		`a#CybotCookiebotDialogBodyButtonAccept,` +
		`button[id="CybotCookiebotDialogBodyLevelButtonLevelOptinAllowAll"],` +
		`button[data-action-type="accept"],` + // Usercentrics
		`button.uc-accept-button,` +
		`button[data-testid="accept-all"],` + // DuckDuckGo
		`a.btn--primary,` +
		`button.btn--primary,` +
		`button[class*="accept-all"],` + // generic "accept all"
		`button[class*="acceptAll"],` +
		`button[id*="accept-all"],` +
		`button[id*="acceptAll"],` +
		`a[class*="accept-all"],` +
		`[aria-label*="Accept all" i],` + // case-insensitive English
		`[aria-label*="Zaakceptuj" i],` + // case-insensitive Polish
		`[aria-label*="Akceptuj" i],` +
		`button[class*="zgadzam" i],` + // Polish "I agree"
		`button[title*="Akceptuj" i]` // Polish "Accept"

	btn, err := page.Timeout(3 * time.Second).Element(combined)
	if err != nil {
		return nil
	}
	return btn.Click(proto.InputMouseButtonLeft, 1)
}

// fetchPageData opens a URL, extracts Schema.org JSON-LD and clean visible
// text (scripts/styles/nav stripped), and returns a compact string ready to
// include in an LLM prompt.
func (s *Scraper) fetchPageData(ctx context.Context, pageURL string) (string, error) {
	page, err := s.newStealthPage()
	if err != nil {
		return "", fmt.Errorf("failed to create stealth page: %w", err)
	}
	defer page.Close()

	if err := page.Navigate(pageURL); err != nil {
		return "", fmt.Errorf("failed to navigate to page: %w", err)
	}

	if err := page.Timeout(pageTimeout).WaitLoad(); err != nil {
		log.Printf("WaitLoad timeout for %s: %v", pageURL, err)
	}

	_ = acceptCookieConsent(page)

	page.Timeout(idleWaitTimeout).WaitRequestIdle(idleTimeout, nil, nil, nil)()

	// Extract Schema.org JSON-LD blocks — compact structured product data.
	jsonLD, err := page.Eval(`() =>
		Array.from(document.querySelectorAll('script[type="application/ld+json"]'))
			.map(s => s.textContent.trim())
			.filter(t => t.length > 0)
			.join('\n')
	`)
	if err != nil {
		log.Printf("JSON-LD extraction failed for %s: %v", pageURL, err)
	}

	// Extract visible text: remove noisy elements, insert newlines at block
	// boundaries, collapse whitespace.
	pageText, err := page.Eval(`() => {
		const clone = document.body.cloneNode(true);
		clone.querySelectorAll(
			'script,style,nav,header,footer,aside,noscript,iframe,svg'
		).forEach(el => el.remove());
		clone.querySelectorAll(
			'p,div,h1,h2,h3,h4,h5,h6,li,tr,br,section,article'
		).forEach(el => el.prepend('\n'));
		return clone.textContent
			.replace(/[ \t]+/g, ' ')
			.replace(/\n{3,}/g, '\n\n')
			.trim();
	}`)
	if err != nil {
		return "", fmt.Errorf("text extraction failed for %s: %w", pageURL, err)
	}

	var sb strings.Builder

	if jsonLD != nil && jsonLD.Value.Str() != "" {
		sb.WriteString("=== STRUCTURED DATA ===\n")
		sb.WriteString(jsonLD.Value.Str())
		sb.WriteString("\n")
	}

	sb.WriteString("=== PAGE TEXT ===\n")
	sb.WriteString(pageText.Value.Str())

	return sb.String(), nil
}
