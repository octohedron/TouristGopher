package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
)

func init() {
	// set root directory
	ROOT, err := os.Getwd()
	logErr(err)
	gMapsKey = os.Getenv("GMAPS_KEY")
	pRoot = ROOT
	// Establish a pool of 5 Redis connections to the Redis server
	rPOOL = newPool("localhost:6379")
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	conn := rPOOL.Get()
	defer conn.Close()
	placesJSON, err := redis.Strings(conn.Do("SMEMBERS", "places"))
	logErr(err)
	places := reviews{}
	for _, place := range placesJSON {
		p := review{}
		err = json.Unmarshal([]byte(place), &p)
		logErr(err)
		p.Identifier = getRandomString(8)
		p.MarkerVar = getRandomString(8)
		places = append(places, p)
	}
	sort.Sort(places)
	err = template.Must(template.New("index.html").Funcs(template.FuncMap{
		"toJS": func(v string) template.JS {
			return template.JS(v)
		},
	}).ParseFiles(
		pRoot+"/index.html")).Execute(w, struct {
		Places   []review
		gMapsKey string
	}{places, gMapsKey})
	logErr(err)
}

func getHomeData() {
	client := &http.Client{}
	req, err := http.NewRequest("GET",
		"http://localhost:5000/api/2000/48.8589507,2.2775172/restaurants", nil)
	logErr(err)
	res, err := client.Do(req)
	logErr(err)
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return
	}
	var places []review
	err = json.Unmarshal(body, &places)
	logErr(err)
	conn := rPOOL.Get()
	defer conn.Close()
	for _, place := range places {
		json, err := json.Marshal(place)
		logErr(err)
		_, err = conn.Do("SADD", "places", json)
		logErr(err)
	}
	logErr(err)
}

func css(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/css")
	t, err := template.New("styles.css").ParseFiles(pRoot + "/styles.css")
	logErr(err)
	t.Execute(w, r)
}

func api(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	vars := mux.Vars(r)
	fullQ := fmt.Sprintf("%s%s%s", vars["radius"], vars["coords"], vars["needle"])
	hashed := fmt.Sprintf("%x", sha1.Sum([]byte(fullQ)))
	conn := rPOOL.Get()
	defer conn.Close()
	placesJSON, err := redis.Bytes(conn.Do("HGET", string(hashed), "json"))
	logErr(err)
	if placesJSON != nil {
		w.Write(placesJSON)
	} else {
		// call
		client := &http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://localhost:5000/api/%s/%s/%s",
				vars["radius"], vars["coords"], vars["needle"]), nil)
		logErr(err)
		res, err := client.Do(req)
		logErr(err)
		body, err := ioutil.ReadAll(res.Body)
		logErr(err)
		_, err = conn.Do("HSET", string(hashed), "json", body)
		logErr(err)
		// expire in 10 days
		_, err = conn.Do("EXPIRE", string(hashed), 3600*240)
		logErr(err)
		w.Write(body)
	}
}

func main() {
	// get home data and store it in redis
	go getHomeData()
	r := mux.NewRouter()
	r.HandleFunc("/", index)
	r.HandleFunc("/api/{radius}/{coords}/{needle}", api)
	r.HandleFunc("/styles.css", css)
	r.PathPrefix("/js/").Handler(http.StripPrefix("/js/",
		http.FileServer(http.Dir(pRoot+"/js/"))))
	r.PathPrefix("/fonts/").Handler(http.StripPrefix("/fonts/",
		http.FileServer(http.Dir(pRoot+"/fonts/"))))
	r.PathPrefix("/css/").Handler(http.StripPrefix("/css/",
		http.FileServer(http.Dir(pRoot+"/css/"))))
	r.PathPrefix("/images/").Handler(http.StripPrefix("/images/",
		http.FileServer(http.Dir(pRoot+"/images/"))))
	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + gPORT,
		WriteTimeout: 20 * time.Second,
		ReadTimeout:  20 * time.Second,
	}
	err := srv.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
