package store

import (
	"errors"
	"testing"
)

func TestIsRedisReadOnlyErr(t *testing.T) {
	err := errors.New("READONLY You can't write against a read only replica.")
	if !isRedisReadOnlyErr(err) {
		t.Fatal("expected READONLY error to be detected")
	}
	if isRedisReadOnlyErr(errors.New("connection refused")) {
		t.Fatal("expected non-READONLY error to be ignored")
	}
}

func TestRedisHandleWriteErrSuppressesReadOnlyTemporarily(t *testing.T) {
	client := NewRedis(DisabledRedisConfig())
	err := errors.New("READONLY You can't write against a read only replica.")
	if got := client.handleWriteErr(err); got != nil {
		t.Fatalf("expected READONLY write error to be downgraded, got %v", got)
	}
	if !client.skipWrite() {
		t.Fatal("expected redis writes to be skipped after READONLY")
	}

	other := errors.New("connection refused")
	if got := client.handleWriteErr(other); !errors.Is(got, other) {
		t.Fatalf("expected non-READONLY error to be returned, got %v", got)
	}
}
