package main

import (
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"
)

// generate random ids
const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

type review struct {
	Rating            string `json:"Rating"`
	Sources           int    `json:"Sources"`
	Number_of_Ratings int    `json:"Number_of_Ratings"`
	Location          string `json:"Location"`
	Name              string `json:"Name"`
	Identifier        string
	MarkerVar         string
}

type reviews []review

var pRoot = ""
var gPORT = "8000"
var gMapsKey = ""

// Declare a global variable to store the Redis connection pool.
var rPOOL *redis.Pool

func logErr(err error) {
	if err != nil {
		log.Println(err)
	}
}

func newPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}
}

func (slice reviews) Len() int {
	return len(slice)
}

func (slice reviews) Less(i, j int) bool {
	a, err := strconv.ParseFloat(slice[i].Rating, 32)
	logErr(err)
	b, err := strconv.ParseFloat(slice[j].Rating, 32)
	logErr(err)
	return a > b
}

func (slice reviews) Swap(i, j int) {
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
