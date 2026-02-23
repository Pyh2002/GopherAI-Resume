package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"gopherai-resume/internal/model"
)

type MessagePublisher struct {
	conn      *amqp.Connection
	queueName string
}

func NewMessagePublisher(conn *amqp.Connection, queueName string) *MessagePublisher {
	return &MessagePublisher{
		conn:      conn,
		queueName: queueName,
	}
}

func (p *MessagePublisher) Publish(ctx context.Context, msg model.Message) error {
	ch, err := p.conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel failed: %w", err)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		p.queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare queue failed: %w", err)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message payload failed: %w", err)
	}

	if err := ch.PublishWithContext(
		ctx,
		"",
		p.queueName,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         payload,
			DeliveryMode: amqp.Persistent,
		},
	); err != nil {
		return fmt.Errorf("publish message failed: %w", err)
	}
	return nil
}
