package main

//import (
//	"github.com/minio/pkg/env"
//)
//
//var (
//	logFile   string
//	address   string
//	authToken = env.Get("WEBHOOK_AUTH_TOKEN", "")
//)

//func processJSONData(jsonData map[string]interface{}) {
//
//	data, err := json.Marshal(jsonData)
//	if err != nil {
//		log.Println("Error marshaling jsonData:", err)
//		return
//	}
//
//	ProcessJSONData(data)
//
//	//fmt.Println(event)
//
//	//stopCh := make(chan struct{})
//	//backend := NewBackend(stopCh)
//	//events := EventList{
//	//	Items: []Event{*event},
//	//}
//	//
//	//backend.sendEvents(events)
//}

//func main() {
//	flag.StringVar(&logFile, "log-file", "", "path to the file where webhook will log incoming events")
//	flag.StringVar(&address, "address", ":8080", "bind to a specific ADDRESS:PORT, ADDRESS can be an IP or hostname")
//
//	flag.Parse()
//
//	var mu sync.Mutex
//
//	sigs := make(chan os.Signal, 1)
//	signal.Notify(sigs, syscall.SIGHUP)
//
//	go func() {
//		for _ = range sigs {
//			mu.Lock()
//			mu.Unlock()
//		}
//	}()
//
//	err := http.ListenAndServe(address, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		if authToken != "" {
//			if authToken != r.Header.Get("Authorization") {
//				http.Error(w, "authorization header missing", http.StatusBadRequest)
//				return
//			}
//		}
//		switch r.Method {
//		case http.MethodPost:
//			mu.Lock()
//			data, err := ioutil.ReadAll(r.Body)
//			if err != nil {
//				mu.Unlock()
//				return
//			}
//
//			var jsonData map[string]interface{}
//			err = json.Unmarshal(data, &jsonData)
//			if err != nil {
//				mu.Unlock()
//				return
//			}
//
//			processJSONData(jsonData)
//
//			mu.Unlock()
//		default:
//		}
//	}))
//	if err != nil {
//		log.Fatal(err)
//	}
//}

func main() {
	myEvent := Event{
		Devops:     "",
		Workspace:  "",
		Cluster:    "",
		Message:    "",
		Level:      "",
		AuditID:    "shiki2034-fc3b-46a5-1113-89066c3ad423",
		Stage:      "ResponseComplete",
		RequestURI: "",
		Verb:       "",
		User: User{
			username: "admin",

			groups: []string{"system:authenticated"},
		},
		ImpersonatedUser: nil,
		SourceIPs:        []string{"10.233.103.183"},
		UserAgent:        "MinIO (linux; amd64) minio-go/v7.0.52 MinIO Console/(dev)",
		ObjectRef: ObjectRef{
			Resource:        "MinIO",
			Namespace:       "",
			Name:            "PutObject",
			UID:             "",
			APIGroup:        "",
			APIVersion:      "",
			ResourceVersion: "",
			Subresource:     "",
		},
		ResponseStatus: ResponseStatus{
			Code:     200,
			Metadata: make(map[string]interface{}),
			reason:   "upload",
			status:   "INFO",
		},

		RequestObject:            nil,
		ResponseObject:           nil,
		RequestReceivedTimestamp: "2023-05-17T03:32:47.394877Z",
		StageTimestamp:           "2023-05-17T03:32:47.394877Z",
		Annotations:              nil,
	}

	myEventList := EventList{
		Items: []Event{myEvent},
	}

	stopCh := make(chan struct{})
	backend := NewBackend(stopCh)
	backend.sendEvents(myEventList)
}
