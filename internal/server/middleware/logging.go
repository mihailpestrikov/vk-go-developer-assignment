package middleware

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func UnaryLoggerInterceptor(logger zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		requestID := uuid.New().String()
		reqLogger := logger.With().
			Str("request_id", requestID).
			Str("method", info.FullMethod).
			Logger()

		reqLogger.Info().Msg("Unary request started")

		resp, err := handler(ctx, req)

		latency := time.Since(start)

		if err != nil {
			st, _ := status.FromError(err)
			reqLogger.Error().
				Str("code", st.Code().String()).
				Str("error", st.Message()).
				Dur("latency_ms", latency).
				Msg("Unary request failed")
		} else {
			reqLogger.Info().
				Str("code", "OK").
				Dur("latency_ms", latency).
				Msg("Unary request completed")
		}

		return resp, err
	}
}

func StreamLoggerInterceptor(logger zerolog.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		requestID := uuid.New().String()
		reqLogger := logger.With().
			Str("request_id", requestID).
			Str("method", info.FullMethod).
			Bool("client_stream", info.IsClientStream).
			Bool("server_stream", info.IsServerStream).
			Logger()

		reqLogger.Info().Msg("Stream request started")

		wrappedStream := &loggingServerStream{
			ServerStream: ss,
			logger:       reqLogger,
		}

		err := handler(srv, wrappedStream)

		latency := time.Since(start)

		if err != nil {
			st, _ := status.FromError(err)
			reqLogger.Error().
				Str("code", st.Code().String()).
				Str("error", st.Message()).
				Dur("latency_ms", latency).
				Msg("Stream request failed")
		} else {
			reqLogger.Info().
				Str("code", "OK").
				Dur("latency_ms", latency).
				Msg("Stream request completed")
		}

		return err
	}
}

type loggingServerStream struct {
	grpc.ServerStream
	logger zerolog.Logger
}

func (s *loggingServerStream) SendMsg(m interface{}) error {
	err := s.ServerStream.SendMsg(m)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send message")
		return err
	}

	s.logger.Debug().Msg("Message sent to client")
	return nil
}

func (s *loggingServerStream) RecvMsg(m interface{}) error {
	err := s.ServerStream.RecvMsg(m)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to receive message")
		return err
	}

	s.logger.Debug().Msg("Message received from client")
	return nil
}
