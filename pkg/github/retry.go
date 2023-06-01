package github

import (
	"errors"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

var (
	defaultRetryOptions = []retry.Option{
		retry.Delay(5 * time.Second),
		retry.MaxJitter(3 * time.Second),
		retry.Attempts(10),
		retry.MaxDelay(time.Second * 30),
		retry.LastErrorOnly(true),
	}
)

// Checks if the error should be retried or not
func checkIfRecoverable(err error) error {
	var rateLimitErr *github.RateLimitError
	isRateLimit := errors.As(err, &rateLimitErr)

	var abuseErr *github.AbuseRateLimitError
	isAbuseLimit := errors.As(err, &abuseErr)

	// If it is one of these errors, it can be retried
	if isRateLimit || isAbuseLimit {
		logrus.Info("Got rate limited, retrying")
		return err
	}

	return retry.Unrecoverable(err)
}
