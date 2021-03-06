package node

import (
	"github.com/MG-RAST/Shock/shock-server/conf"
	e "github.com/MG-RAST/Shock/shock-server/errors"
	"github.com/MG-RAST/Shock/shock-server/logger"
	"github.com/MG-RAST/Shock/shock-server/node"
	"github.com/MG-RAST/Shock/shock-server/node/file"
	"github.com/MG-RAST/Shock/shock-server/node/filter"
	"github.com/MG-RAST/Shock/shock-server/preauth"
	"github.com/MG-RAST/Shock/shock-server/request"
	"github.com/MG-RAST/Shock/shock-server/user"
	"github.com/MG-RAST/Shock/shock-server/util"
	"github.com/jaredwilkening/goweb"
	"io"
	"net/http"
	"strconv"
	"time"
)

// GET: /node/{id}
// ToDo: clean up this function. About to get unmanageable
func (cr *Controller) Read(id string, cx *goweb.Context) {
	// Log Request and check for Auth
	request.Log(cx.Request)
	u, err := request.Authenticate(cx.Request)
	if err != nil && err.Error() != e.NoAuth {
		request.AuthError(err, cx)
		return
	}

	// Fake public user
	if u == nil {
		if conf.Bool(conf.Conf["anon-read"]) {
			u = &user.User{Uuid: ""}
		} else {
			cx.RespondWithErrorMessage(e.NoAuth, http.StatusUnauthorized)
			return
		}
	}

	// Gather query params
	query := request.Q(cx.Request.URL.Query())

	var fFunc filter.FilterFunc = nil
	if query.Has("filter") {
		if filter.Has(query.Value("filter")) {
			fFunc = filter.Filter(query.Value("filter"))
		}
	}

	// Load node and handle user unauthorized
	n, err := node.Load(id, u.Uuid)
	if err != nil {
		if err.Error() == e.UnAuth {
			cx.RespondWithError(http.StatusUnauthorized)
			return
		} else if err.Error() == e.MongoDocNotFound {
			cx.RespondWithNotFound()
			return
		} else {
			// In theory the db connection could be lost between
			// checking user and load but seems unlikely.
			logger.Error("Err@node_Read:LoadNode:" + id + ":" + err.Error())

			n, err = node.LoadFromDisk(id)
			if err != nil {
				logger.Error("Err@node_Read:LoadNodeFromDisk:" + id + ":" + err.Error())
				cx.RespondWithError(http.StatusInternalServerError)
				return
			}
		}
	}

	// Switch though param flags
	// ?download=1
	if query.Has("download") {
		if !n.HasFile() {
			cx.RespondWithErrorMessage("Node has no file", http.StatusBadRequest)
			return
		}
		filename := n.Id
		if query.Has("filename") {
			filename = query.Value("filename")
		}

		//_, chunksize :=
		// ?index=foo
		if query.Has("index") {
			//handling bam file
			if query.Value("index") == "bai" {
				s := &request.Streamer{R: []file.SectionReader{}, W: cx.ResponseWriter, ContentType: "application/octet-stream", Filename: filename, Size: n.File.Size, Filter: fFunc}

				var region string

				if query.Has("region") {
					//retrieve alingments overlapped with specified region
					region = query.Value("region")
				}

				argv, err := request.ParseSamtoolsArgs(query)
				if err != nil {
					cx.RespondWithErrorMessage("Invaid args in query url", http.StatusBadRequest)
					return
				}

				err = s.StreamSamtools(n.FilePath(), region, argv...)
				if err != nil {
					cx.RespondWithErrorMessage("error while involking samtools", http.StatusBadRequest)
					return
				}

				return
			}

			// if forgot ?part=N
			if !query.Has("part") {
				cx.RespondWithErrorMessage("Index parameter requires part parameter", http.StatusBadRequest)
				return
			}
			// open file
			r, err := n.FileReader()
			if err != nil {
				logger.Error("Err@node_Read:Open: " + err.Error())
				cx.RespondWithError(http.StatusInternalServerError)
				return
			}
			// load index
			idx, err := n.Index(query.Value("index"))
			if err != nil {
				cx.RespondWithErrorMessage("Invalid index", http.StatusBadRequest)
				return
			}

			if idx.Type() == "virtual" {
				csize := conf.CHUNK_SIZE
				if query.Has("chunksize") {
					csize, err = strconv.ParseInt(query.Value("chunksize"), 10, 64)
					if err != nil {
						cx.RespondWithErrorMessage("Invalid chunksize", http.StatusBadRequest)
						return
					}
				}
				idx.Set(map[string]interface{}{"ChunkSize": csize})
			}
			var size int64 = 0
			s := &request.Streamer{R: []file.SectionReader{}, W: cx.ResponseWriter, ContentType: "application/octet-stream", Filename: filename, Filter: fFunc}
			for _, p := range query.List("part") {
				pos, length, err := idx.Part(p)
				if err != nil {
					cx.RespondWithErrorMessage("Invalid index part", http.StatusBadRequest)
					return
				}
				size += length
				s.R = append(s.R, io.NewSectionReader(r, pos, length))
			}
			s.Size = size
			err = s.Stream()
			if err != nil {
				// causes "multiple response.WriteHeader calls" error but better than no response
				cx.RespondWithErrorMessage(err.Error(), http.StatusBadRequest)
				logger.Error("err:@node_Read s.stream: " + err.Error())
			}
		} else {
			nf, err := n.FileReader()
			if err != nil {
				// File not found or some sort of file read error.
				// Probably deserves more checking
				logger.Error("err:@node_Read node.FileReader: " + err.Error())
				cx.RespondWithError(http.StatusBadRequest)
				return
			}
			s := &request.Streamer{R: []file.SectionReader{nf}, W: cx.ResponseWriter, ContentType: "application/octet-stream", Filename: filename, Size: n.File.Size, Filter: fFunc}
			err = s.Stream()
			if err != nil {
				// causes "multiple response.WriteHeader calls" error but better than no response
				cx.RespondWithErrorMessage(err.Error(), http.StatusBadRequest)
				logger.Error("err:@node_Read: s.stream: " + err.Error())
			}
		}
		return
	} else if query.Has("download_url") {
		if !n.HasFile() {
			cx.RespondWithErrorMessage("Node has not file", http.StatusBadRequest)
			return
		} else if u.Uuid == "" {
			cx.RespondWithErrorMessage(e.NoAuth, http.StatusUnauthorized)
			return
		} else {
			options := map[string]string{}
			if query.Has("filename") {
				options["filename"] = query.Value("filename")
			}
			if p, err := preauth.New(util.RandString(20), "download", n.Id, options); err != nil {
				cx.RespondWithError(http.StatusInternalServerError)
				logger.Error("err:@node_Read download_url: " + err.Error())
			} else {
				cx.RespondWithData(util.UrlResponse{Url: util.ApiUrl(cx) + "/preauth/" + p.Id, ValidTill: p.ValidTill.Format(time.ANSIC)})
			}
		}
	} else {
		// Base case respond with node in json
		cx.RespondWithData(n)
	}
}
