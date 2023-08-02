package publisher

import (
	pb "github.com/lyonsoftworks/evvent/gen/api/v1/events"
)

type EventPublisher interface {
	PublishToChannel(string, *pb.Event) error
	Broadcast(*pb.Event) error
	SubscribeToChannel(string, chan *pb.Event)
	UnsubscribeFromChannel(string, chan *pb.Event)
}
