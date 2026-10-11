package repository

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestCacheReadWriteAndMiss(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	repo := &RedisRepo{Client: client}
	ctx := context.Background()
	if _, err := repo.GetLongURL(ctx, "c"); !errors.Is(err, redis.Nil) || !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("miss error %v", err)
	}
	target := "https://example.com/a%2Fb?q=a%2Bb#fragment"
	if err := repo.SetKey(ctx, "c", target, time.Hour); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetLongURL(ctx, "c"); err != nil || got != target {
		t.Fatalf("got %q, error %v", got, err)
	}
	if server.TTL("c") != time.Hour {
		t.Fatalf("TTL %s", server.TTL("c"))
	}
}

// Accept a TCP connection but never reply, exercising real go-redis socket deadlines.
func TestRedisUnresponsivePeerIsBounded(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	done := make(chan struct{})
	release := make(chan struct{})
	defer func() { close(release); <-done }()
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		// Deliberately never answer the client handshake.
		<-release
	}()
	opts := redisOptions(40 * time.Millisecond)
	opts.Addr = listener.Addr().String()
	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := (&RedisRepo{Client: client}).Ping(ctx); err == nil {
		t.Fatal("unresponsive peer accepted")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("Redis exceeded budget: %s", time.Since(start))
	}
}
