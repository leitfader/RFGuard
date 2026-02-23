package ingest

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"rfguard/internal/config"
	"rfguard/internal/model"
	"rfguard/internal/normalize"
)

func StartSyslog(ctx context.Context, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	current := cfg.Get().Ingest.Syslog
	if !current.Enabled {
		if logger != nil {
			logger.Info("syslog ingest disabled")
		}
		return
	}
	if logger != nil {
		logger.Info("syslog ingest enabled", "udp_addr", current.UDPAddr, "tcp_addr", current.TCPAddr)
	}
	if current.UDPAddr != "" {
		go listenUDP(ctx, current.UDPAddr, cfg, parser, out, logger)
	}
	if current.TCPAddr != "" {
		go listenTCP(ctx, current.TCPAddr, cfg, parser, out, logger)
	}
}

func listenUDP(ctx context.Context, addr string, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		if logger != nil {
			logger.Error("syslog udp resolve error", "err", err)
		}
		return
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		if logger != nil {
			logger.Error("syslog udp listen error", "err", err)
		}
		return
	}
	defer conn.Close()
	buf := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				if logger != nil {
					logger.Warn("syslog udp read error", "err", err)
				}
				continue
			}
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				processLine(ctx, cfg, parser, out, logger, line)
			}
		}
	}
}

func listenTCP(ctx context.Context, addr string, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if logger != nil {
			logger.Error("syslog tcp listen error", "err", err)
		}
		return
	}
	defer ln.Close()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if logger != nil {
				logger.Warn("syslog tcp accept error", "err", err)
			}
			continue
		}
		go handleTCPConn(ctx, conn, cfg, parser, out, logger)
	}
}

func handleTCPConn(ctx context.Context, conn net.Conn, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 8192), 1024*1024)
	for scanner.Scan() {
		processLine(ctx, cfg, parser, out, logger, scanner.Text())
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
	if err := scanner.Err(); err != nil && logger != nil {
		logger.Warn("syslog tcp scanner error", "err", err)
	}
}

func processLine(ctx context.Context, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger, line string) {
	fields, err := parser.ParseLine(line)
	if err != nil || fields == nil {
		return
	}
	ev, err := normalize.Normalize(*fields, cfg.Get())
	if err != nil {
		if logger != nil {
			logger.Warn("syslog normalize error", "err", err)
		}
		return
	}
	ev.Source = "syslog"
	SendNonBlocking(ctx, out, ev, logger)
}
