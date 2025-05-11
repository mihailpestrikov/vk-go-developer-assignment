package server

import (
	"context"
	"errors"
	"vk-go-developer-assignment/pkg/config"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "vk-go-developer-assignment/api/proto"
	"vk-go-developer-assignment/pkg/subpub"
)

type PubSubHandler struct {
	pb.UnimplementedPubSubServer
	sp     subpub.SubPub
	logger zerolog.Logger
	cfg    *config.Config
}

func NewPubSubHandler(sp subpub.SubPub, logger zerolog.Logger, cfg *config.Config) *PubSubHandler {
	return &PubSubHandler{
		sp:     sp,
		logger: logger,
		cfg:    cfg,
	}
}

func (h *PubSubHandler) Register(server *grpc.Server) {
	pb.RegisterPubSubServer(server, h)
}

func (h *PubSubHandler) Subscribe(req *pb.SubscribeRequest, stream grpc.ServerStreamingServer[pb.Event]) error {
	key := req.GetKey()
	log := h.logger.With().Str("key", key).Logger()

	log.Info().Msg("New subscription request")

	msgCh := make(chan interface{}, h.cfg.SubPub.BufferSize)
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	sub, err := h.sp.Subscribe(key, func(msg interface{}) {
		select {
		case msgCh <- msg:
		case <-ctx.Done():
			return
		default:
			log.Warn().Msg("Message channel full, dropping message")
		}
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to subscribe")
		return status.Error(codes.Internal, "failed to subscribe")
	}
	defer sub.Unsubscribe()

	log.Info().Msg("Subscription created")

	for {
		select {
		case msg := <-msgCh:
			data, ok := msg.(string)
			if !ok {
				log.Warn().
					Interface("msg", msg).
					Msg("Received non-string message, skipping")
				continue
			}

			event := &pb.Event{Data: data}
			if err := stream.Send(event); err != nil {
				log.Error().
					Err(err).
					Msg("Failed to send event to client")
				return status.Error(codes.Unavailable, "failed to send event")
			}

			log.Debug().
				Str("data", data).
				Msg("Event sent to client")

		case <-ctx.Done():
			log.Info().Msg("Client disconnected")
			return nil
		}
	}
}

func (h *PubSubHandler) Publish(ctx context.Context, req *pb.PublishRequest) (*emptypb.Empty, error) {
	key := req.GetKey()
	data := req.GetData()

	if key == "" {
		return nil, status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	log := h.logger.With().
		Str("key", key).
		Str("data", data).
		Logger()

	log.Info().Msg("Publishing message")

	err := h.sp.Publish(key, data)
	if err != nil {
		if errors.Is(err, subpub.ErrSubPubClosed) {
			log.Error().Err(err).Msg("SubPub system is closed")
			return nil, status.Error(codes.Unavailable, "service unavailable: system is closed")
		} else if errors.Is(err, subpub.ErrPublishFailed) {
			log.Error().Err(err).Msg("Failed to publish message: no active subscribers")
			return nil, status.Error(codes.NotFound, "no active subscribers for this key")
		} else {
			log.Error().Err(err).Msg("Failed to publish message: internal error")
			return nil, status.Error(codes.Internal, "internal error while publishing message")
		}
	}

	log.Info().Msg("Message published successfully")
	return &emptypb.Empty{}, nil
}
