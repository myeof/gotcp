package tcp

import (
	"golang.org/x/time/rate"
)

var sendRateLimiter *rate.Limiter
var receiveRateLimiter *rate.Limiter

func InitRate(limit float64) {
	if limit == 0 {
		return
	}
	sendRateLimiter = rate.NewLimiter(rate.Limit(limit), int(limit))
	receiveRateLimiter = rate.NewLimiter(rate.Limit(limit), int(limit))
}
