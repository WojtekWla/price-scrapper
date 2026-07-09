package notifier

import (
	"log"
	"price-scrapper/config"
	"price-scrapper/models"
	"price-scrapper/notifier/discord"
)

type Notifier struct {
	logProductsInConsole bool
	discordEnabled       bool
	discordNotifier      *discord.Notifier
}

func NewNotifier(conf config.NotifierConfig) *Notifier {
	communicationNotifier := Notifier{
		logProductsInConsole: conf.ConsoleFlag,
		discordEnabled:       conf.DiscordFlag,
	}

	if communicationNotifier.logProductsInConsole {
		log.Printf("Logging products in console enabled")
	} else {
		log.Printf("Logging in console disabled")
	}

	if communicationNotifier.discordEnabled && conf.DiscordWebhookURL != "" {
		log.Printf("Discord webhook successfully created")
		communicationNotifier.discordNotifier = discord.New(conf.DiscordWebhookURL)
	} else {
		log.Printf("Discord notification disabled")
	}

	return &communicationNotifier
}

func (n *Notifier) Notify(productName string, products []models.ScrapedProduct) {
	if n.logProductsInConsole {
		n.logInConsole(productName, products)
	}

	if n.discordEnabled {
		if err := n.discordNotifier.NotifyProducts(productName, products); err != nil {
			log.Printf("Discord notification failed for %q: %v", productName, err)
		}
	}
}

func (n *Notifier) logInConsole(productName string, products []models.ScrapedProduct) {
	log.Printf("Listing products %q", productName)

	for _, product := range products {
		log.Printf("**%s**\n%.2f PLN — [Link](%s)\n\n", product.Name, float64(product.Price)/100, product.Link)
	}
}
