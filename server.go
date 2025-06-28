package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
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

var (
	mapWidth  int
	mapHeight int
)

type ChatMessage struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"` // Unix milli
}

type Player struct {
	ID             int           `json:"id"`
	X              int           `json:"x"`
	Y              int           `json:"y"`
	Direction      string        `json:"direction"`
	CharacterIndex int           `json:"characterIndex"`
	IsMoving       bool          `json:"-"`
	NextMove       string        `json:"-"`
	ChatMessages   []ChatMessage `json:"chatMessages"`
}

type MapData struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func loadMapDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var mapData MapData
	if err := json.NewDecoder(file).Decode(&mapData); err != nil {
		return 0, 0, err
	}

	return mapData.Width, mapData.Height, nil
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
				if exists {
					msg := ChatMessage{
						Text:      text,
						Timestamp: time.Now().UnixMilli(),
					}
					sender.ChatMessages = append(sender.ChatMessages, msg)
					if len(sender.ChatMessages) > 3 {
						sender.ChatMessages = sender.ChatMessages[1:] // remove o mais antigo
					}
				}
				playersMu.Unlock()
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
	now := time.Now().UnixMilli()
	playersMu.Lock()
	for _, p := range players {
		// Remove mensagens com mais de 15 segundos
		filtered := p.ChatMessages[:0]
		for _, m := range p.ChatMessages {
			if now-m.Timestamp <= 15_000 {
				filtered = append(filtered, m)
			}
		}
		p.ChatMessages = filtered
	}
	playersMu.Unlock()

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
	var err error
	mapWidth, mapHeight, err = loadMapDimensions("maps/test.json")
	if err != nil {
		log.Fatalf("Erro ao carregar mapa: %v", err)
	}

	http.HandleFunc("/ws", handleWS)
	http.Handle("/", http.FileServer(http.Dir("./")))

	go broadcastLoop()

	log.Printf("Servidor rodando em http://localhost:8080 (mapa: %dx%d)\n", mapWidth, mapHeight)
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
