#!/bin/bash
cat internal/telegram/handlers/chore.go | grep 'slog\.Warn'
cat internal/telegram/handlers/chore.go | grep -C 2 'math/rand'
