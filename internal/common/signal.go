// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

type SignalGroup chan struct{}

func (s SignalGroup) Wait() {
	<-s
}

func (s SignalGroup) Broadcast() {
	close(s)
}
