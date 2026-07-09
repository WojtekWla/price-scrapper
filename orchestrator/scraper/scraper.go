package scraper

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
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

// searchBoxSelector matches the most common on-site search inputs across Polish
// e-commerce shops. The first visible match is used to type the product query.
const searchBoxSelector = `` +
	`input[type="search"],` +
	`input[name="q"],` +
	`input[name="query"],` +
	`input[name="search"],` +
	`input[name="text"],` +
	`input[name="phrase"],` +
	`input[name="string"],` +
	`input[name="szukaj"],` +
	`input[id*="search" i],` +
	`input[class*="search" i],` +
	`input[placeholder*="szukaj" i],` +
	`input[placeholder*="search" i],` +
	`input[aria-label*="szukaj" i],` +
	`input[aria-label*="search" i]`

type Scraper struct {
	browser *rod.Browser
}

func New() (*Scraper, error) {
	l := launcher.New().
		Headless(false).
		Set("disable-blink-features", "AutomationControlled").
		Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36").
		Set("lang", "en-US,en").
		NoSandbox(true)

	if bin := os.Getenv("CHROME_BIN"); bin != "" {
		l = l.Bin(bin)
	}

	u, err := l.Launch()

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

// SearchAndScrapeProduct visits each given site one by one: it reaches the shop
// through DuckDuckGo, uses the shop's own on-site search box to look up the
// product, and scrapes the resulting listing page. Results from all sites are
// joined with a separator so the LLM can tell the sources apart. When no sites
// are provided it falls back to a global DuckDuckGo search.
func (s *Scraper) SearchAndScrapeProduct(ctx context.Context, productName string, sites []string) (string, error) {
	if len(sites) == 0 {
		return s.globalSearch(ctx, productName)
	}

	var sb strings.Builder
	scraped := 0

	for _, site := range sites {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		log.Printf("searching %q on site %q", productName, site)
		data, err := s.searchProductOnSite(ctx, productName, site)
		if err != nil {
			log.Printf("site %q failed: %v", site, err)
			continue
		}

		scraped++
		sb.WriteString(fmt.Sprintf("\n\n--- SOURCE: %s ---\n\n", site))
		sb.WriteString(data)
	}

	if scraped == 0 {
		return "", fmt.Errorf("no product results across %d site(s)", len(sites))
	}

	log.Printf("scraped %d/%d site(s) for %q", scraped, len(sites), productName)
	return sb.String(), nil
}

// searchProductOnSite reaches the shop via DuckDuckGo, runs the product query in
// the shop's own search box and scrapes the results page it lands on.
func (s *Scraper) searchProductOnSite(ctx context.Context, productName, site string) (string, error) {
	domain := normalizeSiteDomain(site)
	if domain == "" {
		return "", fmt.Errorf("invalid site %q", site)
	}

	page, err := s.newStealthPage()
	if err != nil {
		return "", fmt.Errorf("failed to create stealth page: %w", err)
	}
	defer page.Close()

	page.MustSetViewport(1366, 768, 1, false)

	if err := s.reachSiteViaSearch(ctx, page, domain); err != nil {
		return "", fmt.Errorf("failed to reach %s: %w", domain, err)
	}

	if err := performSiteSearch(page, productName); err != nil {
		return "", fmt.Errorf("on-site search on %s failed: %w", domain, err)
	}

	landedURL := domain
	if info, err := page.Info(); err == nil {
		landedURL = info.URL
	}

	return extractPageData(page, landedURL)
}

// reachSiteViaSearch opens DuckDuckGo, searches for the shop domain and clicks
// the first result on that domain so we arrive at the shop like an organic
// visitor. If the click does not land us on the shop, it navigates directly.
func (s *Scraper) reachSiteViaSearch(ctx context.Context, page *rod.Page, domain string) error {
	params := url.Values{}
	params.Set("q", domain)
	params.Set("kl", "pl-pl")

	if err := page.Navigate(searchURL + "?" + params.Encode()); err != nil {
		return fmt.Errorf("failed to navigate to DuckDuckGo: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("DuckDuckGo failed to load: %w", err)
	}

	_ = acceptCookieConsent(page)

	if _, err := page.Timeout(pageTimeout).Element(`article[data-testid="result"]`); err != nil {
		return fmt.Errorf("no DuckDuckGo results for %q: %w", domain, err)
	}

	links, err := page.Elements(`article[data-testid="result"] h2 a`)
	if err != nil || len(links) == 0 {
		return fmt.Errorf("no result links for %q", domain)
	}

	target := links[0]
	for _, l := range links {
		if href, err := l.Attribute("href"); err == nil && href != nil && strings.Contains(*href, domain) {
			target = l
			break
		}
	}

	if err := target.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click result: %w", err)
	}
	if err := page.Timeout(pageTimeout).WaitLoad(); err != nil {
		log.Printf("shop load wait timeout for %s: %v", domain, err)
	}

	// If clicking did not leave DuckDuckGo, fall back to a direct visit.
	if info, err := page.Info(); err == nil && !strings.Contains(info.URL, domain) {
		log.Printf("click did not reach %s (at %s), navigating directly", domain, info.URL)
		if err := page.Navigate("https://" + domain); err != nil {
			return fmt.Errorf("direct navigation to %s failed: %w", domain, err)
		}
		_ = page.Timeout(pageTimeout).WaitLoad()
	}

	_ = acceptCookieConsent(page)
	return nil
}

// performSiteSearch finds the shop's search input, types the product name and
// submits it, then waits for the results page to settle.
func performSiteSearch(page *rod.Page, productName string) error {
	box, err := page.Timeout(pageTimeout).Element(searchBoxSelector)
	if err != nil {
		return fmt.Errorf("search box not found: %w", err)
	}

	if err := box.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to focus search box: %w", err)
	}
	_ = box.SelectAllText()
	if err := box.Input(productName); err != nil {
		return fmt.Errorf("failed to type query: %w", err)
	}
	if err := box.Type(input.Enter); err != nil {
		return fmt.Errorf("failed to submit query: %w", err)
	}

	if err := page.Timeout(pageTimeout).WaitLoad(); err != nil {
		log.Printf("results load timeout: %v", err)
	}
	page.Timeout(idleWaitTimeout).WaitRequestIdle(idleTimeout, nil, nil, nil)()

	_ = acceptCookieConsent(page)
	return nil
}

// globalSearch is the fallback used when a product has no sites configured: it
// runs a single global DuckDuckGo search and scrapes the top result pages.
func (s *Scraper) globalSearch(ctx context.Context, productName string) (string, error) {
	urls, err := s.collectSearchResultURLs(ctx, productName)
	if err != nil {
		return "", fmt.Errorf("failed to collect search results: %w", err)
	}
	if len(urls) == 0 {
		return "", fmt.Errorf("no search results found for %q", productName)
	}

	log.Printf("global search found %d URLs to scrape", len(urls))

	combinedHTML, err := s.scrapePages(ctx, urls)
	if err != nil {
		return "", fmt.Errorf("failed to scrape pages: %w", err)
	}
	return combinedHTML, nil
}

// normalizeSiteDomain reduces a stored site value (domain or full URL) to a bare
// host suitable for DuckDuckGo's `site:` operator.
func normalizeSiteDomain(site string) string {
	site = strings.TrimSpace(site)
	if site == "" {
		return ""
	}
	if !strings.Contains(site, "://") {
		site = "//" + site
	}
	u, err := url.Parse(site)
	if err != nil || u.Host == "" {
		return strings.TrimPrefix(strings.TrimSpace(site), "//")
	}
	return u.Host
}

func (s *Scraper) collectSearchResultURLs(ctx context.Context, query string) ([]string, error) {
	page, err := s.newStealthPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create stealth page: %w", err)
	}
	defer page.Close()

	page.MustSetViewport(1366, 768, 1, false)

	params := url.Values{}
	params.Set("q", query)
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

// fetchPageData opens a URL and returns its extracted product data. Used by the
// global-search fallback where we scrape result URLs directly.
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

	return extractPageData(page, pageURL)
}

// extractPageData pulls Schema.org JSON-LD and clean visible text (scripts,
// styles and chrome stripped) from an already-loaded page and returns a compact
// string ready to include in an LLM prompt.
func extractPageData(page *rod.Page, pageURL string) (string, error) {
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
