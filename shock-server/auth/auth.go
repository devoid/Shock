package auth

import (
	"errors"
	"github.com/MG-RAST/Shock/shock-server/auth/basic"
	"github.com/MG-RAST/Shock/shock-server/auth/globus"
	"github.com/MG-RAST/Shock/shock-server/conf"
	"github.com/MG-RAST/Shock/shock-server/user"
	"strings"
)

var authCache cache

func init() {
	authCache = cache{m: make(map[string]cacheValue)}
}

func AuthHeaderType(header string) string {
	tmp := strings.Split(header, " ")
	if len(tmp) > 1 {
		return tmp[0]
	}
	return ""
}

func Authenticate(header string) (u *user.User, err error) {
	if u = authCache.lookup(header); u != nil {
		return u, nil
	}

	switch conf.Conf["auth-type"] {
	case "globus":
		switch AuthHeaderType(header) {
		case "Globus-Goauthtoken", "OAuth":
			// check cache
			// auth from server
			if u, err = globus.AuthToken(strings.Split(header, " ")[1]); err == nil {
				authCache.add(header, u)
				return
			} else {
				return nil, err
			}
			// cache results
		case "Basic":
			if username, password, err := basic.DecodeHeader(header); err == nil {
				if u, err := globus.AuthUsernamePassword(username, password); err == nil {
					authCache.add(header, u)
					return u, nil
				} else {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	case "mgrast":
		switch AuthHeaderType(header) {
		case "OAuth":
		case "Basic":
			return nil, errors.New("This instance does not support username/password authentication. Please use MG-RAST your token.")
		}
	case "oauth":
		// stub
	case "basic":
		if username, password, err := basic.DecodeHeader(header); err == nil {
			return basic.Auth(username, password)
		} else {
			return nil, err
		}
	}
	return
}
