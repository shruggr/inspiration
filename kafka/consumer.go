package kafka

import (
	"context"
	"log/slog"

	kafkamessage "github.com/bsv-blockchain/teranode/util/kafka/kafka_message"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

// SubtreeHandler is called for each subtree message consumed from Kafka.
type SubtreeHandler func(ctx context.Context, subtreeHash string, fetchURL string) error

// BlockHandler is called for each block-final message consumed from Kafka.
type BlockHandler func(ctx context.Context, height uint32, headerBytes []byte, subtreeHashes [][]byte, txCount uint64) error

// Consumer reads from Teranode's "subtrees" and "blocks-final" Kafka topics.
type Consumer struct {
	brokers        []string
	groupID        string
	subtreeHandler SubtreeHandler
	blockHandler   BlockHandler
	logger         *slog.Logger
}

// NewConsumer creates a new Kafka consumer for Teranode topics.
func NewConsumer(brokers []string, groupID string, subtreeHandler SubtreeHandler, blockHandler BlockHandler, logger *slog.Logger) *Consumer {
	return &Consumer{
		brokers:        brokers,
		groupID:        groupID,
		subtreeHandler: subtreeHandler,
		blockHandler:   blockHandler,
		logger:         logger,
	}
}

// Run starts consuming from both topics. It blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	errc := make(chan error, 2)

	go func() { errc <- c.consumeSubtrees(ctx) }()
	go func() { errc <- c.consumeBlocks(ctx) }()

	// Return the first error (context cancellation propagates to both).
	return <-errc
}

func (c *Consumer) consumeSubtrees(ctx context.Context) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  c.brokers,
		GroupID:  c.groupID,
		Topic:    "subtrees",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			return err
		}

		var pb kafkamessage.KafkaSubtreeTopicMessage
		if err := proto.Unmarshal(msg.Value, &pb); err != nil {
			c.logger.Error("unmarshal subtree message", "error", err, "offset", msg.Offset)
			if err := r.CommitMessages(ctx, msg); err != nil {
				return err
			}
			continue
		}

		if err := c.subtreeHandler(ctx, pb.GetHash(), pb.GetURL()); err != nil {
			c.logger.Error("handle subtree", "error", err, "hash", pb.GetHash())
			continue
		}

		if err := r.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *Consumer) consumeBlocks(ctx context.Context) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  c.brokers,
		GroupID:  c.groupID,
		Topic:    "blocks-final",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			return err
		}

		var pb kafkamessage.KafkaBlocksFinalTopicMessage
		if err := proto.Unmarshal(msg.Value, &pb); err != nil {
			c.logger.Error("unmarshal blocks-final message", "error", err, "offset", msg.Offset)
			if err := r.CommitMessages(ctx, msg); err != nil {
				return err
			}
			continue
		}

		if err := c.blockHandler(ctx, pb.GetHeight(), pb.GetHeader(), pb.GetSubtreeHashes(), pb.GetTransactionCount()); err != nil {
			c.logger.Error("handle block", "error", err, "height", pb.GetHeight())
			continue
		}

		if err := r.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}
