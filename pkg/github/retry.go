package github

import (
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

var (
	defaultRetryOptions = []retry.Option{
		retry.Delay(5 * time.Second),
		retry.MaxJitter(3 * time.Second),
		retry.Attempts(3),
		retry.MaxDelay(time.Second * 10),
		retry.LastErrorOnly(true),
	}
)

func checkIfRecoverable(err error) error {
	_, isRateLimit := err.(*github.RateLimitError)
	_, isAbuseLimit := err.(*github.AbuseRateLimitError)

	// If it is one of these errors, it can be retried
	if isRateLimit || isAbuseLimit {
		logrus.Info("Got rate limited, retrying")
		return err
	}

	return retry.Unrecoverable(err)
}
