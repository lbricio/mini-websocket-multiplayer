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

const (
	mapWidth  = 16
	mapHeight = 16
)

type Player struct {
	ID             int    `json:"id"`
	X              int    `json:"x"`
	Y              int    `json:"y"`
	Direction      string `json:"direction"`
	CharacterIndex int    `json:"characterIndex"`
	IsMoving       bool   `json:"-"`
	NextMove       string `json:"-"`
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade falhou:", err)
		return
	}

	playersMu.Lock()
	player := &Player{
		ID:             nextPlayerID,
		X:              5,
		Y:              5,
		Direction:      "down",
		CharacterIndex: nextPlayerID % 8, // Para testes, simples round-robin
		IsMoving:       false,
		NextMove:       "",
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
	p.Direction = dir

	var dx, dy int
	switch dir {
	case "up":
		dy = -1
	case "down":
		dy = 1
	case "left":
		dx = -1
	case "right":
		dx = 1
	}

	// Limitar dentro do mapa
	newX := p.X + dx
	newY := p.Y + dy

	if newX >= 0 && newX < mapWidth && newY >= 0 && newY < mapHeight {
		p.X = newX
		p.Y = newY
	}

	// Tempo fixo por tile
	const moveDuration = 100 * time.Millisecond

	go func(p *Player) {
		time.Sleep(moveDuration)

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
