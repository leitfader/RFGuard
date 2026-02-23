package ingest

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"

	"rfguard/internal/config"
	"rfguard/internal/model"
	"rfguard/internal/normalize"
)

func StartTCPStream(ctx context.Context, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	current := cfg.Get().Ingest.TCPStream
	if !current.Enabled {
		if logger != nil {
			logger.Info("tcp stream ingest disabled")
		}
		return
	}
	if logger != nil {
		logger.Info("tcp stream ingest enabled", "addr", current.Addr)
	}
	ln, err := net.Listen("tcp", current.Addr)
	if err != nil {
		if logger != nil {
			logger.Error("tcp stream listen error", "err", err)
		}
		return
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				if logger != nil {
					logger.Warn("tcp stream accept error", "err", err)
				}
				continue
			}
			go handleTCPStreamConn(ctx, conn, cfg, parser, out, logger)
		}
	}()
}

func handleTCPStreamConn(ctx context.Context, conn net.Conn, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 8192), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		fields, err := parser.ParseLine(line)
		if err != nil || fields == nil {
			continue
		}
		ev, err := normalize.Normalize(*fields, cfg.Get())
		if err != nil {
			if logger != nil {
				logger.Warn("tcp stream normalize error", "err", err)
			}
			continue
		}
		ev.Source = "tcp_stream"
		SendNonBlocking(ctx, out, ev, logger)
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
	if err := scanner.Err(); err != nil && logger != nil {
		logger.Warn("tcp stream scanner error", "err", err)
	}
}
