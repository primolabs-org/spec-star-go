package logging

import (
    "log/slog"
    "os"
)

func NewJSONLogger() *slog.Logger {
    return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
