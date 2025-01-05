package logger

import "log/slog"

func SlogErr(err error) slog.Attr {
	return slog.String("err", err.Error())
}
