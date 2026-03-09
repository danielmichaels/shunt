// file: internal/broker/terminal_error_test.go

package broker

import (
	"errors"
	"fmt"
	"testing"

	"github.com/danielmichaels/shunt/internal/rule"
)

func TestIsTerminalError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ErrMalformedJSON is terminal",
			err:  ErrMalformedJSON,
			want: true,
		},
		{
			name: "ErrInvalidPayload is terminal",
			err:  ErrInvalidPayload,
			want: true,
		},
		{
			name: "rule.ErrMalformedPayload is terminal",
			err:  rule.ErrMalformedPayload,
			want: true,
		},
		{
			name: "wrapped ErrMalformedJSON is terminal",
			err:  fmt.Errorf("processing failed: %w", ErrMalformedJSON),
			want: true,
		},
		{
			name: "wrapped ErrInvalidPayload is terminal",
			err:  fmt.Errorf("validation failed: %w", ErrInvalidPayload),
			want: true,
		},
		{
			name: "wrapped rule.ErrMalformedPayload is terminal",
			err:  fmt.Errorf("not UTF-8: %w", rule.ErrMalformedPayload),
			want: true,
		},
		{
			name: "generic error is not terminal",
			err:  errors.New("connection reset"),
			want: false,
		},
		{
			name: "timeout error is not terminal",
			err:  errors.New("context deadline exceeded"),
			want: false,
		},
		{
			name: "network error is not terminal",
			err:  fmt.Errorf("dial tcp: connection refused"),
			want: false,
		},
		{
			name: "nil error is not terminal",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTerminalError(tt.err)
			if got != tt.want {
				t.Errorf("isTerminalError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsTerminalError_DoublyWrapped(t *testing.T) {
	wrapped := fmt.Errorf("layer 2: %w", fmt.Errorf("layer 1: %w", ErrMalformedJSON))
	if !isTerminalError(wrapped) {
		t.Error("isTerminalError should detect doubly-wrapped ErrMalformedJSON")
	}
}
