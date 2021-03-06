package amqp

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/influxdb/influxdb/client"
	"github.com/influxdb/telegraf/outputs"
	"github.com/streadway/amqp"
)

type AMQP struct {
	// AMQP brokers to send metrics to
	URL string
	// AMQP exchange
	Exchange string
	// Routing Key Tag
	RoutingTag string `toml:"routing_tag"`

	channel *amqp.Channel
	sync.Mutex
}

var sampleConfig = `
	# AMQP url
	url = "amqp://localhost:5672/influxdb"
	# AMQP exchange
	exchange = "telegraf"
	# Telegraf tag to use as a routing key
	#  ie, if this tag exists, it's value will be used as the routing key
	routing_tag = "host"
`

func (q *AMQP) Connect() error {
	q.Lock()
	defer q.Unlock()
	connection, err := amqp.Dial(q.URL)
	if err != nil {
		return err
	}
	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("Failed to open a channel: %s", err)
	}

	err = channel.ExchangeDeclare(
		q.Exchange, // name
		"topic",    // type
		true,       // durable
		false,      // delete when unused
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("Failed to declare an exchange: %s", err)
	}
	q.channel = channel
	go func() {
		log.Printf("Closing: %s", <-connection.NotifyClose(make(chan *amqp.Error)))
		log.Printf("Trying to reconnect")
		for err := q.Connect(); err != nil; err = q.Connect() {
			log.Println(err)
			time.Sleep(10 * time.Second)
		}

	}()
	return nil
}

func (q *AMQP) Close() error {
	return q.channel.Close()
}

func (q *AMQP) SampleConfig() string {
	return sampleConfig
}

func (q *AMQP) Description() string {
	return "Configuration for the AMQP server to send metrics to"
}

func (q *AMQP) Write(bp client.BatchPoints) error {
	q.Lock()
	defer q.Unlock()
	if len(bp.Points) == 0 {
		return nil
	}

	var zero_time time.Time
	for _, p := range bp.Points {
		// Combine tags from Point and BatchPoints and grab the resulting
		// line-protocol output string to write to AMQP
		var value, key string
		if p.Raw != "" {
			value = p.Raw
		} else {
			for k, v := range bp.Tags {
				if p.Tags == nil {
					p.Tags = make(map[string]string, len(bp.Tags))
				}
				p.Tags[k] = v
			}
			if p.Time == zero_time {
				if bp.Time == zero_time {
					p.Time = time.Now()
				} else {
					p.Time = bp.Time
				}
			}
			value = p.MarshalString()
		}

		if q.RoutingTag != "" {
			if h, ok := p.Tags[q.RoutingTag]; ok {
				key = h
			}
		}

		err := q.channel.Publish(
			q.Exchange, // exchange
			key,        // routing key
			false,      // mandatory
			false,      // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(value),
			})
		if err != nil {
			return fmt.Errorf("FAILED to send amqp message: %s", err)
		}
	}
	return nil
}

func init() {
	outputs.Add("amqp", func() outputs.Output {
		return &AMQP{}
	})
}
