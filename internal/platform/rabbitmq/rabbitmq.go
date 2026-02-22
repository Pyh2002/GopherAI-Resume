package rabbitmq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func New(ctx context.Context, url string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq failed: %w", err)
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel failed: %w", err)
	}
	defer ch.Close()

	done := make(chan error, 1)
	go func() {
		_, queueErr := ch.QueueDeclarePassive(
			"healthcheck",
			false,
			false,
			false,
			false,
			nil,
		)
		done <- queueErr
	}()

	select {
	case <-checkCtx.Done():
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq health check timeout: %w", checkCtx.Err())
	case err := <-done:
		// QueueDeclarePassive on non-existing queue returns an error but it still
		// proves broker is reachable. We only fail on connection/channel failures above.
		_ = err
		return conn, nil
	}
}
