package main

import (
	"database/sql"
	"net/http"
	"sync"

	"github.com/MrDoctorKovacic/MDroid-Core/format"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// Chat corresponds to each row in the 'chat' table, along with a lock for the Messages
type Chat struct {
	RowID           int       `json:"RowID"`
	ID              string    `json:"ID"`
	Name            string    `json:"Name"`
	DisplayName     string    `json:"DisplayName"`
	ServiceName     string    `json:"ServiceName"`
	Messages        []Message `json:"Messages"`
	Recipients      []Handle  `json:"Recipients"`
	LastMessageDate int       `json:"LastMessageDate"`
	lock            sync.RWMutex
}

func handleChatGetAll(w http.ResponseWriter, r *http.Request) {
	refreshChats()

	// Sort chats
	chats := make(map[int]*Chat, 0)
	for _, chat := range server.ChatMap {
		chats[chat.LastMessageDate] = chat
	}

	format.WriteResponse(&w, r, format.JSONResponse{OK: true, Output: chats})
}

func handleChatGet(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	server.lock.Lock()
	defer server.lock.Unlock()

	server.openDB()
	defer server.DB.Close()

	chat, err := getAllMessages(params["id"])
	if err != nil {
		log.Error().Msg(err.Error())
		format.WriteResponse(&w, r, format.JSONResponse{OK: false, Output: err.Error()})
		return
	}

	format.WriteResponse(&w, r, format.JSONResponse{OK: true, Output: chat})
}

func handleChatGetLast(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	server.lock.Lock()
	defer server.lock.Unlock()

	server.openDB()
	defer server.DB.Close()

	message, err := getLastMessage(params["id"])
	if err != nil {
		log.Error().Msg(err.Error())
		format.WriteResponse(&w, r, format.JSONResponse{OK: false, Output: err.Error()})
		return
	}

	format.WriteResponse(&w, r, format.JSONResponse{OK: true, Output: message})
}

func parseChatRows(rows *sql.Rows) []*Chat {
	var out []*Chat
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		c := Chat{}
		rows.Scan(&c.RowID, &c.ID, &c.Name, &c.DisplayName)
		out = append(out, &c)
	}
	return out
}
