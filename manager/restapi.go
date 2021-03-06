package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
)

type restAPI struct {
	manager *manager
	router  *mux.Router
}

func startRESTAPI(bindAddr string, manager *manager) {

	a := restAPI{
		manager: manager,
	}

	a.setupRouter()

	log.Println("RESTAPI: Binding to", bindAddr)
	err := http.ListenAndServe(bindAddr, a.router)
	if err != nil {
		log.Fatal(err)
	}
}

func (a *restAPI) setupRouter() {
	r := mux.NewRouter()
	// targets
	r.HandleFunc("/targets/{id}/logs", a.GetTargetLogs).Methods("PUT")
	r.HandleFunc("/targets/{id}", a.GetTarget).Methods("GET")
	r.HandleFunc("/targets", a.GetTargets).Methods("GET")
	// tasks
	r.HandleFunc("/orders", a.GetOrders).Methods("GET")
	r.HandleFunc("/orders", a.AddOrder).Methods("POST")
	// static
	ui := http.Dir(WorkDir + "/ui")
	r.PathPrefix("/ui").Handler(http.StripPrefix("/ui", http.FileServer(ui)))
	r.PathPrefix("/ws").HandlerFunc(a.websocket)

	r.Use(loggingMiddleware)

	a.router = r
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
		log.Println(r.Method, r.URL, "responded in", time.Since(start))
	})
}

func (a *restAPI) newTaskID() string {
	return uuid.NewV4().String()
}

func (a *restAPI) AddOrder(w http.ResponseWriter, r *http.Request) {

	decoder := yaml.NewDecoder(r.Body)
	defer r.Body.Close()

	var order order
	err := decoder.Decode(&order)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	log.Println("Received order:", order)

	err = order.validate()
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, "Invalid order: ", err)
		return
	}

	// add system generated meta values
	order.ID = a.newTaskID()
	order.Created = time.Now().UnixNano()

	err = a.manager.addOrder(&order)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	// order is accepted
	b, err := json.Marshal(order) // marshal updated order
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	HTTPResponse(w, http.StatusAccepted, b)
	return
}

func (a *restAPI) GetOrders(w http.ResponseWriter, r *http.Request) {

	orders, err := a.manager.getOrders()
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(orders)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) GetTargets(w http.ResponseWriter, r *http.Request) {

	//var targets []*model.target
	//for _, t := range a.manager.targets {
	//	targets = append(targets, t)
	//}
	//// TODO sort by ID

	a.manager.RLock()
	defer a.manager.RUnlock()

	b, err := json.Marshal(a.manager.targets)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) GetTarget(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	a.manager.RLock()
	defer a.manager.RUnlock()

	target, found := a.manager.targets[id]
	if !found {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	b, err := json.Marshal(target)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) GetTargetLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	a.manager.RLock()
	defer a.manager.RUnlock()

	if _, found := a.manager.targets[id]; !found {
		HTTPResponseError(w, http.StatusNotFound, id, " is not found!")
		return
	}

	err := a.manager.requestLogs(id)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}
	HTTPResponseSuccess(w, http.StatusOK, "Requested logs for ", id)
	return
}

func (a *restAPI) websocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{} // use default options
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("websocket: upgrade error:", err)
		return
	}
	defer c.Close()
	for {
		a.manager.update.L.Lock()
		a.manager.update.Wait()
		log.Println("websocket: sending update!")
		b, _ := json.Marshal(a.manager.targets)
		err = c.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			log.Println("websocket: write error:", err)
			a.manager.update.L.Unlock()
			break
		}
		a.manager.update.L.Unlock()
	}
}

// HTTPResponseError serializes and writes an error response
//	If no message is provided, the status text will be set as the message
func HTTPResponseError(w http.ResponseWriter, code int, message ...interface{}) {
	if len(message) == 0 {
		message = make([]interface{}, 1)
		message[0] = http.StatusText(code)
	}
	log.Println("Request error:", message)
	body, _ := json.Marshal(&map[string]string{
		"error": fmt.Sprint(message...),
	})
	HTTPResponse(w, code, body)
}

// HTTPResponseSuccess serializes and writes a success response
//	If no message is provided, the status text will be set as the message
func HTTPResponseSuccess(w http.ResponseWriter, code int, message ...interface{}) {
	if len(message) == 0 {
		message = make([]interface{}, 1)
		message[0] = http.StatusText(code)
	}
	body, _ := json.Marshal(&map[string]string{
		"message": fmt.Sprint(message...),
	})
	HTTPResponse(w, code, body)
}

// HTTPResponse writes a response
func HTTPResponse(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write(body)
	if err != nil {
		log.Printf("HTTPResponse: error writing reponse: %s", err)
	}
}
