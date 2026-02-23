package ingest

import (
	"context"
	"log/slog"

	"github.com/segmentio/kafka-go"

	"rfguard/internal/config"
	"rfguard/internal/model"
	"rfguard/internal/normalize"
)

func StartKafka(ctx context.Context, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	current := cfg.Get().Ingest.Kafka
	if !current.Enabled {
		if logger != nil {
			logger.Info("kafka ingest disabled")
		}
		return
	}
	if logger != nil {
		logger.Info("kafka ingest enabled", "brokers", current.Brokers, "topic", current.Topic, "group_id", current.GroupID)
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  current.Brokers,
		Topic:    current.Topic,
		GroupID:  current.GroupID,
		MinBytes: 1e3,
		MaxBytes: 10e6,
	})
	go func() {
		defer reader.Close()
		for {
			m, err := reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if logger != nil {
					logger.Warn("kafka read error", "err", err)
				}
				continue
			}
			line := string(m.Value)
			fields, err := parser.ParseLine(line)
			if err != nil || fields == nil {
				continue
			}
			ev, err := normalize.Normalize(*fields, cfg.Get())
			if err != nil {
				if logger != nil {
					logger.Warn("kafka normalize error", "err", err)
				}
				continue
			}
			ev.Source = "kafka"
			SendNonBlocking(ctx, out, ev, logger)
		}
	}()
}
