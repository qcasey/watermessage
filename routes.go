package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/user"
	"time"

	"github.com/MrDoctorKovacic/MDroid-Core/format"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

const (
	//AttachmentDirectory = "~/Library/Messages/Attachments/"
	LocalDirectory = "/Library/Messages/Attachments/"
	Port           = ":1358"
)

func startRouter() {

	// get user directory
	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	attachmentsDirectory := fmt.Sprintf("%s%s", user.HomeDir, LocalDirectory)

	// Init router
	router := mux.NewRouter()

	router.HandleFunc("/chats", handleChatGetAll).Methods("GET")
	router.HandleFunc("/chats/{id}", handleChatGet).Methods("GET")
	router.HandleFunc("/chats/{id}/last", handleChatGetLast).Methods("GET")

	router.HandleFunc("/attachments", handleAttachmentsGetAll).Methods("GET")
	router.HandleFunc("/attachments/{id}", handleAttachmentsGet).Methods("GET")

	router.
		PathPrefix("/file/").
		Handler(http.StripPrefix("/file/", http.FileServer(http.Dir(attachmentsDirectory))))

	//
	// Finally, welcome and meta routes
	//
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response := format.JSONResponse{Output: "OK", OK: true}
		format.WriteResponse(&w, r, response)
	}).Methods("GET")

	// Use checksum middleware
	router.Use(checksumMiddleware)

	log.Info().Msg("Starting server on port 1358")

	// Start the router in an endless loop
	for {
		err := http.ListenAndServe(Port, router)
		log.Error().Msg(err.Error())
		log.Error().Msg("Router failed! We messed up really bad to get this far. Restarting the router...")
		time.Sleep(time.Second * 10)
	}
}

// authMiddleware will match http bearer token again the one hardcoded in our config
/*
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		reqToken := r.Header.Get("Authorization")
		splitToken := strings.Split(reqToken, "Bearer")
		if len(splitToken) != 2 || strings.TrimSpace(splitToken[1]) != settings.AuthToken {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("403 - Invalid Auth Token!"))
		}

		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}*/

func checksumMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for r.Method == "POST" {
			params := mux.Vars(r)
			checksum, ok := params["checksum"]

			if !ok || checksum == "" {
				break
			}

			body, err := ioutil.ReadAll(r.Body)
			defer r.Body.Close() //  must close
			if err != nil {
				log.Error().Msg(fmt.Sprintf("Error reading body: %v", err))
				format.WriteResponse(&w, r, format.JSONResponse{Output: "Can't read body", OK: false})
				break
			}

			if md5.Sum(body) != md5.Sum([]byte(checksum)) {
				log.Error().Msg(fmt.Sprintf("Invalid checksum %s", checksum))
				format.WriteResponse(&w, r, format.JSONResponse{Output: fmt.Sprintf("Invalid checksum %s", checksum), OK: false})
				break
			}
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
			break
		}

		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}
