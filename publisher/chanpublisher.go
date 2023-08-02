package publisher

import (
	"sync"

	pb "github.com/lyonsoftworks/evvent/gen/api/v1/events"
	"go.uber.org/zap"
)

var logger *zap.Logger

// ChannelSubscriber represents a subscriber to a channel.
type ChannelSubscriber struct {
	channel chan *pb.Event
}

// ChannelEventPublisher is an implementation of the EventPublisher interface.
type ChannelEventPublisher struct {
	subscribersMap map[string][]*ChannelSubscriber
	mutex          sync.RWMutex
}

// NewChannelEventPublisher creates a new instance of ChannelEventPublisher.
func NewChannelEventPublisher() *ChannelEventPublisher {
	logger, _ = zap.NewDevelopment()

	return &ChannelEventPublisher{
		subscribersMap: make(map[string][]*ChannelSubscriber),
	}
}

// PublishToChannel publishes an event to a specific channel.
func (p *ChannelEventPublisher) PublishToChannel(channel string, event *pb.Event) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	subscribers, ok := p.subscribersMap[channel]
	if !ok {
		logger.Info("Channel has no subscribers", zap.String("channel", channel))
		return nil
	}

	for _, subscriber := range subscribers {
		select {
		case subscriber.channel <- event:
			logger.Info("Sent event to channel", zap.String("channel", channel), zap.String("event_id", event.Id))
		default:
			logger.Error("Error publishing to channel", zap.String("channel", channel), zap.String("event_id", event.Id))
			// The channel is full or closed, drop the event or handle it accordingly.
			// For simplicity, we will just ignore it here.
		}
	}

	return nil
}

// Broadcast broadcasts an event to all subscribed channels.
func (p *ChannelEventPublisher) Broadcast(event *pb.Event) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, subscribers := range p.subscribersMap {
		for _, subscriber := range subscribers {
			select {
			case subscriber.channel <- event:
			default:
				logger.Error("Error broadcasting", zap.String("event_id", event.Id))
				// The channel is full or closed, drop the event or handle it accordingly.
				// For simplicity, we will just ignore it here.
			}
		}
	}

	return nil
}

// SubscribeToChannel subscribes to a specific channel.
func (p *ChannelEventPublisher) SubscribeToChannel(channel string, ch chan *pb.Event) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	subscribers, ok := p.subscribersMap[channel]
	if !ok {
		subscribers = make([]*ChannelSubscriber, 0)
	}
	subscribers = append(subscribers, &ChannelSubscriber{channel: ch})
	p.subscribersMap[channel] = subscribers
}

// UnsubscribeFromChannel unsubscribes from a specific channel.
func (p *ChannelEventPublisher) UnsubscribeFromChannel(channel string, ch chan *pb.Event) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	subscribers, ok := p.subscribersMap[channel]
	if !ok {
		return
	}

	for i, subscriber := range subscribers {
		if subscriber.channel == ch {
			subscribers = append(subscribers[:i], subscribers[i+1:]...)
			close(subscriber.channel)
			break
		}
	}

	// Update the subscribers list for the channel.
	p.subscribersMap[channel] = subscribers
}
