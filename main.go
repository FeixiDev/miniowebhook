package main

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
