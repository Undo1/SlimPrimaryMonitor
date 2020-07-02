// websockets.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/websocket"
)

var connections map[*websocket.Conn]bool = make(map[*websocket.Conn]bool)

type candidate struct {
	UserID     int
	UserName   string
	Votes      int
	HasChanged bool
}

var candidates map[int]*candidate = make(map[int]*candidate)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil) // error ignored for sake of simplicity

		connections[conn] = true
		defer delete(connections, conn)

		for {
			// Read message from browser
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println(err)
				return
			}

			switch string(msg) {
			case "?":
				if err = conn.WriteMessage(msgType, jsonCandidates(&candidates)); err != nil {
					fmt.Println(err)
					return
				}
			case "ping":
				if err = conn.WriteMessage(msgType, []byte("pong")); err != nil {
					fmt.Println(err)
					return
				}
			default:
				// Print the message to the console
				fmt.Printf("Received unrecognized message from %s: %s\n", conn.RemoteAddr(), string(msg))

				// Write message back to browser
				if err = conn.WriteMessage(msgType, []byte("Unrecognized message")); err != nil {
					fmt.Println(err)
					return
				}
			}
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "websockets.html")
	})

	go http.ListenAndServe(":8080", nil)

	for {
		scrapeElection()
		time.Sleep(10 * 1000000000)
	}
}

func jsonCandidates(input *map[int]*candidate) []byte {
	response, err := json.Marshal(input)
	if err != nil {
		log.Fatal(err)
	}

	return response
}

func scrapeElection() {
	// Request the HTML page.
	link := "https://stackoverflow.com/election/11?tab=primary"
	res, err := http.Get(link)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	changedCandidates := make(map[int]*candidate)

	// Find the review items
	doc.Find("div.candidate-row").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		votes, _ := strconv.Atoi(s.Find(".js-vote-count").Text())
		userName := s.Find("div.user-details a").Text()
		strUserID, _ := s.Find("div.user-details a").Attr("href")
		userID, _ := strconv.Atoi(strings.Split(strUserID, "/")[2])

		// Random stuff
		if rand.Intn(10) == 0 {
			votes = rand.Intn(100)
		}

		fmt.Printf("Candidate %d: %s: %d\n", userID, userName, votes)

		if c, ok := candidates[userID]; ok { // Check if we've already seen this candidate
			if c.Votes != votes {
				c.Votes = votes
				c.HasChanged = true
				changedCandidates[userID] = c
			} else {
				c.HasChanged = false
			}
		} else {
			candidates[userID] = &candidate{UserID: userID, UserName: userName, Votes: votes, HasChanged: true}
			changedCandidates[userID] = candidates[userID]
		}
	})

	if len(changedCandidates) > 0 {
		fmt.Printf("Broadcasting to %d clients\n", len(connections))

		changedCandidateJSON := jsonCandidates(&changedCandidates)
		for connection := range connections {
			err := connection.WriteMessage(1, changedCandidateJSON)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
