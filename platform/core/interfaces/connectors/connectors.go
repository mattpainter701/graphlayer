package connectors

type IMessageBroker interface {
	Publish(topic string, message interface{}) error
	Subscribe(topic string, handler interface{}) error
}
