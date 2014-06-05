package routes

import (
	"github.com/danjac/photoshare/api/models"
	"github.com/danjac/photoshare/api/render"
	"github.com/danjac/photoshare/api/session"
	"github.com/dchest/uniuri"
	"github.com/gorilla/mux"
	"net/http"
)

func upload(w http.ResponseWriter, r *http.Request) {

	user, err := session.GetCurrentUser(r)
	if err != nil {
		render.Error(w, err)
		return
	}
	if user == nil {
		render.Status(w, http.StatusUnauthorized, "Not logged in")
		return
	}

	title := r.FormValue("title")
	src, hdr, err := r.FormFile("photo")
	if err != nil {
		render.Error(w, err)
		return
	}
	contentType := hdr.Header["Content-Type"][0]

	defer src.Close()

	filename := uniuri.New()

	if contentType == "image/png" {
		filename += ".png"
	} else if contentType == "image/jpeg" {
		filename += ".jpg"
	} else {
		render.Status(w, http.StatusBadRequest, "Not a valid image")
		return
	}

	photo := &models.Photo{Title: title,
		OwnerID: user.ID}

	if err := photo.Save(); err != nil {
		render.Error(w, err)
		return
	}

	go photo.ProcessImage(src, filename, contentType)

	render.JSON(w, http.StatusOK, photo)
}

func getPhotos(w http.ResponseWriter, r *http.Request) {

	photos, err := models.GetPhotos()
	if err != nil {
		render.Error(w, err)
		return
	}
	render.JSON(w, http.StatusOK, photos)
}

// this should be DELETE
func logout(w http.ResponseWriter, r *http.Request) {

	if err := session.Logout(w); err != nil {
		render.Error(w, err)
		return
	}

	render.Status(w, http.StatusOK, "Logged out")

}

// return current logged in user, or 401
func authenticate(w http.ResponseWriter, r *http.Request) {

	user, err := session.GetCurrentUser(r)
	if err != nil {
		render.Error(w, err)
		return
	}

	var status int

	if user != nil {
		status = http.StatusOK
	} else {
		status = http.StatusNotFound
	}

	render.JSON(w, status, user)
}

func login(w http.ResponseWriter, r *http.Request) {

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		render.Status(w, http.StatusBadRequest, "Email or password missing")
		return
	}

	user, err := models.Authenticate(email, password)
	if err != nil {
		render.Error(w, err)
		return
	}

	if user != nil {
		if err := session.Login(w, user); err != nil {
			render.Error(w, err)
			return
		}
	}

	render.JSON(w, http.StatusOK, user)
}

func Init() http.Handler {
	r := mux.NewRouter()

	s := r.PathPrefix("/auth").Subrouter()
	s.HandleFunc("/", authenticate).Methods("GET")
	s.HandleFunc("/", login).Methods("POST")
	s.HandleFunc("/", logout).Methods("DELETE")

	s = r.PathPrefix("/photos").Subrouter()
	s.HandleFunc("/", getPhotos).Methods("GET")
	s.HandleFunc("/", upload).Methods("POST")

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))

	return session.NewCSRF(r)
}