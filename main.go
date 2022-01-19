package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type queue chan string

type QueueStorage struct {
	Queues map[string]queue
	sync.Mutex
}

func NewStorage() *QueueStorage {
	return &QueueStorage{
		Queues: make(map[string]queue),
	}
}

func writeMessage(res http.ResponseWriter, message string) {
	if _, err := io.WriteString(res, message); err != nil {
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (storage *QueueStorage) handleGet(res http.ResponseWriter, req *http.Request) {
	queueName := req.URL.Path

	queue, ok := storage.Queues[queueName]
	if !ok {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	timeoutStr := req.URL.Query().Get("timeout")

	if timeoutStr == "" {
		select {
		case message := <-queue:
			writeMessage(res, message)
		default:
			res.WriteHeader(http.StatusNotFound)
		}
	}

	if timeoutStr != "" {
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		select {
		case message := <-queue:
			writeMessage(res, message)
		case <-time.After(time.Duration(timeout) * time.Second):
			res.WriteHeader(http.StatusNotFound)
		}
	}
}

func (storage *QueueStorage) createQueue(queueName string) {
	storage.Lock()

	_, ok := storage.Queues[queueName]
	if !ok {
		storage.Queues[queueName] = make(queue)
	}

	storage.Unlock()
}

func (storage *QueueStorage) handlePut(res http.ResponseWriter, req *http.Request) {
	v := req.URL.Query().Get("v")

	if v == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	queueName := req.URL.Path

	storage.createQueue(queueName)

	go func() {
		queue := storage.Queues[queueName]
		queue <- v
	}()
}

func GetHandler() http.HandlerFunc {
	storage := NewStorage()

	return func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			storage.handleGet(res, req)
		case http.MethodPut:
			storage.handlePut(res, req)
		default:
			res.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func main() {

	// port flag
	var port int
	flag.IntVar(&port, "p", 8080, "set the listening port")

	http.HandleFunc("/", GetHandler())

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
