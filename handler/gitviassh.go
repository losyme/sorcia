package handler

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	errorhandler "sorcia/error"
	"sorcia/model"
	"sorcia/setting"
	"sorcia/util"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

var authorizedKey []byte
var gitRPC, gitRepo string
var userID string
var reponame string

// RunSSH ...
func RunSSH(conf *setting.BaseStruct, db *sql.DB) {
	ssh.Handle(func(s ssh.Session) {
		authorizedKey = gossh.MarshalAuthorizedKey(s.PublicKey())

		if len(s.Command()) == 2 {
			gitRPC = s.Command()[0]
			gitRepo = s.Command()[1]

			if strings.HasPrefix(gitRepo, "/") {
				gitRepo = strings.Split(gitRepo, "/")[1]
			}

			if !strings.HasSuffix(gitRepo, ".git") {
				log.Printf("ssh: invalid git repository name")
				return
			}

			reponame = strings.Split(gitRepo, ".git")[0]
			userIDInt, err := strconv.Atoi(userID)
			if err != nil {
				log.Printf("ssh: cannot convert userID to integer")
				return
			}
			repoID := model.GetRepoIDFromReponame(db, reponame)

			if isRepoPrivate := model.GetRepoType(db, reponame); isRepoPrivate && gitRPC == "upload-pack" {
				if !model.CheckRepoOwnerFromUserIDAndReponame(db, userIDInt, reponame) && !model.CheckRepoMemberExistFromUserIDAndRepoID(db, userIDInt, repoID) {
					log.Printf("ssh: no repo access")
					return
				} else if model.CheckRepoMemberExistFromUserIDAndRepoID(db, userIDInt, repoID) {
					permission := model.GetRepoMemberPermissionFromUserIDAndRepoID(db, userIDInt, repoID)
					if permission != "read" && permission != "read/write" {
						log.Printf("ssh: no repo access")
						return
					}
				}
			}

			if gitRPC == "git-receive-pack" {
				if !model.CheckRepoOwnerFromUserIDAndReponame(db, userIDInt, reponame) && !model.CheckRepoMemberExistFromUserIDAndRepoID(db, userIDInt, repoID) {
					log.Printf("ssh: no repo access")
					return
				} else if model.CheckRepoMemberExistFromUserIDAndRepoID(db, userIDInt, repoID) {
					permission := model.GetRepoMemberPermissionFromUserIDAndRepoID(db, userIDInt, repoID)
					if permission != "read/write" {
						log.Printf("ssh: no repo access")
						return
					}
				}
			}
		} else {
			log.Printf("ssh: no git command")
			return
		}

		cmd := exec.Command(gitRPC, gitRepo)
		cmd.Dir = conf.Paths.RepoPath

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("ssh: cant open stdout pipe: %v", err)
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Printf("ssh: cant open stderr pipe: %v", err)
			return
		}

		input, err := cmd.StdinPipe()
		if err != nil {
			log.Printf("ssh: cant open stdin pipe: %v", err)
			return
		}

		if err = cmd.Start(); err != nil {
			log.Printf("ssh: start error: %v", err)
			return
		}

		go io.Copy(input, s)
		io.Copy(s, stdout)
		io.Copy(s.Stderr(), stderr)

		if err = cmd.Wait(); err != nil {
			log.Printf("ssh: command failed: %v", err)
			return
		}

		s.SendRequest("exit-status", false, []byte{0, 0, 0, 0})

		if gitRPC == "git-receive-pack" {
			go util.GenerateRefs(conf.Paths.RefsPath, conf.Paths.RepoPath, gitRepo)
		}

		return
	})

	publicKeyOption := ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
		sshDetail := model.GetSSHAllAuthKeys(db)

		for i := 0; i < len(sshDetail.AuthKeys); i++ {
			authKeyByte := []byte(sshDetail.AuthKeys[i])
			allowed, _, _, _, err := gossh.ParseAuthorizedKey(authKeyByte)
			errorhandler.CheckError("Error on Parse authorized key", err)

			if ssh.KeysEqual(key, allowed) {
				userID = sshDetail.UserIDs[i]
				return true
			}
		}
		log.Printf("Failed to handshake")
		return false
	})

	log.Printf("Starting ssh server on port %s...", conf.Server.SSHPort)
	sshPort := fmt.Sprintf(":%s", conf.Server.SSHPort)
	log.Fatal(ssh.ListenAndServe(sshPort, nil, ssh.NoPty(), publicKeyOption, ssh.HostKeyFile(filepath.Join(conf.Paths.SSHPath, "id_rsa"))))
}
