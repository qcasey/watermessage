package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

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

func parseMessageRows(rows *sql.Rows) []Message {
	var out []Message
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		m := Message{}
		err := rows.Scan(&m.RowID, &m.Handle.ID, &m.Text, &m.IsFromMe, &m.HasAttachment, &m.Delivered, &m.Date, &m.DateDelivered, &m.DateRead)
		if err != nil {
			log.Error().Msg(err.Error())
		}
		if m.Handle.ID == nil {
			newID := "me"
			m.Handle.ID = &newID
		}
		if m.HasAttachment {
			attachments, err := getAttachment(m.RowID)
			if err == nil {
				m.Attachments = attachments
			} else {
				log.Error().Msg(err.Error())
			}
		}
		out = append(out, m)
	}
	return out
}
