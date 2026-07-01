package models

type Product struct {
	Name         string
	SitesToVisit []string
	Frequency    string
}

type Job struct {
	Id           string
	ProductName  string
	SitesToVisit []string
	Frequency    string
	TimeToRun    int64
}

type ScrapedProduct struct {
	SearchTerm string
	Name       string
	Price      int64
	Link       string
	Time       int64
}
