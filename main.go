package main

import "github.com/user/k8sdoc/cmd"

var (
	version = "0.1.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	cmd.Execute(version, commit, date)
}
