package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"k8s.io/klog"
	"net"
	"net/http"
	"os"
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
	Username string
	Groups   []string
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
	Reason   string
	Status   string
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

func ProcessJSONData(jsonData []byte, sip string) {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error getting hostname:", err)
		return
	}

	addrs, err := net.LookupIP(hostname)
	if err != nil {
		fmt.Println("Error getting IP address:", err)
		return
	}

	var ip string
	for i, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			if i == 0 {
				ip = ipv4.String()
			} else {
				ip = ipv4.String()
				break
			}
		}
	}
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

	version, ok := data["version"].(string)
	if !ok {
		version = ""
	}

	timeValue, ok := data["time"].(string)
	if !ok {
		timeValue = ""
	}

	parentUser, ok := data["parentUser"].(string)
	if !ok {
		parentUser = ""
	}

	userAgent, ok := data["userAgent"].(string)
	if !ok {
		userAgent = ""
	}

	//parsedTime, err := time.Parse(time.RFC3339, timeValue)
	//fmt.Println(err)

	validNames := []string{"PutObject", "DeleteMultipleObjects", "PutBucket", "DeleteBucket", "SiteReplicationInfo"}
	for _, validName := range validNames {
		if name == validName {
			event := Event{
				Devops:     "",
				Workspace:  hostname + "(" + ip + ")",
				Cluster:    "",
				Message:    "",
				Level:      "",
				AuditID:    uuid.New().String(),
				Stage:      "",
				RequestURI: "",
				Verb:       "",
				User: User{
					Username: parentUser,

					Groups: []string{},
				},
				ImpersonatedUser: nil,
				SourceIPs:        []string{sip},
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
					Status:   "INFO",
				},

				RequestObject:            nil,
				ResponseObject:           nil,
				RequestReceivedTimestamp: timeValue[:len(timeValue)-4] + timeValue[len(timeValue)-1:],
				StageTimestamp:           timeValue[:len(timeValue)-4] + timeValue[len(timeValue)-1:],
				Annotations:              nil,
			}
			if name == "PutObject" {
				event.ResponseStatus.Reason = "上传" + object + "到bucket" + "(" + bucket + ")"
			} else if name == "DeleteMultipleObjects" {
				event.ResponseStatus.Reason = "从bucket(" + bucket + " )" + "删除" + object
			} else if name == "PutBucket" {
				event.ResponseStatus.Reason = "创建bucket" + " " + bucket
			} else if name == "DeleteBucket" {
				event.ResponseStatus.Reason = "删除bucket" + " " + bucket
			} else if name == "SiteReplicationInfo" {
				event.ResponseStatus.Reason = "登录"
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
