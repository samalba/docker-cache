package main

import (
	"net/http"
	"fmt"
	"encoding/json"
	"github.com/gorilla/mux"
)

type apiHandler func(*Cache, http.ResponseWriter, *http.Request)

func StartHttpApi(cache *Cache, address string) error {
	router := initRouter(cache)
	err := http.ListenAndServe(address, router)
	if err != nil {
		return err
	}
	return nil
}

func makeApiHandler(cache *Cache, handler apiHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(cache, w, r)
	}
}

func initRouter(cache *Cache) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", makeApiHandler(cache, handlerRoot))
	r.HandleFunc("/hosts", makeApiHandler(cache, handlerHosts))
	r.HandleFunc("/containers", makeApiHandler(cache, handlerContainers))
	r.HandleFunc("/containers/{name:.*}", makeApiHandler(cache, handlerContainersInfo))
	return r
}

func jsonResponse(data interface{}, w http.ResponseWriter) {
	b, err := json.Marshal(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid data in the cache. Cannot JSON encode: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(b)
}

func handlerRoot(cache *Cache, w http.ResponseWriter, r *http.Request) {
	data := []string{"/hosts", "/containers"}
	jsonResponse(data, w)
}

func handlerHosts(cache *Cache, w http.ResponseWriter, r *http.Request) {
	data, err := cache.ListHosts()
	if err != nil {
		http.Error(w, "Cannot list hosts", http.StatusInternalServerError)
		return
	}
	jsonResponse(data, w)
}

func handlerContainers(cache *Cache, w http.ResponseWriter, r *http.Request) {
	data, err := cache.ListContainers("")
	if err != nil {
		http.Error(w, "Cannot list containers", http.StatusInternalServerError)
		return
	}
	jsonResponse(data, w)
}

func handlerContainersInfo(cache *Cache, w http.ResponseWriter, r *http.Request) {
	data := mux.Vars(r)
	jsonResponse(data, w)
}
