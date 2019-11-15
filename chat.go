package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/MrDoctorKovacic/MDroid-Core/format"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

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
	//log.Info().Msg(fmt.Sprintf("Counted %d chats", len(out)))
	return out
}

func getAllMessagesInChat(chatID string) ([]Message, error) {
	// Default to handle ID, check if it's a group chat
	selector := "chat.room_name IS NULL AND handle.id"
	if strings.Contains(chatID, "chat") {
		selector = "chat.chat_identifier"
	}

	sql := fmt.Sprintf("SELECT DISTINCT message.ROWID, handle.id, message.text, message.is_from_me, message.cache_has_attachments, message.is_delivered, message.date, message.date_delivered, message.date_read FROM message LEFT OUTER JOIN chat ON chat.room_name = message.cache_roomnames LEFT OUTER JOIN handle ON handle.ROWID = message.handle_id WHERE message.service = 'iMessage' AND %s = '%s' ORDER BY message.date DESC LIMIT 50", selector, chatID)
	rows, err := query(sql)
	if err != nil {
		return nil, err
	}
	return parseMessageRows(rows), nil
}

func getLastMessageInChat(chatID string) (Message, error) {
	// Default to handle ID, check if it's a group chat
	selector := "handle.id"
	if strings.Contains(chatID, "chat") {
		selector = "chat.chat_identifier"
	}

	sql := fmt.Sprintf("SELECT DISTINCT message.ROWID, handle.id, message.text, message.is_from_me, message.cache_has_attachments, message.is_delivered, message.date, message.date_delivered, message.date_read FROM message LEFT OUTER JOIN chat ON chat.room_name = message.cache_roomnames LEFT OUTER JOIN handle ON handle.ROWID = message.handle_id WHERE message.service = 'iMessage' AND %s = '%s' ORDER BY message.date DESC LIMIT 1", selector, chatID)
	rows, err := query(sql)
	if err != nil {
		return Message{}, err
	}

	messages := parseMessageRows(rows)
	if messages == nil {
		return Message{}, nil
	}
	return messages[0], nil
}
