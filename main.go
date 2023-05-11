package main

import (
	"Gotest/MinIO_webhook/k8s.io/klog"
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

type TypeMeta struct {
	// +optional
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty" protobuf:"bytes,1,opt,name=apiVersion"`
	// +optional
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty" protobuf:"bytes,2,opt,name=kind"`
}

type Unknown struct {
	TypeMeta `json:",inline" protobuf:"bytes,1,opt,name=typeMeta"`
	// Raw will hold the complete serialized object which couldn't be matched
	// with a registered type. Most likely, nothing should be done with this
	// except for passing it through the system.
	Raw []byte `protobuf:"bytes,2,opt,name=raw"`
	// ContentEncoding is encoding used to encode 'Raw' data.
	// Unspecified means no encoding.
	ContentEncoding string `protobuf:"bytes,3,opt,name=contentEncoding"`
	// ContentType  is serialization method used to serialize 'Raw'.
	// Unspecified means ContentTypeJSON.
	ContentType string `protobuf:"bytes,4,opt,name=contentType"`
}

type Backend struct {
	url              string
	senderCh         chan interface{}
	client           http.Client
	sendTimeout      time.Duration
	getSenderTimeout time.Duration
	stopCh           <-chan struct{}
}
type MicroTime struct {
	time.Time `protobuf:"-"`
}

type Events struct {

	// AuditLevel at which event was generated
	Level string

	// Unique audit ID, generated for each request.
	AuditID string
	// Stage of the request handling when this event instance was generated.
	Stage string

	// RequestURI is the request URI as sent by the client to a server.
	RequestURI string
	// Verb is the kubernetes verb associated with the request.
	// For non-resource requests, this is the lower-cased HTTP method.
	Verb string
	// Authenticated user information.
	User string

	ImpersonatedUser string

	SourceIPs []string

	UserAgent string

	ObjectRef *ObjectReference

	ResponseStatus string

	RequestObject Unknown

	ResponseObject Unknown

	RequestReceivedTimestamp MicroTime

	StageTimestamp MicroTime

	Annotations map[string]string
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

	Events
}

type EventList struct {
	Items []Event
}

type ObjectReference struct {
	// +optional
	Resource string
	// +optional
	Namespace string
	// +optional
	Name string
	// +optional
	UID string
	// APIGroup is the name of the API group that contains the referred object.
	// The empty string represents the core API group.
	// +optional
	APIGroup string
	// APIVersion is the version of the API group that contains the referred object.
	// +optional
	APIVersion string
	// +optional
	ResourceVersion string
	// +optional
	Subresource string
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
		fmt.Println("finish")
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
		Devops:    "DevOps",
		Workspace: "Workspace",
		Cluster:   "Cluster",
		Events: Events{
			RequestURI:               "Path",
			Verb:                     "",
			Level:                    "None",
			AuditID:                  "123456",
			Stage:                    "ResponseComplete",
			ImpersonatedUser:         "nil",
			UserAgent:                "User-Agent",
			RequestReceivedTimestamp: MicroTime{},
			Annotations:              nil,
			ObjectRef: &ObjectReference{
				Resource:        "users",
				Namespace:       "",
				Name:            "",
				UID:             "",
				APIGroup:        "",
				APIVersion:      "",
				ResourceVersion: "",
				Subresource:     "",
			},
		},
	}

	myEventList := EventList{
		Items: []Event{myEvent},
	}

	stopCh := make(chan struct{})
	backend := NewBackend(stopCh)

	backend.sendEvents(myEventList)
}
