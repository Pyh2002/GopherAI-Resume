package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"gopherai-resume/internal/model"
	"gopherai-resume/internal/repository"
)

type MessagePersistWorker struct {
	conn      *amqp.Connection
	repo      *repository.MessageRepository
	queueName string

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewMessagePersistWorker(conn *amqp.Connection, repo *repository.MessageRepository, queueName string) *MessagePersistWorker {
	return &MessagePersistWorker{
		conn:      conn,
		repo:      repo,
		queueName: queueName,
	}
}

func (w *MessagePersistWorker) Start(ctx context.Context) error {
	if w.cancel != nil {
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	ch, err := w.conn.Channel()
	if err != nil {
		cancel()
		return fmt.Errorf("open worker channel failed: %w", err)
	}

	_, err = ch.QueueDeclare(
		w.queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		cancel()
		return fmt.Errorf("declare worker queue failed: %w", err)
	}

	deliveries, err := ch.Consume(
		w.queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		cancel()
		return fmt.Errorf("consume queue failed: %w", err)
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer ch.Close()

		for {
			select {
			case <-workerCtx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}

				var msg model.Message
				if err := json.Unmarshal(d.Body, &msg); err != nil {
					log.Printf("worker decode message failed: %v", err)
					_ = d.Nack(false, false)
					continue
				}

				if err := w.repo.Create(&msg); err != nil {
					log.Printf("worker persist message failed: %v", err)
					_ = d.Nack(false, false)
					continue
				}

				_ = d.Ack(false)
			}
		}
	}()

	return nil
}

func (w *MessagePersistWorker) Close() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}
