package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"sorcia/setting"
	"strings"

	"github.com/urfave/cli"
)

// SSHServe ...
var SSHServe = cli.Command{
	Name:        "sshserve",
	Usage:       "Start web server",
	Description: `This serves git via SSH`,
	Action:      runSSH,
}

func parseSSHCmd(cmd string) (string, string) {
	ss := strings.SplitN(cmd, " ", 2)
	if len(ss) != 2 {
		return "", ""
	}
	return ss[0], strings.Replace(ss[1], "'/", "'", 1)
}

func runSSH(c *cli.Context) error {

	// Get config values
	conf := setting.GetConf()

	sshCmd := os.Getenv("SSH_ORIGINAL_COMMAND")
	if len(sshCmd) == 0 {
		println("Hi there, You've successfully authenticated, but sorcia does not provide shell access.")
		println("If this is unexpected, please log in with password and setup sorcia under another user.")
		return nil
	}

	_, args := parseSSHCmd(sshCmd)
	repoFullName := strings.ToLower(strings.Trim(args, "'"))
	repoFields := strings.SplitN(repoFullName, "/", 2)
	if len(repoFields) != 2 {
		fmt.Printf("Invalid repository path", "Invalid repository path: %v", args)
	}
	ownerName := strings.ToLower(repoFields[0])
	repoName := strings.TrimSuffix(strings.ToLower(repoFields[1]), ".git")
	repoName = strings.TrimSuffix(repoName, ".wiki")

	fmt.Println(repoFullName)
	fmt.Println(ownerName)
	fmt.Println(repoName)

	cmdSSHServe := exec.Command("git-shell", "-c", sshCmd)
	cmdSSHServe.Dir = conf.Paths.DataPath // This should be repo root path
	cmdSSHServe.Stdout = os.Stdout
	cmdSSHServe.Stdin = os.Stdin
	cmdSSHServe.Stderr = os.Stderr
	if err := cmdSSHServe.Run(); err != nil {
		fmt.Printf("Fail to execute git command: %v", err)
	}

	return nil
}
