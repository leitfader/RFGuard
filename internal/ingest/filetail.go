package ingest

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"rfguard/internal/config"
	"rfguard/internal/model"
	"rfguard/internal/normalize"
)

func StartFileTail(ctx context.Context, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	current := cfg.Get().Ingest.FileTail
	if !current.Enabled {
		if logger != nil {
			logger.Info("file tail ingest disabled")
		}
		return
	}
	for _, path := range current.Files {
		path := path
		if logger != nil {
			logger.Info("file tail ingest enabled", "path", path, "start_at_end", current.StartAtEnd)
		}
		go tailFile(ctx, path, current.StartAtEnd, cfg, parser, out, logger)
	}
}

func tailFile(ctx context.Context, path string, startAtEnd bool, cfg *config.Manager, parser *Parser, out chan<- model.NormalizedEvent, logger *slog.Logger) {
	var file *os.File
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if file == nil {
			f, err := os.Open(path)
			if err != nil {
				if logger != nil {
					logger.Warn("tail open failed", "path", path, "err", err)
				}
				if !BackoffSleep(ctx, 500*time.Millisecond) {
					return
				}
				continue
			}
			file = f
			if startAtEnd {
				if pos, err := file.Seek(0, io.SeekEnd); err == nil {
					offset = pos
				}
			}
		}

		reader := bufio.NewReader(file)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if !BackoffSleep(ctx, 200*time.Millisecond) {
						_ = file.Close()
						return
					}
					info, statErr := os.Stat(path)
					if statErr == nil && info.Size() < offset {
						_ = file.Close()
						file = nil
						break
					}
					continue
				}
				if logger != nil {
					logger.Warn("tail read error", "path", path, "err", err)
				}
				_ = file.Close()
				file = nil
				break
			}
			offset += int64(len(line))
			fields, err := parser.ParseLine(line)
			if err != nil || fields == nil {
				continue
			}
			ev, err := normalize.Normalize(*fields, cfg.Get())
			if err != nil {
				if logger != nil {
					logger.Warn("tail normalize error", "err", err)
				}
				continue
			}
			ev.Source = "file_tail"
			SendNonBlocking(ctx, out, ev, logger)
		}
	}
}
