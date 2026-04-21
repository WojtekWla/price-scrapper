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
