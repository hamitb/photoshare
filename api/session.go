package api

import (
	jwt "github.com/dgrijalva/jwt-go"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	tokenHeader = "X-Auth-Token"
	expiry      = 60 // minutes
)

type SessionManager interface {
	ReadToken(*http.Request) (int64, error)
	WriteToken(http.ResponseWriter, int64) error
}

func NewSessionManager(config *AppConfig) (SessionManager, error) {
	mgr := &defaultSessionManager{}
	var err error
	mgr.signKey, err = ioutil.ReadFile(config.PrivateKey)
	if err != nil {
		return mgr, err
	}
	mgr.verifyKey, err = ioutil.ReadFile(config.PublicKey)
	if err != nil {
		return mgr, err
	}
	return mgr, nil
}

type defaultSessionManager struct {
	verifyKey, signKey []byte
}

func (m *defaultSessionManager) ReadToken(r *http.Request) (int64, error) {
	tokenString := r.Header.Get(tokenHeader)
	if tokenString == "" {
		return 0, nil
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) ([]byte, error) {
		return m.verifyKey, nil
	})
	switch err.(type) {
	case nil:
		if !token.Valid {
			return 0, nil
		}
		token := token.Claims["uid"].(string)
		if userID, err := strconv.ParseInt(token, 10, 0); err != nil {
			return 0, nil
		} else {
			return userID, nil
		}
	case *jwt.ValidationError:
		return 0, nil
	default:
		return 0, err
	}
}

func (m *defaultSessionManager) WriteToken(w http.ResponseWriter, userID int64) error {
	token := jwt.New(jwt.GetSigningMethod("RS256"))
	token.Claims["uid"] = strconv.FormatInt(userID, 10)
	token.Claims["exp"] = time.Now().Add(time.Minute * expiry).Unix()
	tokenString, err := token.SignedString(m.signKey)
	if err != nil {
		return err
	}
	w.Header().Set(tokenHeader, tokenString)
	return nil
}
