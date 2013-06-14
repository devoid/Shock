package main

import (
	"fmt"
	"github.com/MG-RAST/Shock/shock-client/conf"
	"github.com/MG-RAST/Shock/shock-client/lib"
	"io"
	"os"
	"strconv"
	"strings"
)

const Usage = `Usage: shock-client <command> [options...] [args..]
Global Options:
    -conf                       Config file location (default ~/.shock-client.cfg)

Commands:
help                            This help message
create [options...]
    -attributes=<i>             JSON formated attribute file
                                Note: Attributes will replace all current attributes
    Mutualy exclusive options:
    -full=<u>                   Path to file
    -parts=<p>                  Number of parts to be uploaded
    -virtual_file=<s>           Comma seperated list of node ids
    -remote_path=<p>            Remote file path
    
pcreate [options...]
    -full=<u>                   Path to file
    -threads=<i>                number of threads to use for uploading (default 4)
    
    Note: parallel uploading for the whole file.

update [options...] <id>
    -part=<p> -file=<f>         The part number to be uploaded and path to file
                                Note: parts must be set
    Note: With the inclusion of part update options are the same as create.
    
get <id>
    
download [options...] <id> [<output>]
    -index=<i>                  Name of index (must be used with -parts)
    -parts={p}                  Part(s) from index, may be a range eg. 1-10
    -index_options=<o>          Additional index options. Varies by index type
    
    Note: if output is not present the download will be written to stdout.
    
pdownload [options...] <id> [<output>]
    -threads=<i>                number of threads to use for downloading (default 4)
    
    Note: parallel download for the whole file. if output is not present the download will 
          be written to a file named as the shock node id.
    
acl <add/rm> <all/read/write/delete> <users> <id>
    Note: users are in the form of comma delimited list of email address or uuids 

chown <user> <id>
    Note: user is email address or uuid 

auth show                       Displays username of currently authenticated user
     set                        Prompts for user authentication and store credentials 
     unset                      Deletes stored credentials
`

// print help & die
func helpf(e string) {
	fmt.Fprintln(os.Stderr, Usage)
	if e != "" {
		fmt.Fprintln(os.Stderr, "Error: "+e)
	}
	os.Exit(1)
}

func handle(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func handleToken(err error) {
	if strings.Contains(err.Error(), "no such file or directory") {
		fmt.Fprintln(os.Stderr, "No stored authentication credentials found.\n")
	} else {
		fmt.Fprintln(os.Stderr, "%s\n", err.Error())
	}
	os.Exit(1)
}

func setToken(fatal bool) {
	t := &lib.Token{}
	if err := t.Load(); err != nil {
		if fatal {
			handleToken(err)
		}
	} else {
		lib.SetTokenAuth(t)
	}
}

func acl(action, perm, users, id string) (err error) {
	n := lib.Node{Id: id}
	switch action {
	case "add":
		return n.AclAdd(perm, users)
	case "rm":
		return n.AclRemove(perm, users)
	case "chown":
		return n.AclChown(users)
	default:
		helpf("")
	}
	return
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "help" {
		helpf("")
	}
	conf.Initialize(args[1:])

	setToken(true)
	switch args[0] {
	case "create", "update":
		n := lib.Node{}
		if args[0] == "update" {
			if len(args) != 2 {
				helpf("update requires <id>")
			} else {
				n.Id = args[1]
			}
		}
		opts := lib.Opts{}
		if ne(conf.Flags["attributes"]) {
			opts["attributes"] = (*conf.Flags["attributes"])
		}
		if t, err := fileOptions(conf.Flags); err != nil {
			helpf(err.Error())
		} else {
			if t == "part" {
				if args[0] == "create" {
					helpf("part option only usable with update")
				}
				if !ne(conf.Flags["file"]) {
					helpf("part option requires file")
				}
				opts["upload_type"] = t
				opts["part"] = (*conf.Flags["part"])
				opts["file"] = (*conf.Flags["file"])
				if err := n.Update(opts); err != nil {
					fmt.Printf("Error updating %s: %s\n", n.Id, err.Error())
				} else {
					n.PP()
				}
			} else {
				if t != "" {
					opts["upload_type"] = t
					opts[t] = (*conf.Flags[t])
					if args[0] == "create" {
						if err := n.Create(opts); err != nil {
							fmt.Printf("Error creating node: %s\n", err.Error())
						} else {
							n.PP()
						}
					} else {
						if err := n.Update(opts); err != nil {
							fmt.Printf("Error updating %s: %s\n", n.Id, err.Error())
						} else {
							n.PP()
						}
					}
				} else {
					if err := n.Create(opts); err != nil {
						fmt.Printf("Error creating node: %s\n", err.Error())
					} else {
						n.PP()
					}
				}
			}
		}
	case "pcreate":
		n := lib.Node{}
		var filename string
		if ne(conf.Flags["full"]) {
			filename = (*conf.Flags["full"])
		} else {
			helpf("pcreate requires file path: -full=<u>")
		}

		var filesize int64
		if fi, err := os.Stat(filename); err == nil {
			filesize = fi.Size()
		} else {
			helpf(err.Error())
		}

		totalChunk := int(filesize / conf.CHUNK_SIZE)
		m := filesize % conf.CHUNK_SIZE
		if m != 0 {
			totalChunk += 1
		}
		splits := conf.DOWNLOAD_THREADS
		if ne(conf.Flags["threads"]) {
			if th, err := strconv.Atoi(*conf.Flags["threads"]); err == nil {
				splits = th
			}
		}
		if totalChunk < splits {
			splits = totalChunk
		}
		if splits == 1 {
			opts := lib.Opts{}
			opts["upload_type"] = "full"
			opts["full"] = filename
			if err := n.Create(opts); err != nil {
				fmt.Printf("Error creating node: %s\n", err.Error())
			} else {
				n.PP()
			}
		} else {
			//create parts
			opts := lib.Opts{}
			opts["upload_type"] = "parts"
			opts["parts"] = strconv.Itoa(splits)
			if err := n.Create(opts); err != nil {
				fmt.Printf("Error creating node: %s\n", err.Error())
			}

			//calculate split size
			splitsize := filesize / int64(splits)
			if filesize%int64(splits) != 0 {
				splitsize += 1
			}

			origf, err := os.Open(filename)
			if err != nil {
				fmt.Printf("Error open file: %s\n", err.Error())
			}
			ch := make(chan int, 1)
			for i := 0; i < splits; i++ {
				opts := lib.Opts{}
				opts["upload_type"] = "part"
				opts["part"] = strconv.Itoa(i + 1)

				tmpfname := fmt.Sprintf("%s_%d", filename, i+1)
				tmpf, err := os.Create(tmpfname)
				if err != nil {
					fmt.Printf("Error create temp file: %s\n", err.Error())
				}

				io.CopyN(tmpf, origf, int64(splitsize))
				tmpf.Close()

				opts["file"] = tmpfname

				go uploadChunk(n, opts, tmpfname, ch)
			}
			origf.Close()
			fmt.Printf("Uploading using %d threads...\n", splits)
			for i := 0; i < splits; i++ {
				<-ch
			}
			n.Get()
			n.PP()
		}

	case "get":
		if len(args) != 2 {
			helpf("get requires <id>")
		}
		n := lib.Node{Id: args[1]}
		if err := n.Get(); err != nil {
			fmt.Printf("Error retrieving %s: %s\n", n.Id, err.Error())
		} else {
			n.PP()
		}
	case "download":
		if len(args) < 2 {
			helpf("download requires <id>")
		}
		index := conf.Flags["index"]
		parts := conf.Flags["parts"]
		indexOptions := conf.Flags["index_options"]
		opts := lib.Opts{}
		if ne(index) || ne(parts) || ne(indexOptions) {
			if ne(index) && ne(parts) {
				opts["index"] = (*index)
				opts["parts"] = (*parts)
				if ne(indexOptions) {
					opts["index_options"] = (*indexOptions)
				}
			} else {
				helpf("index and parts options must be used together")
			}
		}
		n := lib.Node{Id: args[1]}
		if ih, err := n.Download(opts); err != nil {
			fmt.Printf("Error downloading %s: %s\n", n.Id, err.Error())
		} else {
			if len(args) == 3 {
				if oh, err := os.Create(args[2]); err == nil {
					if s, err := io.Copy(oh, ih); err != nil {
						fmt.Printf("Error writing output: %s\n", err.Error())
					} else {
						fmt.Printf("Success. Wrote %d bytes\n", s)
					}
				} else {
					fmt.Printf("Error writing output: %s\n", err.Error())
				}
			} else {
				io.Copy(os.Stdout, ih)
			}
		}
	case "pdownload":
		if len(args) < 2 {
			helpf("pdownload requires <id>")
		}
		n := lib.Node{Id: args[1]}
		if err := n.Get(); err != nil {
			fmt.Printf("Error retrieving %s: %s\n", n.Id, err.Error())
		}

		totalChunk := int(n.File.Size / conf.CHUNK_SIZE)
		m := n.File.Size % conf.CHUNK_SIZE
		if m != 0 {
			totalChunk += 1
		}
		if totalChunk < 1 {
			totalChunk = 1
		}

		splits := conf.DOWNLOAD_THREADS
		if ne(conf.Flags["threads"]) {
			if th, err := strconv.Atoi(*conf.Flags["threads"]); err == nil {
				splits = th
			}
		}
		if totalChunk < splits {
			splits = totalChunk
		}

		fmt.Printf("downloading using %d threads\n", splits)

		split_size := totalChunk / splits
		remainder := totalChunk % splits
		if remainder > 0 {
			split_size += 1
		}

		var filename string
		if len(args) == 3 {
			filename = args[2]
		} else {
			filename = n.Id
		}

		oh, err := os.Create(filename)
		if err != nil {
			fmt.Printf("Error creating output file %s: %s\n", filename, err.Error())
			return
		}
		oh.Close()

		ch := make(chan int, 1)
		for i := 0; i < splits; i++ {
			start_chunk := i*split_size + 1
			end_chunk := (i + 1) * split_size
			if end_chunk > totalChunk {
				end_chunk = totalChunk
			}
			part_string := fmt.Sprintf("%d-%d", start_chunk, end_chunk)
			opts := lib.Opts{}
			opts["index"] = "size"
			opts["parts"] = part_string
			start_offset := (int64(start_chunk) - 1) * conf.CHUNK_SIZE
			go downloadChunk(n, opts, filename, start_offset, ch)
		}
		for i := 0; i < splits; i++ {
			<-ch
		}
	case "auth":
		if len(args) != 2 {
			helpf("auth requires show/set/unset")
		}
		switch args[1] {
		case "set":
			var username, password string
			fmt.Printf("Please authenticate to store your credentials.\nusername: ")
			fmt.Scan(&username)
			fmt.Printf("password: ")
			fmt.Scan(&password)
			if t, err := lib.OAuthToken(username, password); err == nil {
				if err := t.Store(); err != nil {
					fmt.Printf("Authenticated but failed to store token: %s\n", err.Error())
				}
				fmt.Printf("Authenticated credentials stored for user %s. Expires in %d days.\n", t.UserName, t.ExpiresInDays())
			} else {
				fmt.Printf("%s\n", err.Error())
			}
		case "unset":
			t := &lib.Token{}
			if err := t.Delete(); err != nil {
				fmt.Printf("%s\n", err.Error())
			} else {
				fmt.Printf("Stored authentication credentials have been deleted.\n")
			}
		case "show":
			t := &lib.Token{}
			if err := t.Load(); err != nil {
				handleToken(err)
			} else {
				fmt.Printf("Authenticated credentials stored for user %s. Expires in %d days.\n", t.UserName, t.ExpiresInDays())
			}
		}
	case "acl":
		if len(args) != 5 {
			helpf("acl requires 4 arguments")
		}
		if err := acl(args[1], args[2], args[3], args[4]); err != nil {
			handle(err)
		}
	case "chown":
		if len(args) != 3 {
			helpf("chown requires <user> and <id>")
		}
		if err := acl("chown", "", args[1], args[2]); err != nil {
			handle(err)
		}
	default:
		helpf("invalid command")
	}
}
