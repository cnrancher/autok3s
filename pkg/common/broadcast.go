package common

import "sync"

type (
	// Subscriber interface chan.
	Subscriber     chan interface{}
	subscriberFunc func(v interface{}) bool
)

// Broadcaster sends events to multiple subscribers.
type Broadcaster struct {
	subs map[Subscriber]subscriberFunc
	m    sync.RWMutex
}

// NewBroadcaster returns new broadcaster struct.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subs: make(map[Subscriber]subscriberFunc),
	}
}

// Register helps init subscriber with specified subscribe function.
func (b *Broadcaster) Register(sf subscriberFunc) Subscriber {
	ch := make(Subscriber)
	b.m.Lock()
	b.subs[ch] = sf
	b.m.Unlock()

	return ch
}

// Evict specified subscriber.
func (b *Broadcaster) Evict(s Subscriber) {
	b.m.Lock()
	defer b.m.Unlock()

	delete(b.subs, s)
	close(s)
}

// Broadcast events to each subscriber.
func (b *Broadcaster) Broadcast(v interface{}) {
	b.m.Lock()
	defer b.m.Unlock()

	var wg sync.WaitGroup
	for s, sf := range b.subs {
		wg.Add(1)
		go b.publish(s, sf, v, &wg)
	}
	wg.Wait()
}

// publish event message (which is filtered by subscribe function) to subscriber.
func (b *Broadcaster) publish(s Subscriber, sf subscriberFunc, v interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

	// skip not fit data
	if sf != nil && !sf(v) {
		return
	}
	// select {
	// case s <- v:
	// }
	s <- v
}

// Close all subscribers.
func (b *Broadcaster) Close() {
	b.m.Lock()
	defer b.m.Unlock()

	for s := range b.subs {
		delete(b.subs, s)
		close(s)
	}
}
