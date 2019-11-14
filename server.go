package main

import (
	"database/sql"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/MrDoctorKovacic/MDroid-Core/format"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type ServerType struct {
	Chats        []*Chat
	ChatMap      map[string]*Chat
	DB           *sql.DB
	SQLiteFile   string
	LastModified time.Time
	lock         sync.RWMutex
}

type Chat struct {
	RowID           int       `json:RowID`
	ID              string    `json:ID`
	Name            string    `json:Name`
	DisplayName     string    `json:DisplayName`
	ServiceName     string    `json:DisplayName`
	Messages        []Message `json:Messages`
	LastMessageDate int       `json:LastMessageDate`
	lock            sync.RWMutex
}

type Message struct {
	RowID         string  `json:RowID,omitempty`
	Text          *string `json:Text`
	IsFromMe      bool    `json:IsFromMe`
	Delivered     bool    `json:Delivered`
	Date          int     `json:Date`
	DateDelivered int     `json:DateDelivered`
	DateRead      int     `json:DateRead`
	Handle        Handle  `json:Handle`
}

type Handle struct {
	ID *string `json:ID`
}

func hasBeenModified() bool {
	info, err := os.Stat(server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
	}
	if info.ModTime().After(server.LastModified) {
		server.LastModified = info.ModTime()
		return true
	}
	return false
}

func handleChatGetAll(w http.ResponseWriter, r *http.Request) {
	server.lock.Lock()
	defer server.lock.Unlock()
	var err error
	server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer server.DB.Close()

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
	var err error
	server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer server.DB.Close()

	chat, err := getAllMessagesInChat(params["id"])
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
	var err error
	server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer server.DB.Close()

	message, err := getLastMessageInChat(params["id"])
	if err != nil {
		log.Error().Msg(err.Error())
		format.WriteResponse(&w, r, format.JSONResponse{OK: false, Output: err.Error()})
		return
	}

	format.WriteResponse(&w, r, format.JSONResponse{OK: true, Output: message})
}
