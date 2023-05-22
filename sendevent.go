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

//type MicroTime struct {
//	time.Time `protobuf:"-"`
//}

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
	RequestReceivedTimestamp string
	StageTimestamp           string
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

//func NowMicro() MicroTime {
//	return MicroTime{time.Now()}
//}

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

	return &b
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

func ProcessJSONData(jsonData []byte) {
	var data map[string]interface{}
	json.Unmarshal(jsonData, &data)

	apiData := data["api"].(map[string]interface{})

	name := apiData["name"].(string)

	bucket, ok := apiData["bucket"].(string)
	if !ok {
		bucket = ""
	}

	object, ok := apiData["object"].(string)
	if !ok {
		object = ""
	}

	version := data["version"].(string)

	timeValue := data["time"].(string)

	parentUser := data["parentUser"].(string)

	userAgent := data["userAgent"].(string)

	//parsedTime, err := time.Parse(time.RFC3339, timeValue)
	//fmt.Println(err)

	validNames := []string{"PutObject", "DeleteMultipleObjects", "PutBucket", "DeleteBucket", "SiteReplicationInfo"}
	for _, validName := range validNames {
		if name == validName {
			event := Event{
				Devops:     "",
				Workspace:  "",
				Cluster:    "",
				Message:    "",
				Level:      "",
				AuditID:    uuid.New().String(),
				Stage:      "",
				RequestURI: "",
				Verb:       "",
				User: User{
					username: parentUser,

					groups: []string{},
				},
				ImpersonatedUser: nil,
				SourceIPs:        []string{},
				UserAgent:        userAgent,
				ObjectRef: ObjectRef{
					Resource:        "MinIO",
					Namespace:       "",
					Name:            name,
					UID:             "",
					APIGroup:        "",
					APIVersion:      version,
					ResourceVersion: "",
					Subresource:     "",
				},
				ResponseStatus: ResponseStatus{
					Code:     200,
					Metadata: make(map[string]interface{}),
					status:   "INFO",
				},

				RequestObject:            nil,
				ResponseObject:           nil,
				RequestReceivedTimestamp: timeValue[:len(timeValue)-4] + timeValue[len(timeValue)-1:],
				StageTimestamp:           timeValue[:len(timeValue)-4] + timeValue[len(timeValue)-1:],
				Annotations:              nil,
			}
			if name == "PutObject" {
				event.ResponseStatus.reason = "Upload" + object + "to the bucket" + bucket
			} else if name == "DeleteMultipleObjects" {
				event.ResponseStatus.reason = "Delete" + object + "from the bucket" + bucket
			} else if name == "PutBucket" {
				event.ResponseStatus.reason = "Create the bucket" + bucket
			} else if name == "DeleteBucket" {
				event.ResponseStatus.reason = "Delete the bucket" + bucket
			} else if name == "SiteReplicationInfo" {
				event.ResponseStatus.reason = "Login"
			}
			fmt.Println(event)

			events := EventList{
				Items: []Event{event},
			}
			stopCh := make(chan struct{})
			backend := NewBackend(stopCh)

			backend.sendEvents(events)
		}
	}

}
