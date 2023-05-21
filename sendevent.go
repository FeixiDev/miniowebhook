package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"k8s.io/klog"
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

type MicroTime struct {
	time.Time `protobuf:"-"`
}

type Event struct {
	Devops                   string
	Workspace                string
	Cluster                  string
	Message                  string
	Level                    string
	AuditID                  string
	Stage                    string
	RequestURI               string
	Verb                     string
	User                     User
	ImpersonatedUser         interface{}
	SourceIPs                []string
	UserAgent                string
	ObjectRef                ObjectRef
	ResponseStatus           ResponseStatus
	RequestObject            interface{}
	ResponseObject           interface{}
	RequestReceivedTimestamp MicroTime
	StageTimestamp           MicroTime
	Annotations              interface{}
}

type User struct {
	username string
	groups   []string
}

type ObjectRef struct {
	Resource        string
	Namespace       string
	Name            string
	UID             string
	APIGroup        string
	APIVersion      string
	ResourceVersion string
	Subresource     string
}

type ResponseStatus struct {
	Code     int
	Metadata map[string]interface{}
	reason   string
	status   string
}

type EventList struct {
	Items []Event
}

func NowMicro() MicroTime {
	return MicroTime{time.Now()}
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

func ProcessJSONData(jsonData []byte) (*Event, error) {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, err
	}

	apiData, ok := data["api"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("api key not found or not a map[string]interface{}")
	}

	name, ok := apiData["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name key not found or not a string")
	}

	version, ok := data["version"].(string)
	if !ok {
		return nil, fmt.Errorf("version key not found or not a string")
	}

	timeValue, ok := data["time"].(string)
	if !ok {
		return nil, fmt.Errorf("time key not found or not a string")
	}

	parentUser, ok := data["parentUser"].(string)
	if !ok {
		return nil, fmt.Errorf("parentUser key not found or not a string")
	}

	parsedTime, err := time.Parse(time.RFC3339, timeValue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time: %v", err)
	}

	validNames := []string{"PutObject", "DeleteMultipleObjects", "PutBucket", "DeleteBucket", "SiteReplicationInfo"}
	isValidName := false
	for _, validName := range validNames {
		if name == validName {
			isValidName = true
			break
		}
	}

	if !isValidName {
		return nil, nil // 当 name 不是有效值时，返回 nil 而不是错误
	}

	event := Event{
		ObjectRef: ObjectRef{
			Name:       name,
			APIVersion: version,
		},
		RequestReceivedTimestamp: MicroTime{parsedTime},
		StageTimestamp:           MicroTime{parsedTime},
		User: User{
			username: parentUser,
		},
		ResponseStatus: ResponseStatus{
			Code:     200,
			Metadata: make(map[string]interface{}),
			reason:   "",
			status:   "INFO",
		},
		AuditID: uuid.New().String(),
	}
	return &event, nil
}
