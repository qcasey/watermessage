package main

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

func query(SQL string) (*sql.Rows, error) {
	log.Debug().Msg(SQL)

	if server.DB == nil {
		log.Info().Msg("Creating new DB connection")
		var err error
		server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
		if err != nil {
			return nil, err
		}
		defer server.DB.Close()
	}

	rows, err := server.DB.Query(SQL)
	if err != nil {
		return nil, err
	}
	return rows, nil
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

// Get group chats?
//sql := "SELECT DISTINCT chat.ROWID, chat.chat_identifier, chat.guid, chat.display_name FROM message LEFT OUTER JOIN chat ON chat.room_name = message.cache_roomnames LEFT OUTER JOIN handle ON handle.ROWID = message.handle_id WHERE message.is_from_me = 0 AND chat.service_name = 'iMessage' AND message.handle_id > 0 ORDER BY message.date DESC"

func refreshChats() {
	server.lock.Lock()
	defer server.lock.Unlock()

	var err error
	server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer server.DB.Close()

	sql := "SELECT DISTINCT chat.ROWID, chat.chat_identifier, chat.guid, chat.display_name FROM chat LEFT OUTER JOIN chat_message_join ON chat.ROWID = chat_message_join.chat_id WHERE chat.service_name = 'iMessage' ORDER BY chat_message_join.message_date DESC"
	rows, err := query(sql)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	newChats := parseChatRows(rows)

	start := time.Now()
	updatedChats := 0
	for _, chat := range newChats {
		// Only fetch entire chat history if there are new messages
		if isNewMessage(chat.ID) {
			updatedChats++
			chat.Messages, err = getAllMessages(chat.ID)
			if err != nil {
				log.Error().Msg(err.Error())
			}
			if len(chat.Messages) > 0 {
				chat.LastMessageDate = chat.Messages[0].Date
				server.ChatMap[chat.ID] = chat
			} else {
				log.Debug().Msg(fmt.Sprintf("0 messages in row %d, %s", chat.RowID, chat.ID))
			}
		}
	}

	log.Debug().Msg(fmt.Sprintf("Took %dms to parse %d rows", time.Since(start).Milliseconds(), updatedChats))
}
