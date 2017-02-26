// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_logrus

import (
	"path"
	"time"

	"github.com/mwitkow/go-grpc-middleware"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/Sirupsen/logrus"
)

var (
	// SystemField is used in every log statement made through grpc_logrus. Can be overwritten before any initialization code.
	SystemField = "system"
)

// UnaryServerInterceptor returns a new unary server interceptors that adds logrus.Entry to the context.
func UnaryServerInterceptor(entry *logrus.Entry, opts ...Option) grpc.UnaryServerInterceptor {
	o := evaluateOptions(opts)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		newCtx := newLoggerForCall(ctx, entry, info.FullMethod)

		startTime := time.Now()
		resp, err := handler(newCtx, req)
		fields := logrus.Fields{
			"grpc_code":    grpc.Code(err).String(),
			"grpc_time_ns": time.Now().Sub(startTime),
		}
		if err != nil {
			fields[logrus.ErrorKey] = err
		}
		levelLogf(
			Extract(newCtx).WithFields(fields), // re-extract logger from newCtx, as it may have extra fields that changed in the holder.
			o.levelFunc(grpc.Code(err)),
			"finished unary call")
		return resp, err
	}
}

// StreamServerInterceptor returns a new streaming server interceptor that adds logrus.Entry to the context.
func StreamServerInterceptor(entry *logrus.Entry, opts ...Option) grpc.StreamServerInterceptor {
	o := evaluateOptions(opts)
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx := newLoggerForCall(stream.Context(), entry, info.FullMethod)
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx

		startTime := time.Now()
		err := handler(srv, wrapped)
		fields := logrus.Fields{
			"grpc_code":    grpc.Code(err).String(),
			"grpc_time_ns": time.Now().Sub(startTime),
		}
		if err != nil {
			fields[logrus.ErrorKey] = err
		}
		levelLogf(
			Extract(newCtx).WithFields(fields), // re-extract logger from newCtx, as it may have extra fields that changed in the holder.
			o.levelFunc(grpc.Code(err)),
			"finished streaming call")
		return err
	}
}

func levelLogf(entry *logrus.Entry, level logrus.Level, format string, args ...interface{}) {
	switch level {
	case logrus.DebugLevel:
		entry.Debugf(format, args...)
	case logrus.InfoLevel:
		entry.Infof(format, args...)
	case logrus.WarnLevel:
		entry.Warningf(format, args...)
	case logrus.ErrorLevel:
		entry.Errorf(format, args...)
	case logrus.FatalLevel:
		entry.Fatalf(format, args...)
	case logrus.PanicLevel:
		entry.Panicf(format, args...)
	}
}

func newLoggerForCall(ctx context.Context, entry *logrus.Entry, fullMethodString string) context.Context {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)
	callLog := entry.WithFields(
		logrus.Fields{
			SystemField:    "grpc",
			"grpc_service": service,
			"grpc_method":  method,
		})
	return toContext(ctx, callLog)
}