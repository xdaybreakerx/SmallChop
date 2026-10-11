package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestListenerFailureStopsOtherServer(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	other := &http.Server{Addr: "127.0.0.1:0"}
	if err := serve(ctx, &http.Server{Addr: occupied.Addr().String()}, other); err == nil {
		t.Fatal("listener bind failure hidden")
	}
	if err := other.ListenAndServe(); err != http.ErrServerClosed {
		t.Fatalf("other listener not shut down: %v", err)
	}
}

func TestCancellationStopsListeners(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &http.Server{Addr: "127.0.0.1:0"}
	if err := serve(ctx, server); err != nil {
		t.Fatal(err)
	}
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		t.Fatalf("listener not shut down: %v", err)
	}
}
