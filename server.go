package main

import (
	"database/sql"
	"fmt"
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
	Attachments  map[string]Attachment
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
	RowID         string       `json:RowID,omitempty`
	Text          *string      `json:Text`
	IsFromMe      bool         `json:IsFromMe`
	HasAttachment bool         `json:HasAttachment`
	Delivered     bool         `json:Delivered`
	Date          int          `json:Date`
	DateDelivered int          `json:DateDelivered`
	DateRead      int          `json:DateRead`
	Handle        Handle       `json:Handle`
	Attachments   []Attachment `json:Attachment`
}

type Handle struct {
	ID *string `json:ID`
}

type Attachment struct {
	MessageID     *string `json:MessageID`
	ID            *string `json:ID`
	Filename      *string `json:Filename`
	MIMEType      *string `json:MIMEType`
	TransferState int     `json:TransferState`
	TotalBytes    int     `json:TotalBytes`
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

func handleAttachmentsGetAll(w http.ResponseWriter, r *http.Request) {
	server.lock.RLock()
	defer server.lock.RUnlock()
	format.WriteResponse(&w, r, format.JSONResponse{OK: true, Output: server.Attachments})
}

func handleAttachmentsGet(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	server.lock.RLock()
	defer server.lock.RUnlock()

	attachment, ok := server.Attachments[params["id"]]
	if !ok {
		em := fmt.Sprintf("Could not fetch attachment id %s. It likely doesn't exist", params["id"])
		log.Error().Msg(em)
		format.WriteResponse(&w, r, format.JSONResponse{OK: false, Output: ""})
	}

	format.WriteResponse(&w, r, format.JSONResponse{OK: true, Output: attachment})
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
