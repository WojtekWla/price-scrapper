package models

type Product struct {
	Name      string
	Frequency string
}

type Job struct {
	Id          string
	ProductName string
	Frequency   string
	TimeToRun   int64
}

type ScrapedProduct struct {
	Name  string
	Price int64
	Link  string
	Time  int64
}
