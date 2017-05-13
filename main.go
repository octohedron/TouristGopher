package main

import (
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"html/template"
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
	Level     int       `json:"level"`
	Text      string    `json:"text"`
	UserName  string    `json:"name"`
	Timestamp time.Time `json:"timestamp,omitempty"`
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
	template.Must(template.New("index.html").ParseFiles(
		PROJ_ROOT+"/index.html")).Execute(w, r)
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

func main() {
	ReviewChannel = make(chan DBReview, 256)
	// a goroutine for saving reviews
	go saveReviews(&ReviewChannel)
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
