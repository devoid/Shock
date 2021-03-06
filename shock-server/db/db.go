package db

import (
	"fmt"
	"github.com/MG-RAST/Shock/shock-server/conf"
	"labix.org/v2/mgo"
	"os"
	"time"
)

const (
	DbTimeout = time.Duration(time.Second * 1)
)

var (
	Connection connection
)

type connection struct {
	dbname   string
	username string
	password string
	Session  *mgo.Session
	DB       *mgo.Database
}

func Initialize() {
	c := connection{}
	s, err := mgo.DialWithTimeout(conf.Conf["mongodb-hosts"], DbTimeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: no reachable mongodb servers")
		os.Exit(1)
	}
	c.Session = s
	c.DB = c.Session.DB(conf.Conf["mongodb-database"])
	println(conf.Conf["mongodb-user"] + ":" + conf.Conf["mongodb-password"])
	if conf.Conf["mongodb-user"] != "" && conf.Conf["mongodb-password"] != "" {
		c.DB.Login(conf.Conf["mongodb-user"], conf.Conf["mongodb-password"])
	}
	Connection = c
}

func Drop() error {
	return Connection.DB.DropDatabase()
}
