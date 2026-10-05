package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrNotFound    = errors.New("URL mapping not found")
	ErrUnavailable = errors.New("storage unavailable")
)

// Keep driver errors at the repository boundary, preserving their causes for diagnostics.
func databaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if mongo.IsNetworkError(err) || mongo.IsTimeout(err) || errors.Is(err, context.Canceled) ||
		errors.Is(err, mongo.ErrClientDisconnected) || transientServerError(err) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return err
}

// Classify availability errors without treating malformed commands or duplicate keys as outages.
// Codes: https://www.mongodb.com/docs/manual/reference/error-codes/
func transientServerError(err error) bool {
	var serverError mongo.ServerError
	if !errors.As(err, &serverError) {
		return false
	}
	if serverError.HasErrorLabel("RetryableWriteError") || serverError.HasErrorLabel("TransientTransactionError") {
		return true
	}
	for _, code := range []int{91, 10107, 11600, 11602, 13435, 13436} {
		if serverError.HasErrorCode(code) {
			return true
		}
	}
	return false
}
