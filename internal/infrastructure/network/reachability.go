package network

import (
	"fmt"
	"log/slog"
	"net"
	"time"
)

// CheckReachability performs a TCP reachability check with retries.
func CheckReachability(host string, port int, attempts int, delay time.Duration) error {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	var err error

	for i := 1; i <= attempts; i++ {
		slog.Info("Checking reachability", 
			slog.String("address", address), 
			slog.Int("attempt", i), 
			slog.Int("max_attempts", attempts))

		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err == nil {
			conn.Close()
			slog.Info("Service reachable", slog.String("address", address))
			return nil
		}

		slog.Warn("Service unreachable", 
			slog.String("address", address), 
			slog.Int("attempt", i), 
			slog.Any("error", err))

		if i < attempts {
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to reach %s after %d attempts: %w", address, attempts, err)
}
