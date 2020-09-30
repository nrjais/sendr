package main

import (
	"io/ioutil"
	"net/http"
	"sync"
)

func httpServer() {
	http.HandleFunc("/sdp/sen", handlerSDP())
	http.HandleFunc("/sdp/rec", handlerSDP())

	http.HandleFunc("/candidate/sen", handler())

	http.HandleFunc("/candidate/rec", handler())

	http.ListenAndServe("0.0.0.0:8080", nil)
}

func handler() func(http.ResponseWriter, *http.Request) {
	var mu sync.Mutex
	candidates := make([][]byte, 0)
	return func(res http.ResponseWriter, req *http.Request) {
		if req.Method == "POST" {
			b, err := ioutil.ReadAll(req.Body)
			checkErr(err)

			mu.Lock()
			defer mu.Unlock()

			candidates = append(candidates, b)
		} else {
			mu.Lock()
			defer mu.Unlock()
			if len(candidates) == 0 {
				res.WriteHeader(404)
				return
			} else {
				c := candidates[0]
				candidates = candidates[1:]

				res.Write(c)
			}
		}
	}
}


func handlerSDP() func(http.ResponseWriter, *http.Request) {
	var mu sync.Mutex
	offers := make([][]byte, 0)
	return func(res http.ResponseWriter, req *http.Request) {
		if req.Method == "POST" {
			b, err := ioutil.ReadAll(req.Body)
			checkErr(err)

			mu.Lock()
			defer mu.Unlock()

			offers = append(offers, b)
		} else {
			mu.Lock()
			defer mu.Unlock()
			if len(offers) == 0 {
				res.WriteHeader(404)
				return
			} else {
				c := offers[0]
				offers = offers[1:]

				res.Write(c)
			}
		}
	}
}
