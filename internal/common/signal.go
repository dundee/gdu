package common

type SignalGroup chan struct{}

func (s SignalGroup) Wait() {
	<-s
}

func (s SignalGroup) Broadcast() {
	close(s)
}
