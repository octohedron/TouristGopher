package main

import (
	"encoding/json"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
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
}

type DBReview struct {
	Location string
	Json     []byte
}

var PROJ_ROOT = ""

// mem
var users map[string]User
var AuthorizedIps []string
var ReviewChannel chan DBReview
var COOKIE_NAME = "goddit"
var GPORT = "9000"

// Declare a global variable to store the Redis connection pool.
var POOL *redis.Pool

func init() {
	// set root directory
	ROOT, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
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
	// cookie, err := r.Cookie(COOKIE_NAME)
	conn := POOL.Get()
	defer conn.Close()
	places_json, err := redis.Strings(conn.Do("LRANGE", "places", 0, -1))
	if err != nil {
		log.Println(err)
	}
	var places []Review
	for _, place := range places_json {
		p := Review{}
		json.Unmarshal([]byte(place), &p)
		places = append(places, p)
	}
	template.Must(template.New("index.html").ParseFiles(
		PROJ_ROOT+"/index.html")).Execute(w, struct {
		Places []Review
	}{places})
}

/**
 * Channel to save reviews to the database
 */
func saveReviews(r *chan DBReview) {
	for {
		review, ok := <-*r
		if !ok {
			log.Println("Error when trying to save")
			return
		}
		saveReview(&review)
	}
}

func saveReview(rev *DBReview) {
	var err error
	conn := POOL.Get()
	if err != nil {
		log.Println(err)
	}
	defer conn.Close()
	_, err = conn.Do("RPUSH", rev.Location, rev.Json)
	if err != nil {
		log.Println(err)
	}
}

func getHomeData() {
	client := &http.Client{}
	req, err := http.NewRequest("GET",
		"http://touristfriend.club/api/2000/48.857031,2.341719/hotels", nil)
	if err != nil {
		log.Println(err)
	}
	res, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}
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
		_, err = conn.Do("RPUSH", "places", json)
		if err != nil {
			log.Println(err)
		}
	}
	if err != nil {
		log.Println(err)
	}
}

func main() {
	ReviewChannel = make(chan DBReview, 256)
	// a goroutine for saving reviews
	go saveReviews(&ReviewChannel)
	// get home data and store it in memory
	go getHomeData()
	//for keeping track of users in memory
	users = make(map[string]User)
	r := mux.NewRouter()
	r.HandleFunc("/", index)
	r.PathPrefix("/js/").Handler(http.StripPrefix("/js/",
		http.FileServer(http.Dir(PROJ_ROOT+"/js/"))))
	r.PathPrefix("/fonts/").Handler(http.StripPrefix("/fonts/",
		http.FileServer(http.Dir(PROJ_ROOT+"/fonts/"))))
	r.PathPrefix("/css/").Handler(http.StripPrefix("/css/",
		http.FileServer(http.Dir(PROJ_ROOT+"/css/"))))
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
