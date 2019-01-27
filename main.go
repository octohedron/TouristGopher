package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
)

// generate random ids
const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

type User struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Created_at string `json:"created_at"`
	IP         string
}

type Review struct {
	Rating            string `json:"Rating"`
	Sources           int    `json:"Sources"`
	Number_of_Ratings int    `json:"Number_of_Ratings"`
	Location          string `json:"Location"`
	Name              string `json:"Name"`
	Identifier        string
	MarkerVar         string
}
type Reviews []Review
type DBReview struct {
	Location string
	Json     []byte
}

var PROJ_ROOT = ""
var users map[string]User
var AuthorizedIps []string
var ReviewChannel chan DBReview
var GPORT = "8000"
var GMAPS_KEY = ""

// Declare a global variable to store the Redis connection pool.
var POOL *redis.Pool

func logErr(err error) {
	if err != nil {
		log.Println(err)
	}
}

func init() {
	// set root directory
	ROOT, err := os.Getwd()
	logErr(err)
	GMAPS_KEY = os.Getenv("GMAPS_KEY")
	PROJ_ROOT = ROOT
	// Establish a pool of 5 Redis connections to the Redis server
	POOL = newPool("localhost:6379")
}

func newPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	conn := POOL.Get()
	defer conn.Close()
	placesJSON, err := redis.Strings(conn.Do("SMEMBERS", "places"))
	logErr(err)
	places := Reviews{}
	for _, place := range placesJSON {
		p := Review{}
		json.Unmarshal([]byte(place), &p)
		p.Identifier = getRandomString(8)
		p.MarkerVar = getRandomString(8)
		places = append(places, p)
	}
	sort.Sort(places)
	template.Must(template.New("index.html").Funcs(template.FuncMap{
		"toJS": func(v string) template.JS {
			return template.JS(v)
		},
	}).ParseFiles(
		PROJ_ROOT+"/index.html")).Execute(w, struct {
		Places    []Review
		GMAPS_KEY string
	}{places, GMAPS_KEY})
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
	var places []Review
	json.Unmarshal(body, &places)
	conn := POOL.Get()
	defer conn.Close()
	for _, place := range places {
		json, err := json.Marshal(place)
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
	t, err := template.New("styles.css").ParseFiles(PROJ_ROOT + "/styles.css")
	logErr(err)
	t.Execute(w, r)
}

func (slice Reviews) Len() int {
	return len(slice)
}

func (slice Reviews) Less(i, j int) bool {
	a, err := strconv.ParseFloat(slice[i].Rating, 32)
	logErr(err)
	b, err := strconv.ParseFloat(slice[j].Rating, 32)
	logErr(err)
	return a > b
}

func (slice Reviews) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func getRandomString(n int) string {
	b := make([]byte, n)
	src := rand.NewSource(time.Now().UnixNano())
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func api(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	vars := mux.Vars(r)
	fullQ := fmt.Sprintf("%s%s%s", vars["radius"], vars["coords"], vars["needle"])
	hashed := fmt.Sprintf("%x", sha1.Sum([]byte(fullQ)))
	conn := POOL.Get()
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
	//for keeping track of users in memory
	users = make(map[string]User)
	r := mux.NewRouter()
	r.HandleFunc("/", index)
	r.HandleFunc("/api/{radius}/{coords}/{needle}", api)
	r.HandleFunc("/styles.css", css)
	r.PathPrefix("/js/").Handler(http.StripPrefix("/js/",
		http.FileServer(http.Dir(PROJ_ROOT+"/js/"))))
	r.PathPrefix("/fonts/").Handler(http.StripPrefix("/fonts/",
		http.FileServer(http.Dir(PROJ_ROOT+"/fonts/"))))
	r.PathPrefix("/css/").Handler(http.StripPrefix("/css/",
		http.FileServer(http.Dir(PROJ_ROOT+"/css/"))))
	r.PathPrefix("/images/").Handler(http.StripPrefix("/images/",
		http.FileServer(http.Dir(PROJ_ROOT+"/images/"))))
	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + GPORT,
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
	}
	err := srv.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
