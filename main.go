package main

import (
	"Gotest/k8s.io/klog"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	GetSenderTimeout = time.Second
	SendTimeout      = time.Second * 3
	WebhookURL       = "https://10.203.1.23:30278/audit/webhook/event"
)

type Backend struct {
	url              string
	senderCh         chan interface{}
	client           http.Client
	sendTimeout      time.Duration
	getSenderTimeout time.Duration
	stopCh           <-chan struct{}
}

type Event struct {
	// Devops project
	Devops string
	// The workspace which this audit event happened
	Workspace string
	// The cluster which this audit event happened
	Cluster string
	// Message send to user.
	Message string
	Factory string
}

type EventList struct {
	Items []Event
}

func NewBackend(stopCh <-chan struct{}) *Backend {

	b := Backend{
		url:              WebhookURL,
		getSenderTimeout: GetSenderTimeout,
		sendTimeout:      SendTimeout,
		stopCh:           stopCh,
	}
	b.senderCh = make(chan interface{}, 100)

	b.client = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: b.sendTimeout,
	}

	fmt.Println("ended NewBackend.......")
	go b.worker()

	return &b
}

func (b *Backend) worker() {
	fmt.Println("start b worker.......")

	for {
		events := EventList{}

		go b.sendEvents(events)
	}
}

func (b *Backend) sendEvents(events EventList) {

	ctx, cancel := context.WithTimeout(context.Background(), b.sendTimeout)
	defer cancel()

	stopCh := make(chan struct{})

	send := func() {
		ctx, cancel := context.WithTimeout(context.Background(), b.getSenderTimeout)
		defer cancel()

		select {
		case <-ctx.Done():
			klog.Error("Get auditing event sender timeout")
			return
		case b.senderCh <- struct{}{}:
		}

		start := time.Now()
		defer func() {
			stopCh <- struct{}{}
			klog.V(8).Infof("send %d auditing logs used %d", len(events.Items), time.Since(start).Milliseconds())
		}()

		bs, err := b.eventToBytes(events)
		if err != nil {
			klog.Errorf("json marshal error, %s", err)
			return
		}

		klog.V(8).Infof("%s", string(bs))

		response, err := b.client.Post(b.url, "application/json", bytes.NewBuffer(bs))
		if err != nil {
			klog.Errorf("send audit events error, %s", err)
			return
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			klog.Errorf("send audit events error[%d]", response.StatusCode)
			return
		}
	}

	go send()

	defer func() {
		<-b.senderCh
	}()

	select {
	case <-ctx.Done():
		klog.Error("send audit events timeout")
	case <-stopCh:
	}
}

func (b *Backend) eventToBytes(event EventList) ([]byte, error) {

	bs, err := json.Marshal(event)
	return bs, err
}

func main() {
	// 创建一个Event实例
	myEvent := Event{
		Devops:    "MyDevopsProject",
		Workspace: "MyWorkspace",
		Cluster:   "MyCluster",
		Message:   "MyMessage",
		Factory:   "MyFactory",
	}

	myEventList := EventList{
		Items: []Event{myEvent},
	}

	stopCh := make(chan struct{})
	backend := NewBackend(stopCh)

	backend.sendEvents(myEventList)
}
