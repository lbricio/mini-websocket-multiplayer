package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	nextPlayerID int
	players      = make(map[*websocket.Conn]*Player)
	playersMu    sync.Mutex
	upgrader     = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type Player struct {
	ID       int    `json:"id"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	IsMoving bool   `json:"-"`
	NextMove string `json:"-"`
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade falhou:", err)
		return
	}

	playersMu.Lock()
	player := &Player{
		ID:       nextPlayerID,
		X:        5,
		Y:        5,
		IsMoving: false,
		NextMove: "",
	}
	players[conn] = player
	nextPlayerID++
	playersMu.Unlock()

	conn.WriteJSON(map[string]interface{}{
		"type": "init",
		"id":   player.ID,
	})

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Cliente desconectado:", err)
			break
		}

		switch msg["type"] {
		case "move":
			dir, ok := msg["dir"].(string)
			if ok {
				playersMu.Lock()
				p, exists := players[conn]
				if exists {
					if !p.IsMoving {
						startMove(p, dir)
					} else {
						p.NextMove = dir
					}
				}
				playersMu.Unlock()
			}
		case "chat":
			text, ok := msg["text"].(string)
			if ok {
				playersMu.Lock()
				sender, exists := players[conn]
				playersMu.Unlock()
				if exists {
					broadcastChat(sender.ID, text)
				}
			}
		}
	}

	playersMu.Lock()
	delete(players, conn)
	playersMu.Unlock()
	conn.Close()
}

func startMove(p *Player, dir string) {
	p.IsMoving = true

	switch dir {
	case "up":
		p.Y--
	case "down":
		p.Y++
	case "left":
		p.X--
	case "right":
		p.X++
	}

	go func(p *Player) {
		time.Sleep(100 * time.Millisecond)

		var next string
		playersMu.Lock()
		if p.NextMove != "" {
			next = p.NextMove
			p.NextMove = ""
		} else {
			p.IsMoving = false
		}
		playersMu.Unlock()

		if next != "" {
			startMove(p, next)
		}
	}(p)
}

func broadcastLoop() {
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		broadcastState()
	}
}

func broadcastState() {
	playersMu.Lock()
	defer playersMu.Unlock()

	var playerList []Player
	for _, p := range players {
		playerList = append(playerList, *p)
	}

	msg := map[string]interface{}{
		"type":    "state",
		"players": playerList,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("Erro ao serializar estado:", err)
		return
	}

	for conn := range players {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Println("Erro enviando estado, removendo conexão:", err)
			conn.Close()
			delete(players, conn)
		}
	}
}

func broadcastChat(senderID int, text string) {
	playersMu.Lock()
	defer playersMu.Unlock()

	msg := map[string]interface{}{
		"type": "chat",
		"id":   senderID,
		"text": text,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("Erro ao serializar chat:", err)
		return
	}

	for conn := range players {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Println("Erro enviando chat, removendo conexão:", err)
			conn.Close()
			delete(players, conn)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleWS)
	http.Handle("/", http.FileServer(http.Dir("./")))

	go broadcastLoop()

	log.Println("Servidor rodando em http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
