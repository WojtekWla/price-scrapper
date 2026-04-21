package service

import "time"

type handle_freq func() int64

var FrequencyHandler = map[string]handle_freq{
	"every minute": func() int64 {
		return time.Now().Add(1 * time.Minute).Unix()
	},
	"every 5 minutes": func() int64 {
		return time.Now().Add(5 * time.Minute).Unix()
	},
	"hourly": func() int64 {
		return time.Now().Add(1 * time.Hour).Unix()
	},
	"daily": func() int64 {
		return time.Now().Add(24 * time.Hour).Unix()
	},
}
