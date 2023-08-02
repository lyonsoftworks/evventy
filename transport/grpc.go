package transport

import (
	"net/http"

	"github.com/google/uuid"
	pb "github.com/lyonsoftworks/evvent/gen/api/v1/events"
	"github.com/lyonsoftworks/evvent/publisher"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var logger *zap.Logger

// Define Prometheus metrics
var (
	successfulPublishes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evvent_successful_publishes",
		Help: "Total number of successful event publishes",
	})
	subscriptions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evvent_subscriptions",
		Help: "Total number of subscriptions",
	})
	failedPublishes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evvent_failed_publishes",
		Help: "Total number of failed event publishes",
	})
)

func Initialize() {
	logger, _ = zap.NewDevelopment()
}

type EventServer struct {
	pb.UnimplementedEventServiceServer
	publisher publisher.EventPublisher
}

func NewEventServer() *EventServer {
	pub := publisher.NewChannelEventPublisher()

	return &EventServer{
		publisher: pub,
	}
}

func (s *EventServer) PublishEvent(req *pb.PublishEventRequest, stream pb.EventService_PublishEventServer) error {
	evt := req.Event
	to := req.Channel
	evt.Id = uuid.New().String()

	if to != nil {
		logger.Sugar().Infof("Sending event %+v to channel %s", evt, *to)
		err := s.publisher.PublishToChannel(*to, evt)
		if err != nil {
			logger.Error(err.Error(), zap.String("event_id", evt.Id))
			failedPublishes.Inc() // Increment the failed publishes metric
			return status.Errorf(codes.Internal, "Error publishing to channel %s %s", *req.Channel, err.Error())
		}
	} else {
		logger.Sugar().Infof("Broadcasting event %+v to all channels")
		err := s.publisher.Broadcast(evt)
		if err != nil {
			logger.Error(err.Error(), zap.String("event_id", evt.Id))
			failedPublishes.Inc() // Increment the failed publishes metric
			return status.Errorf(codes.Internal, "Error broadcasting %s", err.Error())
		}
	}

	successfulPublishes.Inc() // Increment the successful publishes metric

	stream.Send(&pb.PublishEventResponse{
		EventId:     evt.Id,
		PublishTime: timestamppb.Now(),
	})

	return nil
}

func (s *EventServer) Subscribe(req *pb.SubscribeRequest, stream pb.EventService_SubscribeServer) error {
	ch := make(chan *pb.Event)
	ctx := stream.Context()
	s.publisher.SubscribeToChannel(req.ChannelId, ch)

	subscriptions.Inc() // increment subscriptions metric

	for {
		done := false
		select {
		case <-ctx.Done():
			done = true
			break
		case val := <-ch:
			err := stream.Send(val)
			if err != nil {
				logger.Error(err.Error())
				done = true
			}
		}
		if done {
			break
		}
	}
	s.publisher.UnsubscribeFromChannel(req.ChannelId, ch)

	return nil
}

// Function to register Prometheus metrics handler for HTTP server
func RegisterPrometheusHandler() {
	http.Handle("/metrics", promhttp.Handler())
}
